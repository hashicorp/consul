package cachetype

import (
	"context"

	"github.com/hashicorp/consul/agent/agentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TestStreamingClient is a mock StreamingClient for testing that allows
// for queueing up custom events to a subscriber.
type TestStreamingClient struct {
	events chan eventOrErr
	ctx    context.Context
}

type eventOrErr struct {
	Err   error
	Event *agentpb.Event
}

func NewTestStreamingClient() *TestStreamingClient {
	return &TestStreamingClient{
		events: make(chan eventOrErr, 32),
	}
}

func (t *TestStreamingClient) Subscribe(ctx context.Context, in *agentpb.SubscribeRequest, opts ...grpc.CallOption) (agentpb.Consul_SubscribeClient, error) {
	t.ctx = ctx

	return t, nil
}

func (t *TestStreamingClient) QueueEvents(events ...*agentpb.Event) {
	for _, e := range events {
		t.events <- eventOrErr{Event: e}
	}
}

func (t *TestStreamingClient) QueueErr(err error) {
	t.events <- eventOrErr{Err: err}
}

func (t *TestStreamingClient) Recv() (*agentpb.Event, error) {
	select {
	case eoe := <-t.events:
		if eoe.Err != nil {
			return nil, eoe.Err
		}
		return eoe.Event, nil
	case <-t.ctx.Done():
		return nil, t.ctx.Err()
	}
}

func (t *TestStreamingClient) Header() (metadata.MD, error) { return nil, nil }

func (t *TestStreamingClient) Trailer() metadata.MD { return nil }

func (t *TestStreamingClient) CloseSend() error { return nil }

func (t *TestStreamingClient) Context() context.Context { return nil }

func (t *TestStreamingClient) SendMsg(m interface{}) error { return nil }

func (t *TestStreamingClient) RecvMsg(m interface{}) error { return nil }
