// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	synccore "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Engine is the synchronization engine, responsible for synchronizing chain state.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	metrics module.EngineMetrics
	me      module.Local
	state   protocol.State
	con     network.Conduit
	blocks  storage.Blocks
	comp    network.Engine // compliance layer engine

	pollInterval              time.Duration
	scanInterval              time.Duration
	core                      module.SyncCore
	requestsNotifier          engine.Notifier
	pendingSyncRequestsQueue  *RequestQueue
	pendingBatchRequestsQueue *RequestQueue
	pendingRangeRequestsQueue *RequestQueue
	pendingSyncResponses      engine.MessageStore    // message store for *message.SyncResponse
	pendingBlockResponses     engine.MessageStore    // message store for *message.BlockResponse
	messageHandler            *engine.MessageHandler // message handler that takes care of responses processing
}

// New creates a new main chain synchronization engine.
func New(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net module.Network,
	me module.Local,
	state protocol.State,
	blocks storage.Blocks,
	comp network.Engine,
	core module.SyncCore,
	opts ...OptionFunc,
) (*Engine, error) {

	opt := DefaultConfig()
	for _, f := range opts {
		f(opt)
	}

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:         engine.NewUnit(),
		log:          log.With().Str("engine", "synchronization").Logger(),
		metrics:      metrics,
		me:           me,
		state:        state,
		blocks:       blocks,
		comp:         comp,
		core:         core,
		pollInterval: opt.pollInterval,
		scanInterval: opt.scanInterval,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.SyncCommittee, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.comp == nil {
		panic("must initialize synchronization engine with comp engine")
	}
	e.unit.Launch(e.checkLoop)
	e.unit.Launch(e.requestProcessingLoop)
	e.unit.Launch(e.responseProcessingLoop)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	err := e.Process(originID, event)
	if err != nil {
		engine.LogError(e.log, err)
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	switch _ := event.(type) {
	case *messages.SyncRequest:
		e.pendingSyncRequestsQueue.Push(originID, event)
	case *messages.RangeRequest:
		e.pendingRangeRequestsQueue.Push(originID, event)
	case *messages.BatchRequest:
		e.pendingBatchRequestsQueue.Push(originID, event)
	case *messages.SyncResponse, *messages.BlockResponse:
		return e.messageHandler.Process(originID, event)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
	return nil
}

func (e *Engine) requestProcessingLoop() {
	notifier := e.requestsNotifier.Channel()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-notifier:
			err := e.processAvailableRequests()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing queued message")
			}
		}
	}
}

func (e *Engine) responseProcessingLoop() {
	notifier := e.messageHandler.GetNotifier()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-notifier:
			err := e.processAvailableResponses()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing queued message")
			}
		}
	}
}

func (e *Engine) processSyncRequest(originID flow.Identifier, request *messages.SyncRequest) error {
	e.before(metrics.MessageSyncRequest)
	defer e.after(metrics.MessageSyncRequest)
	return e.onSyncRequest(originID, request)
}

func (e *Engine) processBatchRequest(originID flow.Identifier, request *messages.BatchRequest) error {
	e.before(metrics.MessageBatchRequest)
	defer e.after(metrics.MessageBatchRequest)
	return e.onBatchRequest(originID, request)
}

func (e *Engine) processRangeRequest(originID flow.Identifier, request *messages.RangeRequest) error {
	e.before(metrics.MessageRangeRequest)
	defer e.after(metrics.MessageRangeRequest)
	return e.onRangeRequest(originID, request)
}

func (e *Engine) processSyncResponse(originID flow.Identifier, response *messages.SyncResponse) error {
	e.before(metrics.MessageSyncResponse)
	defer e.after(metrics.MessageSyncResponse)
	return e.onSyncResponse(originID, response)
}

func (e *Engine) processBlockResponse(originID flow.Identifier, response *messages.BlockResponse) error {
	e.before(metrics.MessageBlockResponse)
	defer e.after(metrics.MessageBlockResponse)
	return e.onBlockResponse(originID, response)
}

