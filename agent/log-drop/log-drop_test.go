package logdrop

import (
	"context"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewLogDrop(t *testing.T) {
	mockLogger := NewMockLogger(t)
	mockLogger.On("Log", hclog.Info, "hello", []interface{}{"test", 0}).Return()
	ld := NewLogDropSink(context.Background(), "test", 10, mockLogger, func(_ Log) {})
	require.NotNil(t, ld)
	ld.Accept("test Log", hclog.Info, "hello", "test", 0)
	retry.Run(t, func(r *retry.R) {
		mockLogger.AssertNumberOfCalls(r, "Log", 1)
	})

}

func TestLogDroppedWhenChannelFilled(t *testing.T) {
	mockLogger := NewMockLogger(t)
	ctx, cancelFunc := context.WithCancel(context.Background())
	called := false
	cancelFunc()
	ld := NewLogDropSink(ctx, "test", 1, mockLogger, func(l Log) {
		called = true
	})
	for i := 0; i < 2; i++ {
		ld.Accept("test", hclog.Debug, "hello")
	}
	mockLogger.AssertNotCalled(t, "Log", mock.Anything)
	require.True(t, called)
}
