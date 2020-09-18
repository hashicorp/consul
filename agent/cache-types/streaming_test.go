package cachetype

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// TestStreamingClient is a mock StreamingClient for testing that allows
// for queueing up custom events to a subscriber.
type TestStreamingClient struct {
	events chan eventOrErr
	ctx    context.Context
}

type eventOrErr struct {
	Err   error
	Event *pbsubscribe.Event
}

func NewTestStreamingClient() *TestStreamingClient {
	return &TestStreamingClient{
		events: make(chan eventOrErr, 32),
	}
}

func (t *TestStreamingClient) Subscribe(
	ctx context.Context,
	_ *pbsubscribe.SubscribeRequest,
	_ ...grpc.CallOption,
) (pbsubscribe.StateChangeSubscription_SubscribeClient, error) {
	t.ctx = ctx
	return t, nil
}

func (t *TestStreamingClient) QueueEvents(events ...*pbsubscribe.Event) {
	for _, e := range events {
		t.events <- eventOrErr{Event: e}
	}
}

func (t *TestStreamingClient) QueueErr(err error) {
	t.events <- eventOrErr{Err: err}
}

func (t *TestStreamingClient) Recv() (*pbsubscribe.Event, error) {
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