func (e *Engine) processAvailableResponses() error {
	for {
		select {
		case <-e.unit.Quit():
			return nil
		default:
		}

		msg, ok := e.pendingSyncResponses.Get()
		if ok {
			err := e.processSyncResponse(msg.OriginID, msg.Payload.(*messages.SyncResponse))
			if err != nil {
				return fmt.Errorf("could not process sync response")
			}
			continue
		}

		msg, ok = e.pendingBlockResponses.Get()
		if ok {
			err := e.processBlockResponse(msg.OriginID, msg.Payload.(*messages.BlockResponse))
			if err != nil {
				return fmt.Errorf("could not process block response")
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// processAvailableRequests is processor of pending events which drives events from networking layer to business logic in `Core`.
// Effectively consumes messages from networking layer and dispatches them into corresponding sinks which are connected with `Core`.
func (e *Engine) processAvailableRequests() error {

	for {
		select {
		case <-e.unit.Quit():
			return nil
		default:
		}

		originID, event, ok := e.pendingSyncRequestsQueue.Pop()
		if ok {
			err := e.processSyncRequest(originID, event.(*messages.SyncRequest))
			if err != nil {
				return fmt.Errorf("could not process sync request")
			}
			continue
		}

		originID, event, ok = e.pendingRangeRequestsQueue.Pop()
		if ok {
			err := e.processRangeRequest(originID, event.(*messages.RangeRequest))
			if err != nil {
				return fmt.Errorf("could not process range request")
			}
			continue
		}

		originID, event, ok = e.pendingBatchRequestsQueue.Pop()
		if ok {
			err := e.processBatchRequest(originID, event.(*messages.BatchRequest))
			if err != nil {
				return fmt.Errorf("could not process batch request")
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

func (e *Engine) before(msg string) {
	e.metrics.MessageReceived(metrics.EngineSynchronization, msg)
	e.unit.Lock()
}

func (e *Engine) after(msg string) {
	e.unit.Unlock()
	e.metrics.MessageHandled(metrics.EngineSynchronization, msg)
}

// onSyncRequest processes an outgoing handshake; if we have a higher height, we
// inform the other node of it, so they can organize their block downloads. If
// we have a lower height, we add the difference to our own download queue.
func (e *Engine) onSyncRequest(originID flow.Identifier, req *messages.SyncRequest) error {

	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized height: %w", err)
	}

	// queue any missing heights as needed
	e.core.HandleHeight(final, req.Height)

	// don't bother sending a response if we're within tolerance or if we're
	// behind the requester
	if e.core.WithinTolerance(final, req.Height) || req.Height > final.Height {
		return nil
	}

	// if we're sufficiently ahead of the requester, send a response
	res := &messages.SyncResponse{
		Height: final.Height,
		Nonce:  req.Nonce,
	}
	err = e.con.Unicast(res, originID)
	if err != nil {
		return fmt.Errorf("could not send sync response: %w", err)
	}

	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageSyncResponse)

	return nil
}

// onSyncResponse processes a synchronization response.
func (e *Engine) onSyncResponse(originID flow.Identifier, res *messages.SyncResponse) error {

	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized height: %w", err)
	}

	e.core.HandleHeight(final, res.Height)
	return nil
}

// onRangeRequest processes a request for a range of blocks by height.
func (e *Engine) onRangeRequest(originID flow.Identifier, req *messages.RangeRequest) error {

	// get the latest final state to know if we can fulfill the request
	head, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized head: %w", err)
	}

	// if we don't have anything to send, we can bail right away
	if head.Height < req.FromHeight || req.FromHeight > req.ToHeight {
		return nil
	}

	// get all of the blocks, one by one
	blocks := make([]*flow.Block, 0, req.ToHeight-req.FromHeight+1)
	for height := req.FromHeight; height <= req.ToHeight; height++ {
		block, err := e.blocks.ByHeight(height)
		if errors.Is(err, storage.ErrNotFound) {
			e.log.Error().Uint64("height", height).Msg("skipping unknown heights")
			break
		}
		if err != nil {
			return fmt.Errorf("could not get block for height (%d): %w", height, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		e.log.Debug().Msg("skipping empty range response")
		return nil
	}

	// send the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err = e.con.Unicast(res, originID)
	if err != nil {
		return fmt.Errorf("could not send range response: %w", err)
	}

	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBlockResponse)

	return nil
}

// onBatchRequest processes a request for a specific block by block ID.
func (e *Engine) onBatchRequest(originID flow.Identifier, req *messages.BatchRequest) error {

	// we should bail and send nothing on empty request
	if len(req.BlockIDs) == 0 {
		return nil
	}

	// deduplicate the block IDs in the batch request
	blockIDs := make(map[flow.Identifier]struct{})
	for _, blockID := range req.BlockIDs {
		blockIDs[blockID] = struct{}{}
	}

	// try to get all the blocks by ID
	blocks := make([]*flow.Block, 0, len(blockIDs))
	for blockID := range blockIDs {
		block, err := e.blocks.ByID(blockID)
		if errors.Is(err, storage.ErrNotFound) {
			e.log.Debug().Hex("block_id", blockID[:]).Msg("skipping unknown block")
			continue
		}
		if err != nil {
			return fmt.Errorf("could not get block by ID (%s): %w", blockID, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		e.log.Debug().Msg("skipping empty batch response")
		return nil
	}

	// send the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err := e.con.Unicast(res, originID)
	if err != nil {
		return fmt.Errorf("could not send batch response: %w", err)
	}

	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBlockResponse)

	return nil
}

// onBlockResponse processes a response containing a specifically requested block.
func (e *Engine) onBlockResponse(originID flow.Identifier, res *messages.BlockResponse) error {

	// process the blocks one by one
	for _, block := range res.Blocks {
		e.processIncomingBlock(originID, block)
	}

	return nil
}

// processIncoming processes an incoming block, so we can take into account the
// overlap between block IDs and heights.
func (e *Engine) processIncomingBlock(originID flow.Identifier, block *flow.Block) {

	shouldProcess := e.core.HandleBlock(block.Header)
	if !shouldProcess {
		return
	}

	synced := &events.SyncedBlock{
		OriginID: originID,
		Block:    block,
	}

	e.comp.SubmitLocal(synced)
}

// checkLoop will regularly scan for items that need requesting.
func (e *Engine) checkLoop() {
	pollChan := make(<-chan time.Time)
	if e.pollInterval > 0 {
		poll := time.NewTicker(e.pollInterval)
		pollChan = poll.C
		defer poll.Stop()
	}
	scan := time.NewTicker(e.scanInterval)

CheckLoop:
	for {
		// give the quit channel a priority to be selected
		select {
		case <-e.unit.Quit():
			break CheckLoop
		default:
		}

		select {
		case <-e.unit.Quit():
			break CheckLoop
		case <-pollChan:
			err := e.pollHeight()
			if err != nil {
				e.log.Error().Err(err).Msg("could not poll heights")
			}

		case <-scan.C:
			final, err := e.state.Final().Head()
			if err != nil {
				e.log.Error().Err(err).Msg("could not get final height")
				continue
			}

			e.unit.Lock()
			ranges, batches := e.core.ScanPending(final)
			err = e.sendRequests(ranges, batches)
			if err != nil {
				e.log.Error().Err(err).Msg("could not send requests")
			}
			e.unit.Unlock()
		}
	}

	// some minor cleanup
	scan.Stop()
}

// pollHeight will send a synchronization request to three random nodes.
func (e *Engine) pollHeight() error {

	// get the last finalized header
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get last finalized header: %w", err)
	}

	// get all of the consensus nodes from the state
	participants, err := e.state.Final().Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))

	if err != nil {
		return fmt.Errorf("could not send get consensus identities: %w", err)
	}

	// send the request for synchronization
	req := &messages.SyncRequest{
		Nonce:  rand.Uint64(),
		Height: final.Height,
	}
	err = e.con.Multicast(req, synccore.DefaultPollNodes, participants.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not send sync request: %w", err)
	}

	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageSyncRequest)

	return err
}

// sendRequests sends a request for each range and batch.
func (e *Engine) sendRequests(ranges []flow.Range, batches []flow.Batch) error {

	participants, err := e.state.Final().Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get participants: %w", err)
	}

	var errs error
	for _, ran := range ranges {
		req := &messages.RangeRequest{
			Nonce:      rand.Uint64(),
			FromHeight: ran.From,
			ToHeight:   ran.To,
		}
		err := e.con.Multicast(req, synccore.DefaultBlockRequestNodes, participants.NodeIDs()...)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not submit range request: %w", err))
			continue
		}
		e.log.Debug().
			Uint64("range_from", req.FromHeight).
			Uint64("range_to", req.ToHeight).
			Uint64("range_nonce", req.Nonce).
			Msg("range requested")
		e.core.RangeRequested(ran)
		e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageRangeRequest)
	}

	for _, batch := range batches {
		req := &messages.BatchRequest{
			Nonce:    rand.Uint64(),
			BlockIDs: batch.BlockIDs,
		}
		err := e.con.Multicast(req, synccore.DefaultBlockRequestNodes, participants.NodeIDs()...)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not submit batch request: %w", err))
			continue
		}
		e.core.BatchRequested(batch)
		e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBatchRequest)
	}

	return errs
}
