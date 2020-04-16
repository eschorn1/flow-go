package integration

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/blockproducer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/eventhandler"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/forkchoice"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voter"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	module "github.com/dapperlabs/flow-go/module/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Instance struct {

	// instance parameters
	participants flow.IdentityList
	localID      flow.Identifier
	blockVoteIn  VoteFilter
	blockVoteOut VoteFilter
	blockPropIn  ProposalFilter
	blockPropOut ProposalFilter
	stop         Condition

	// instance data
	queue   chan interface{}
	headers map[flow.Identifier]*flow.Header

	// mocked dependencies
	snapshot     *protocol.Snapshot
	proto        *protocol.State
	builder      *module.Builder
	finalizer    *module.Finalizer
	signer       *mocks.Signer
	verifier     *mocks.Verifier
	communicator *mocks.Communicator

	// real dependencies
	viewstate  *viewstate.ViewState
	pacemaker  *pacemaker.FlowPaceMaker
	producer   *blockproducer.BlockProducer
	forks      *forks.Forks
	aggregator *voteaggregator.VoteAggregator
	voter      *voter.Voter
	validator  *validator.Validator

	// main logic
	handler *eventhandler.EventHandler
	loop    *hotstuff.EventLoop
}

func NewInstance(t require.TestingT, options ...Option) *Instance {

	// generate random default identity
	identity := unittest.IdentityFixture()

	// initialize the default configuration
	cfg := Config{
		Root:              DefaultRoot(),
		Participants:      flow.IdentityList{identity},
		LocalID:           identity.NodeID,
		Timeouts:          timeout.DefaultConfig,
		IncomingVotes:     BlockNoVotes,
		OutgoingVotes:     BlockNoVotes,
		IncomingProposals: BlockNoProposals,
		OutgoingProposals: BlockNoProposals,
		StopCondition:     RightAway,
	}

	// apply the custom options
	for _, option := range options {
		option(&cfg)
	}

	// check the local ID is a participant
	var index uint
	takesPart := false
	for i, participant := range cfg.Participants {
		if participant.NodeID == cfg.LocalID {
			index = uint(i)
			takesPart = true
			break
		}
	}
	require.True(t, takesPart)

	// initialize the instance
	in := Instance{

		// instance parameters
		participants: cfg.Participants,
		localID:      cfg.LocalID,
		blockVoteIn:  cfg.IncomingVotes,
		blockVoteOut: cfg.OutgoingVotes,
		blockPropIn:  cfg.IncomingProposals,
		blockPropOut: cfg.OutgoingProposals,
		stop:         cfg.StopCondition,

		// instance data
		queue:   make(chan interface{}, 1024),
		headers: make(map[flow.Identifier]*flow.Header),

		// instance mocks
		snapshot:     &protocol.Snapshot{},
		proto:        &protocol.State{},
		builder:      &module.Builder{},
		signer:       &mocks.Signer{},
		verifier:     &mocks.Verifier{},
		communicator: &mocks.Communicator{},
		finalizer:    &module.Finalizer{},
	}

	// insert root block into headers register
	in.headers[cfg.Root.ID()] = cfg.Root

	// program the protocol snapshot behaviour
	in.snapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return in.participants.Filter(selector)
		},
		nil,
	)
	for _, participant := range in.participants {
		in.snapshot.On("Identity", participant.NodeID).Return(participant, nil)
	}

	// program the protocol state behaviour
	in.proto.On("Final").Return(in.snapshot)
	in.proto.On("AtNumber", mock.Anything).Return(in.snapshot)
	in.proto.On("AtBlockID", mock.Anything).Return(in.snapshot)

	// program the builder module behaviour
	in.builder.On("BuildOn", mock.Anything, mock.Anything).Return(
		func(parentID flow.Identifier, setter func(*flow.Header)) *flow.Header {
			parent, ok := in.headers[parentID]
			if !ok {
				return nil
			}
			header := &flow.Header{
				ChainID:     "chain",
				ParentID:    parentID,
				Height:      parent.Height + 1,
				PayloadHash: unittest.IdentifierFixture(),
				Timestamp:   time.Now().UTC(),
			}
			setter(header)
			in.headers[header.ID()] = header
			return header
		},
		func(parentID flow.Identifier, setter func(*flow.Header)) error {
			_, ok := in.headers[parentID]
			if !ok {
				return fmt.Errorf("parent block not found (parent: %x)", parentID)
			}
			return nil
		},
	)

	// program the hotstuff signer behaviour
	in.signer.On("CreateProposal", mock.Anything).Return(
		func(block *model.Block) *model.Proposal {
			proposal := &model.Proposal{
				Block:   block,
				SigData: nil,
			}
			return proposal
		},
		nil,
	)
	in.signer.On("CreateVote", mock.Anything).Return(
		func(block *model.Block) *model.Vote {
			vote := &model.Vote{
				View:     block.View,
				BlockID:  block.BlockID,
				SignerID: in.localID,
				SigData:  nil,
			}
			return vote
		},
		nil,
	)
	in.signer.On("CreateQC", mock.Anything).Return(
		func(votes []*model.Vote) *model.QuorumCertificate {
			voterIDs := make([]flow.Identifier, 0, len(votes))
			for _, vote := range votes {
				voterIDs = append(voterIDs, vote.SignerID)
			}
			qc := &model.QuorumCertificate{
				View:      votes[0].View,
				BlockID:   votes[0].BlockID,
				SignerIDs: voterIDs,
				SigData:   nil,
			}
			return qc
		},
		nil,
	)

	// program the hotstuff verifier behaviour
	in.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	in.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// program the hotstuff communicator behaviour
	in.communicator.On("BroadcastProposal", mock.Anything).Return(
		func(header *flow.Header) error {

			// check that we have the parent
			parent, found := in.headers[header.ParentID]
			if !found {
				return fmt.Errorf("can't broadcast with unknown parent")
			}

			// set the height and chain ID
			header.ChainID = parent.ChainID
			header.Height = parent.Height + 1
			return nil
		},
	)
	in.communicator.On("SendVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// program the finalizer module behaviour
	in.finalizer.On("MakeFinal", mock.Anything).Return(
		func(blockID flow.Identifier) error {

			// as we don't use mocks to assert expectations, but only to
			// simulate behaviour, we should drop the call data regularly
			if len(in.headers)%100 == 0 {
				in.snapshot.Calls = nil
				in.proto.Calls = nil
				in.builder.Calls = nil
				in.signer.Calls = nil
				in.verifier.Calls = nil
				in.communicator.Calls = nil
				in.finalizer.Calls = nil
			}

			// check on stop condition
			// TODO: hook into notifier & stop manually, so it works even when
			// no blocks are finalized
			if in.stop(&in) {
				return errStopCondition
			}

			return nil
		},
	)

	// initialize error handling and logging
	var err error
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel).With().Timestamp().Uint("index", index).Hex("local_id", in.localID[:]).Logger()
	notifier := notifications.NewLogConsumer(log)

	// initialize no-op metrics mock
	metrics := &module.Metrics{}
	metrics.On("HotStuffBusyDuration", mock.Anything)
	metrics.On("HotStuffIdleDuration", mock.Anything)

	// initialize the viewstate
	in.viewstate, err = viewstate.New(in.proto, in.localID, filter.Any)
	require.NoError(t, err)

	// initialize the pacemaker
	controller := timeout.NewController(cfg.Timeouts)
	in.pacemaker, err = pacemaker.New(DefaultStart(), controller, notifier)
	require.NoError(t, err)

	// initialize the block producer
	in.producer, err = blockproducer.New(in.signer, in.viewstate, in.builder)
	require.NoError(t, err)

	// initialize the finalizer
	rootBlock := model.BlockFromFlow(cfg.Root, 0)
	rootQC := &model.QuorumCertificate{
		View:    rootBlock.View,
		BlockID: rootBlock.BlockID,
	}
	rootBlockQC := &forks.BlockQC{Block: rootBlock, QC: rootQC}
	forkalizer, err := finalizer.New(rootBlockQC, in.finalizer, notifier)
	require.NoError(t, err)

	// initialize the forks choice
	choice, err := forkchoice.NewNewestForkChoice(forkalizer, notifier)
	require.NoError(t, err)

	// initialize the forks handler
	in.forks = forks.New(forkalizer, choice)

	// initialize the validator
	in.validator = validator.New(in.viewstate, in.forks, in.verifier)

	// initialize the vote aggregator
	in.aggregator = voteaggregator.New(notifier, DefaultPruned(), in.viewstate, in.validator, in.signer)

	// initialize the voter
	in.voter = voter.New(in.signer, in.forks, DefaultVoted())

	// initialize the event handler
	in.handler, err = eventhandler.New(log, in.pacemaker, in.producer, in.forks, in.communicator, in.viewstate, in.aggregator, in.voter, in.validator, notifier)
	require.NoError(t, err)

	// initialize and return the event loop
	in.loop, err = hotstuff.NewEventLoop(log, metrics, in.handler)
	require.NoError(t, err)

	// launch a goroutine to read the queue and submit messages
	go func() {
		for message := range in.queue {
			switch msg := message.(type) {
			case *model.Proposal:
				in.loop.OnReceiveProposal(msg)
			case *model.Vote:
				in.loop.OnReceiveVote(msg)
			}
		}
	}()

	return &in
}