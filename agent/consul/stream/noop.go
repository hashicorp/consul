// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package stream

import (
	"context"
	"fmt"
)

type NoOpEventPublisher struct{}

func (NoOpEventPublisher) Publish([]Event) {}

func (NoOpEventPublisher) RegisterHandler(Topic, SnapshotFunc, bool) error {
	return fmt.Errorf("stream event publisher is disabled")
}

func (NoOpEventPublisher) Run(context.Context) {}

func (NoOpEventPublisher) Subscribe(*SubscribeRequest) (*Subscription, error) {
	return nil, fmt.Errorf("stream event publisher is disabled")
}
