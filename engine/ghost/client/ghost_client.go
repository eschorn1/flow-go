package client

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"

	ghost "github.com/dapperlabs/flow-go/engine/ghost/protobuf"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	jsoncodec "github.com/dapperlabs/flow-go/network/codec/json"
)

// GhostClient is a client for the Ghost Node
type GhostClient struct {
	rpcClient ghost.GhostNodeAPIClient
	close     func() error
	codec     network.Codec
}

func NewGhostClient(addr string) (*GhostClient, error) {

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	grpcClient := ghost.NewGhostNodeAPIClient(conn)

	return &GhostClient{
		rpcClient: grpcClient,
		close:     func() error { return conn.Close() },
		codec:     jsoncodec.NewCodec(),
	}, nil
}

// Close closes the client connection.
func (c *GhostClient) Close() error {
	return c.close()
}

func (c *GhostClient) Send(ctx context.Context, channelID uint8, targetIDs []flow.Identifier, event interface{}) error {

	message, err := c.codec.Encode(event)
	if err != nil {
		return fmt.Errorf("could not encode event: %w", err)
	}

	targets := make([][]byte, len(targetIDs))
	for i, t := range targetIDs {
		targets[i] = t[:]
	}

	req := ghost.SendEventRequest{
		ChannelId: uint32(channelID),
		TargetID:  targets,
		Message:   message,
	}

	_, err = c.rpcClient.SendEvent(ctx, &req)
	if err != nil {
		return fmt.Errorf("failed to send event to the ghost node: %w", err)
	}
	return nil
}

func (c *GhostClient) Subscribe(ctx context.Context) (*FlowMessageStreamReader, error) {
	req := ghost.SubscribeRequest{}
	stream, err := c.rpcClient.Subscribe(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe for events: %w", err)
	}
	return &FlowMessageStreamReader{stream: stream, codec: c.codec}, nil
}

type FlowMessageStreamReader struct {
	stream ghost.GhostNodeAPI_SubscribeClient
	codec  network.Codec
}

func (fmsr *FlowMessageStreamReader) Next() (flow.Identifier, interface{}, error) {
	msg, err := fmsr.stream.Recv()
	if err == io.EOF {
		// read done.
		return flow.ZeroID, nil, fmt.Errorf("end of stream reached: %w", err)
	}
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("failed to read stream: %w", err)
	}

	event, err := fmsr.codec.Decode(msg.GetMessage())
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("failed to decode event: %w", err)
	}

	originID := flow.HashToID(msg.GetSenderID())

	return originID, event, nil
}