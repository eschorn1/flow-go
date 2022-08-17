package orchestrator

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/logging"
)

// OrchestratorNetwork implements a middleware for mounting an attack orchestrator and empowering it to communicate with the corrupt nodes.
type OrchestratorNetwork struct {
	component.Component
	cm                 *component.ComponentManager
	orchestratorMutex  sync.Mutex // to ensure thread-safe calls into orchestrator.
	logger             zerolog.Logger
	orchestrator       insecure.AttackOrchestrator // the mounted orchestrator that implements certain attack logic.
	codec              network.Codec
	corruptNodeIds     flow.IdentityList                                    // identity of the corrupt nodes
	corruptConnections map[flow.Identifier]insecure.CorruptedNodeConnection // existing connections to the corrupt nodes.
	corruptConnector   insecure.CorruptedNodeConnector                      // connection generator to corrupt nodes.
}

var _ insecure.OrchestratorNetwork = &OrchestratorNetwork{}

func NewOrchestratorNetwork(
	logger zerolog.Logger,
	codec network.Codec,
	orchestrator insecure.AttackOrchestrator,
	connector insecure.CorruptedNodeConnector,
	corruptNodeIds flow.IdentityList) (*OrchestratorNetwork, error) {

	orchestratorNetwork := &OrchestratorNetwork{
		orchestrator:       orchestrator,
		logger:             logger,
		codec:              codec,
		corruptConnector:   connector,
		corruptNodeIds:     corruptNodeIds,
		corruptConnections: make(map[flow.Identifier]insecure.CorruptedNodeConnection),
	}

	connector.WithIncomingMessageHandler(orchestratorNetwork.Observe)

	// setting lifecycle management module.
	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			err := orchestratorNetwork.start(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not start orchestratorNetwork: %w", err))
			}

			ready()

			<-ctx.Done()

			err = orchestratorNetwork.stop()
			if err != nil {
				ctx.Throw(fmt.Errorf("could not stop orchestratorNetwork: %w", err))
			}
		}).Build()

	orchestratorNetwork.Component = cm
	orchestratorNetwork.cm = cm

	return orchestratorNetwork, nil
}

// start triggers the sub-modules of orchestrator network.
func (on *OrchestratorNetwork) start(ctx irrecoverable.SignalerContext) error {
	// creates a connection to all corrupt nodes in the orchestrator network.
	for _, corruptNodeId := range on.corruptNodeIds {
		connection, err := on.corruptConnector.Connect(ctx, corruptNodeId.NodeID)
		if err != nil {
			return fmt.Errorf("could not establish corrupt connection to node %x: %w", corruptNodeId.NodeID, err)
		}
		on.corruptConnections[corruptNodeId.NodeID] = connection
		on.logger.Info().Hex("node_id", logging.ID(corruptNodeId.NodeID)).Msg("attack orchestrator successfully registered on corrupt node")
	}

	// registers orchestrator network for orchestrator.
	on.orchestrator.Register(on)

	return nil
}

// stop conducts the termination logic of the sub-modules of orchestrator network.
func (on *OrchestratorNetwork) stop() error {
	// tears down connections to corrupt nodes.
	var errors *multierror.Error
	for _, connection := range on.corruptConnections {
		err := connection.CloseConnection()

		if err != nil {
			errors = multierror.Append(errors, err)
		}
	}

	return errors.ErrorOrNil()
}

// Observe is the inbound message handler of the orchestrator network.
// Instead of dispatching their messages to the networking layer of Flow, the conduits of corrupt nodes
// dispatch the outgoing messages to the orchestrator network by calling the InboundHandler method of it remotely.
func (on *OrchestratorNetwork) Observe(message *insecure.Message) {
	if err := on.processEgressMessage(message); err != nil {
		on.logger.Fatal().Err(err).Msg("could not process message of corrupt node")
	}
}

