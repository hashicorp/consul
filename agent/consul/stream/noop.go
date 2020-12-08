package stream

import (
	"context"
	"fmt"
)

type NoOpEventPublisher struct{}

func (NoOpEventPublisher) Publish([]Event) {}

func (NoOpEventPublisher) Run(context.Context) {}

func (NoOpEventPublisher) Subscribe(*SubscribeRequest) (*Subscription, error) {
	return nil, fmt.Errorf("stream event publisher is disabled")
}
