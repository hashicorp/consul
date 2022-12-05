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
	mockLogger.On("Info", "test log", []interface{}{"val", 0}).Return()
	ld := NewLogDrop(context.Background(), "test", mockLogger)
	require.NotNil(t, ld)
	ld.Info("test log", "val", 0)
	time.Sleep(10 * time.Millisecond)
	mockLogger.AssertNumberOfCalls(t, "Info", 1)
}

func TestLogDroppedWhenChannelFilled(t *testing.T) {
	mockLogger := NewMockLogger(t)
	ctx, cancelFunc := context.WithCancel(context.Background())

	ld := NewLogDrop(ctx, "test", mockLogger)
	cancelFunc()
	time.Sleep(1 * time.Second)
	for i := 0; i < logCHDepth+1; i++ {
		ld.Info("test", "test", "hello")
	}
	mockLogger.AssertNotCalled(t, "Info", mock.Anything)
}
