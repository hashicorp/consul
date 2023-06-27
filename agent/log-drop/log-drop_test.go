// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package logdrop

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestNewLogDrop(t *testing.T) {
	mockLogger := NewMockLogger(t)
	mockLogger.On("Log", hclog.Info, "hello", "test", 0).Return()
	ld := NewLogDropSink(context.Background(), 10, mockLogger, func(_ Log) {})
	require.NotNil(t, ld)
	ld.Accept("test Log", hclog.Info, "hello", "test", 0)
	retry.Run(t, func(r *retry.R) {
		mockLogger.AssertNumberOfCalls(r, "Log", 1)
	})

}

func TestLogDroppedWhenChannelFilled(t *testing.T) {
	mockLogger := NewMockLogger(t)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	block := make(chan interface{})
	mockLogger.On("Log", hclog.Debug, "hello").Run(func(args mock.Arguments) {
		<-block
	})

	var called = make(chan interface{})
	ld := NewLogDropSink(ctx, 1, mockLogger, func(l Log) {
		close(called)
	})
	for i := 0; i < 2; i++ {
		ld.Accept("test", hclog.Debug, "hello")
	}

	select {
	case <-called:
		close(block)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for drop func to be called")
	}
	retry.Run(t, func(r *retry.R) {
		mockLogger.AssertNumberOfCalls(r, "Log", 1)
	})
}
