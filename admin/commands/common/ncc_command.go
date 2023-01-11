package common

import (
	"context"
	"encoding/hex"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/singleton"
	"reflect"
	"strconv"
)

var _ commands.AdminCommand = (*NccCommand)(nil)

type NccCommand struct {
	configs     *updatable_configs.Manager
	network     network.Network
	pingService network.PingService
	libp2p      p2p.LibP2PNode
	me          module.Local
	idProvider  module.IdentityProvider
	translator  p2p.IDTranslator
	middleware  network.Middleware
}

func NewNccCommand(configs *updatable_configs.Manager, network network.Network,
	service network.PingService, libp2p p2p.LibP2PNode, me module.Local, provider module.IdentityProvider,
	translator p2p.IDTranslator, middleware network.Middleware) *NccCommand {
	return &NccCommand{
		configs:     configs,
		network:     network,
		pingService: service,
		libp2p:      libp2p,
		me:          me,
		idProvider:  provider,
		translator:  translator,
		middleware:  middleware,
	}
}

func (s *NccCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	single := singleton.GetSingle()
	input, ok := req.Data.(map[string]interface{})
	cmd, ok := input["cmd"].(string)
	if !ok {
		return "failed to find cmd", nil
	}

	switch cmd {

	case "whoami": // utilizes saved struct interfaces
		flowId := s.me.NodeID()                       // Get my Flow ID
		flowIdString := flowId.String()               // ...to string
		peerId, err := s.translator.GetPeerID(flowId) // Get my Libp2p ID
		if err != nil {
			return err.Error(), nil
		}
		peerIdString := peerId.String() // ...to string
		result := make(map[string]any, 2)
		result["flow_id"] = flowIdString
		result["peer_id"] = peerIdString
		return result, nil

	case "dump-dht": // utilizes singleton to get at DHT directly
		peers := single.Dht.RoutingTable().ListPeers()
		var result []string
		for _, peer := range peers {
			result = append(result, peer.String())
		}
		return commands.ConvertToInterfaceList(result)

	case "dht-forcerefresh": // for use with WireShark
		single.Dht.ForceRefresh()
		return "thanks friend", nil

	case "ping-peerid":
		nodeid, ok := input["peerid"]
		if !ok {
			return "failed to find peerid", nil
		}
		pingId, err := peer.Decode(nodeid.(string))
		if err != nil {
			return err.Error(), nil
		}
		msg, time, err := s.pingService.Ping(context.Background(), pingId)
		if err != nil {
			return err.Error(), nil
		}
		result := make(map[string]any, 2)
		result["result"] = msg.String()
		result["time"] = time.String()
		return result, nil

	case "publish-topic-data":
		topicStr, ok := input["topic"] // e.g. "request-receipts-by-block-id/bf2e32234232a9c563aa22f3c13b4b873d137c4f9c544f62895411cb52f7d472"
		if !ok {
			return "failed to find topic", nil
		}
		topic := channels.Topic(topicStr.(string))
		hexData, ok := input["data"]
		if !ok {
			return "failed to find data", nil
		}
		data, err := hex.DecodeString(hexData.(string))
		if err != nil {
			return "could not decode hex data", nil
		}
		err = s.libp2p.Publish(context.Background(), topic, data)
		if err != nil {
			return err.Error(), nil
		}
		return "thanks friend", nil

	case "publish-bytes":
		hexData, ok := input["data"]
		if !ok {
			return "failed to find data", nil
		}
		data, err := hex.DecodeString(hexData.(string))
		if err != nil {
			return "could not decode hex data", nil
		}
		err = single.Invoke_GossipSubTopic_PublishFunc(data)
		if err != nil {
			return err.Error(), nil
		}
		return "thanks friend", nil

	case "private-ping":
		peerStr, ok := input["peerid"]
		if !ok {
			return "failed to find peerid", nil
		}
		peerId, err := peer.Decode(peerStr.(string))
		if err != nil {
			return err.Error(), nil
		}
		msg, time, err := single.Invoke_PingService_pingFunc(peerId)
		if err != nil {
			return err.Error(), nil
		}
		result := make(map[string]any, 2)
		result["result"] = msg.String()
		result["time"] = time.String()
		return result, nil

	case "getAllTopics": // under development
		xx := single.SubscriptionProvider_getAllTopics()
		outMsg := ""
		for i, x := range xx {
			outMsg = outMsg + "  " + strconv.Itoa(i) + " " + x
		}
		return outMsg, nil

	case "libp2p-addPeer": // under development
		peerStr, ok := input["peerInfo"]
		if !ok {
			return "failed to find peerInfo", nil
		}
		peerInfo, err := peer.AddrInfoFromString(peerStr.(string))
		if err != nil {
			return err.Error(), nil
		}
		obj := single.LibP2PNode
		err = obj.AddPeer(context.Background(), *peerInfo)
		if err != nil {
			return err.Error(), err
		}
		outMsg := "  " + reflect.ValueOf(obj).FieldByName("addr").String()
		return outMsg, nil

	case "libp2p-createStream":
		peerStr, ok := input["peerid"]
		if !ok {
			return "failed to find peerid", nil
		}
		peerId, err := peer.Decode(peerStr.(string))
		if err != nil {
			return err.Error(), nil
		}
		obj := single.LibP2PNode
		stream, err := obj.CreateStream(context.Background(), peerId)
		if err != nil {
			return err.Error(), err
		}
		result := make(map[string]any, 1)
		result["stream"] = stream.ID()
		return result, nil

	case "peer-routing":
		peerStr, ok := input["peerid"]
		if !ok {
			return "failed to find peerid", nil
		}
		peerId, err := peer.Decode(peerStr.(string))
		if err != nil {
			return err.Error(), nil
		}
		outMsg := single.Invoke_PeerRouting(peerId)
		return outMsg, nil

	default:
		return "unrecognized command", nil
	}
}

// Validator ... kind of a placeholder
func (s *NccCommand) Validator(req *admin.CommandRequest) error {
	_, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("malformed input")
	}
	return nil
}
