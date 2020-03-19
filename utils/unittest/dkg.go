// +build relic

package unittest

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/crypto"
)

const (
	dkgType int = iota
)

type message struct {
	orig    int
	msgType int
	data    []byte
}

// implements DKGProcessor interface
type TestDKGProcessor struct {
	t       *testing.T
	current int
	dkg     crypto.DKGstate
	chans   []chan *message
	msgType int
	pkBytes []byte
	// the current node private key (a DKG output)
	currentPrivateKey crypto.PrivateKey
	// the group public key (a DKG output)
	groupPublicKey crypto.PublicKey
	// the group public key shares (a DKG output)
	publicKeyShares []crypto.PublicKey
}

// This is a testing function
// it simulates sending a honest message from one node to another
func (proc *TestDKGProcessor) honestSend(dest int, data []byte) {
	proc.t.Logf("%d Sending to %d:\n", proc.current, dest)
	proc.t.Log(data)
	newMsg := &message{proc.current, proc.msgType, data}
	proc.chans[dest] <- newMsg
}

// This is a testing function
// it simulates sending a message from one node to another
func (proc *TestDKGProcessor) Send(dest int, data []byte) {
	proc.honestSend(dest, data)
}

// This is a testing function
// it simulates broadcasting a message from one node to all nodes
func (proc *TestDKGProcessor) Broadcast(data []byte) {
	proc.t.Logf("%d Broadcasting:", proc.current)
	proc.t.Log(data)
	newMsg := &message{proc.current, proc.msgType, data}
	for i := 0; i < len(proc.chans); i++ {
		if i != proc.current {
			proc.chans[i] <- newMsg
		}
	}
}

func (proc *TestDKGProcessor) Blacklist(node int) {
	proc.t.Logf("%d wants to blacklist %d", proc.current, node)
}
func (proc *TestDKGProcessor) FlagMisbehavior(node int, logData string) {
	proc.t.Logf("%d flags a misbehavior from %d: %s", proc.current, node, logData)
}

func RunDKGKeys(t *testing.T, n int) ([]crypto.PrivateKey, crypto.PublicKey, []crypto.PublicKey) {
	lead := 0
	var wg sync.WaitGroup
	chans := make([]chan *message, n)
	processors := make([]TestDKGProcessor, 0, n)

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, TestDKGProcessor{
			t:       t,
			current: current,
			chans:   chans,
			msgType: dkgType,
		})
		// create DKG in all nodes
		var err error
		processors[current].dkg, err = crypto.NewDKG(crypto.FeldmanVSS, n, current,
			&processors[current], lead)
		assert.Nil(t, err)
	}

	// create the node (buffered) communication channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 2*n)
	}
	// start DKG in all nodes but the leader
	seed := []byte{1, 2, 3, byte(n)}
	wg.Add(n)
	for current := 0; current < n; current++ {
		err := processors[current].dkg.StartDKG(seed)
		assert.Nil(t, err)
		go tsDkgRunChan(&processors[current], &wg, t, 2)
	}

	// synchronize the main thread to end DKG
	wg.Wait()
	for i := 1; i < n; i++ {
		assert.Equal(t, processors[i].pkBytes, processors[0].pkBytes,
			"2 group public keys are mismatching")
	}

	privateKeys := make([]crypto.PrivateKey, n)
	for i := 0; i < n; i++ {
		privateKeys[i] = processors[i].currentPrivateKey
	}
	groupKey := processors[0].groupPublicKey
	publicKeyShares := processors[0].publicKeyShares
	return privateKeys, groupKey, publicKeyShares
}

// This is a testing function
// It simulates processing incoming messages by a node during DKG
// It assumes proc.dkg is already running
func tsDkgRunChan(proc *TestDKGProcessor,
	sync *sync.WaitGroup, t *testing.T, phase int) {
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			err := proc.dkg.ReceiveDKGMsg(newMsg.orig, newMsg.data)
			assert.Nil(t, err)

		// if timeout, finalize DKG and sign the share
		case <-time.After(200 * time.Millisecond):
			switch phase {
			case 0:
				proc.t.Logf("%d shares phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				assert.Nil(t, err)
			case 1:
				proc.t.Logf("%d complaints phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				assert.Nil(t, err)
			case 2:
				proc.t.Logf("%d dkg ended \n", proc.current)
				sk, groupPK, nodesPK, err := proc.dkg.EndDKG()
				assert.NotNil(t, sk)
				assert.NotNil(t, groupPK)
				assert.NotNil(t, nodesPK)
				assert.Nil(t, err, "End dkg failed: %v\n", err)
				if groupPK == nil {
					proc.pkBytes = []byte{}
				} else {
					proc.pkBytes, _ = groupPK.Encode()
				}
				assert.Nil(t, err)
				proc.currentPrivateKey = sk
				proc.groupPublicKey = groupPK
				proc.publicKeyShares = nodesPK
			}
			sync.Done()
			return
		}
	}
}