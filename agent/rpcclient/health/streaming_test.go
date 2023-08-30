// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package health

import (
	"context"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// streamClient is a mock StreamingClient for testing that allows
// for queueing up custom events to a subscriber.
type streamClient struct {
	pbsubscribe.StateChangeSubscription_SubscribeClient
	subFn  func(*pbsubscribe.SubscribeRequest) error
	events chan eventOrErr
	ctx    context.Context
}

type eventOrErr struct {
	Err   error
	Event *pbsubscribe.Event
}

func newStreamClient(sub func(req *pbsubscribe.SubscribeRequest) error) *streamClient {
	if sub == nil {
		sub = func(*pbsubscribe.SubscribeRequest) error {
			return nil
		}
	}
	return &streamClient{
		events: make(chan eventOrErr, 32),
		subFn:  sub,
	}
}

func (t *streamClient) Subscribe(
	ctx context.Context,
	req *pbsubscribe.SubscribeRequest,
	_ ...grpc.CallOption,
) (pbsubscribe.StateChangeSubscription_SubscribeClient, error) {
	if err := t.subFn(req); err != nil {
		return nil, err
	}
	t.ctx = ctx
	return t, nil
}

func (t *streamClient) QueueEvents(events ...*pbsubscribe.Event) {
	for _, e := range events {
		t.events <- eventOrErr{Event: e}
	}
}

func (t *streamClient) QueueErr(err error) {
	t.events <- eventOrErr{Err: err}
}

func (t *streamClient) Recv() (*pbsubscribe.Event, error) {
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
