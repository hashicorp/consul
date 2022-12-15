package logdrop

import (
	"context"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNewLogDrop(t *testing.T) {
	mockLogger := NewMockSinkAdapter(t)
	mockLogger.On("Accept", "test Accept", hclog.Info, "hello", []interface{}{"test", 0}).Return()
	ld := NewLogDropSink(context.Background(), "test", 10, mockLogger, func(_ Log) {})
	require.NotNil(t, ld)
	ld.Accept("test Accept", hclog.Info, "hello", "test", 0)
	retry.Run(t, func(r *retry.R) {
		mockLogger.AssertNumberOfCalls(r, "Accept", 1)
	})

}

func TestLogDroppedWhenChannelFilled(t *testing.T) {
	mockLogger := NewMockSinkAdapter(t)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	block := make(chan interface{})
	mockLogger.On("Accept", "test", hclog.Debug, "hello", []interface{}(nil)).Run(func(args mock.Arguments) {
		<-block
	})

	var called = make(chan interface{})
	ld := NewLogDropSink(ctx, "test", 1, mockLogger, func(l Log) {
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
		mockLogger.AssertNumberOfCalls(r, "Accept", 1)
	})
}