// processEgressMessage processes incoming messages arrived from corrupt conduits by passing them
// to the orchestrator.
func (on *OrchestratorNetwork) processEgressMessage(message *insecure.Message) error {
	event, err := on.codec.Decode(message.Egress.Payload)
	if err != nil {
		return fmt.Errorf("could not decode observed payload: %w", err)
	}

	sender, err := flow.ByteSliceToId(message.Egress.CorruptOriginID)
	if err != nil {
		return fmt.Errorf("could not convert origin id to flow identifier: %w", err)
	}

	targetIds, err := flow.ByteSlicesToIds(message.Egress.TargetIDs)
	if err != nil {
		return fmt.Errorf("could not convert target ids to flow identifiers: %w", err)
	}

	// making sure events are sent sequentially to orchestrator.
	on.orchestratorMutex.Lock()
	defer on.orchestratorMutex.Unlock()

	err = on.orchestrator.HandleEventFromCorruptedNode(&insecure.EgressEvent{
		CorruptOriginId:   sender,
		Channel:           channels.Channel(message.Egress.ChannelID),
		FlowProtocolEvent: event,
		Protocol:          message.Egress.Protocol,
		TargetNum:         message.Egress.TargetNum,
		TargetIds:         targetIds,
	})
	if err != nil {
		return fmt.Errorf("could not handle event by orchestrator: %w", err)
	}

	return nil
}

// SendEgress enforces dissemination of given event via its encapsulated corrupt node networking layer through the Flow network.
// An orchestrator decides when to send an egress message on behalf of a corrupt node.
func (on *OrchestratorNetwork) SendEgress(event *insecure.EgressEvent) error {
	msg, err := on.eventToEgressMessage(event.CorruptOriginId, event.FlowProtocolEvent, event.Channel, event.Protocol, event.TargetNum, event.TargetIds...)
	if err != nil {
		return fmt.Errorf("could not convert egress event to egress message: %w", err)
	}

	err = on.sendMessage(msg, event.CorruptOriginId)
	if err != nil {
		return fmt.Errorf("could not send egress event from corrupt node: %w", err)
	}

	return nil
}

// SendIngress sends an incoming message from the flow network (from another node that could be or honest or corrupt)
// to the corrupt node. This message was intercepted by the orchestrator network and relayed to the orchestrator before being sent
// to the corrupt node.
func (on *OrchestratorNetwork) SendIngress(event *insecure.IngressEvent) error {
	msg, err := on.eventToIngressMessage(event.OriginID, event.FlowProtocolEvent, event.Channel, event.CorruptTargetID)
	if err != nil {
		return fmt.Errorf("could not convert ingress event to ingress message: %w", err)
	}

	err = on.sendMessage(msg, event.CorruptTargetID)
	if err != nil {
		return fmt.Errorf("could not send ingress event to corrupt node: %w", err)
	}
	return nil
}

// sendMessage is a helper function for sending both ingress and egress messages.
func (on *OrchestratorNetwork) sendMessage(msg *insecure.Message, corruptNodeId flow.Identifier) error {
	connection, ok := on.corruptConnections[corruptNodeId]
	if !ok {
		return fmt.Errorf("no connection available for corrupt conduit factory to node %x: ", corruptNodeId)
	}

	err := connection.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("could not send event to corrupt node: %w", err)
	}

	return nil
}

// eventToEgressMessage converts the given application layer event to a protobuf message that is meant to be sent FROM the corrupt node.
func (on *OrchestratorNetwork) eventToEgressMessage(corruptId flow.Identifier,
	event interface{},
	channel channels.Channel,
	protocol insecure.Protocol,
	num uint32,
	targetIds ...flow.Identifier) (*insecure.Message, error) {

	payload, err := on.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	egressMsg := &insecure.EgressMessage{
		ChannelID:       channel.String(),
		CorruptOriginID: corruptId[:],
		TargetNum:       num,
		TargetIDs:       flow.IdsToBytes(targetIds),
		Payload:         payload,
		Protocol:        protocol,
	}

	return &insecure.Message{
		Egress: egressMsg,
	}, nil
}

// eventToIngressMessage converts the given application layer event to a protobuf message that is meant to be sent TO the corrupt node.
func (on *OrchestratorNetwork) eventToIngressMessage(originId flow.Identifier,
	event interface{},
	channel channels.Channel,
	targetId flow.Identifier) (*insecure.Message, error) {

	payload, err := on.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	ingressMsg := &insecure.IngressMessage{
		ChannelID:       channel.String(),
		OriginID:        originId[:], // origin node ID this message was sent from
		CorruptTargetID: targetId[:], // corrupt node ID this message is intended for
		Payload:         payload,
	}

	return &insecure.Message{
		Ingress: ingressMsg,
	}, nil
}
