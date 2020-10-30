package cachetype

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// TestStreamingClient is a mock StreamingClient for testing that allows
// for queueing up custom events to a subscriber.
type TestStreamingClient struct {
	pbsubscribe.StateChangeSubscription_SubscribeClient
	events            chan eventOrErr
	ctx               context.Context
	expectedNamespace string
}

type eventOrErr struct {
	Err   error
	Event *pbsubscribe.Event
}

func NewTestStreamingClient(ns string) *TestStreamingClient {
	return &TestStreamingClient{
		events:            make(chan eventOrErr, 32),
		expectedNamespace: ns,
	}
}

func (t *TestStreamingClient) Subscribe(
	ctx context.Context,
	req *pbsubscribe.SubscribeRequest,
	_ ...grpc.CallOption,
) (pbsubscribe.StateChangeSubscription_SubscribeClient, error) {
	if req.Namespace != t.expectedNamespace {
		return nil, fmt.Errorf("wrong SubscribeRequest.Namespace %v, expected %v",
			req.Namespace, t.expectedNamespace)
	}
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
