package cachetype

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/stream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func NewTestStreamingClient() *TestStreamingClient {
	return &TestStreamingClient{
		events: make(chan *stream.Event, 32),
	}
}

type TestStreamingClient struct {
	events chan *stream.Event
	ctx    context.Context
}

func (t *TestStreamingClient) Subscribe(ctx context.Context, in *stream.SubscribeRequest, opts ...grpc.CallOption) (stream.Consul_SubscribeClient, error) {
	t.ctx = ctx

	return t, nil
}

func (t *TestStreamingClient) QueueEvents(events ...*stream.Event) {
	for _, e := range events {
		t.events <- e
	}
}

func (t *TestStreamingClient) Recv() (*stream.Event, error) {
	select {
	case e := <-t.events:
		return e, nil
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
