package log_drop

import (
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNewLogDrop(t *testing.T) {
	mockLogger := NewMockLogger(t)
	mockLogger.On("Info", "test Log", []interface{}{"val", 0}).Return()
	ld := NewLogDrop(context.Background(), "test", 10, mockLogger, func(_ Log) {})
	require.NotNil(t, ld)
	ld.Info("test Log", "val", 0)
	time.Sleep(10 * time.Millisecond)
	mockLogger.AssertNumberOfCalls(t, "Info", 1)
}

func TestLogDroppedWhenChannelFilled(t *testing.T) {
	mockLogger := NewMockLogger(t)
	ctx, cancelFunc := context.WithCancel(context.Background())
	called := false
	ld := NewLogDrop(ctx, "test", 1, mockLogger, func(l Log) {
		called = true
	})
	cancelFunc()
	time.Sleep(1 * time.Second)
	for i := 0; i < 2; i++ {
		ld.Info("test", "test", "hello")
	}
	mockLogger.AssertNotCalled(t, "Info", mock.Anything)
	require.True(t, called)
}
