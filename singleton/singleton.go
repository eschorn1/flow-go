package singleton

// This package/file exists to allow access to internal (some cases private) data and functions
// Note: the "_x" vs "_X" indicates private/public sources

import (
	"context"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/onflow/flow-go/network/message"
	"time"
)

// GossipSubTopic_PublishFunc -- A public `Publish` function
type GossipSubTopic_PublishFunc func(ctx context.Context, bytes []byte) error

// PingService_pingFunc -- A private `ping` function
type PingService_pingFunc func(ctx context.Context, p peer.ID) (message.PingResponse, time.Duration, error)

// GetAllTopics -- Under dev
type GetAllTopics func() []string

// Single -- Items we want to attach, then access and maybe invoke
type Single struct {
	SubscriptionProvider_getAllTopics GetAllTopics
	LibP2PNodeBuilder                 interface{} // experimenting with abstract/specific types
	O                                 interface{}
	PeerRouting                       routing.PeerRouting
	GossipSubTopic_PublishFunc        GossipSubTopic_PublishFunc
	PingService_pingFunc              PingService_pingFunc // a private function!!
	Dht                               *dht.IpfsDHT
}

var instantiated *Single = nil

// GetSingle (potentially) instantiates single instance
func GetSingle() *Single {
	if instantiated == nil {
		instantiated = new(Single)
	}
	return instantiated
}

/////////
// Stash   - connects an object or interface
// Attach  - attaches a function
// Invoke  - invokes attached function
/////////

func (Single *Single) Stash_SubscriptionProvider_getAllTopics(getAllTopics GetAllTopics) {
	if Single.SubscriptionProvider_getAllTopics == nil { // "re-stashing" really isn't a problem...should remove
		Single.SubscriptionProvider_getAllTopics = getAllTopics
	}
}

func (Single *Single) Stash_LibP2PNodeBuilder(libP2PNodeBuilder interface{}) {
	if Single.LibP2PNodeBuilder == nil {
		Single.LibP2PNodeBuilder = libP2PNodeBuilder
	}
}

func (Single *Single) Stash_Dht(dht *dht.IpfsDHT) {
	if Single.Dht == nil {
		Single.Dht = dht
	}
}

func (Single *Single) Stash_PeerRouting(item routing.PeerRouting) {
	if Single.PeerRouting == nil {
		Single.PeerRouting = item
	}
}

func (Single *Single) Attach_GossipSubTopic_PublishFunc(gossipSubTopic_PublishFunc GossipSubTopic_PublishFunc) {
	if Single.GossipSubTopic_PublishFunc == nil {
		Single.GossipSubTopic_PublishFunc = gossipSubTopic_PublishFunc
	}
}

// Attach_PingService_pingFunc a **private** func -- demo purposes so far
func (Single *Single) Attach_PingService_pingFunc(pingService_pingFunc PingService_pingFunc) {
	if Single.PingService_pingFunc == nil {
		Single.PingService_pingFunc = pingService_pingFunc
	}
}

func (Single *Single) Invoke_PeerRouting(p peer.ID) string {
	if Single.PeerRouting == nil {
		return "routing.PeerRouting has not been attached yet"
	}
	obj := Single.PeerRouting.(routing.PeerRouting)
	ctx := context.Background()
	ret, _ := obj.FindPeer(ctx, p)
	return ret.String()
}

// Invoke_GossipSubTopic_PublishFunc Public publish func that bypasses validation etc...
func (Single *Single) Invoke_GossipSubTopic_PublishFunc(bytes []byte) error {
	if Single.GossipSubTopic_PublishFunc == nil {
		return nil
	}
	ctx2 := context.Background()
	return Single.GossipSubTopic_PublishFunc(ctx2, bytes)
}

// Invoke_PingService_pingFunc The private ping function
func (Single *Single) Invoke_PingService_pingFunc(p peer.ID) (message.PingResponse, time.Duration, error) {
	if Single.PingService_pingFunc == nil {
		return message.PingResponse{}, time.Second, nil
	}
	ctx2 := context.Background()
	return Single.PingService_pingFunc(ctx2, p)
}
