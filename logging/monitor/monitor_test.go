package monitor

import (
	"fmt"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestMonitor_Start(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Error,
	})

	m := New(Config{
		BufferSize: 512,
		Logger:     logger,
		LoggerOptions: &log.LoggerOptions{
			Level: log.Debug},
	})

	logCh := m.Start()
	defer m.Stop()

	logger.Debug("test log")

	for {
		select {
		case log := <-logCh:
			require.Contains(string(log), "[DEBUG] test log")
			return
		case <-time.After(3 * time.Second):
			t.Fatal("Expected to receive from log channel")
		}
	}
}

func TestMonitor_Stop(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Error,
	})

	m := New(Config{
		BufferSize: 512,
		Logger:     logger,
		LoggerOptions: &log.LoggerOptions{
			Level: log.Debug,
		},
	})
	logCh := m.Start()
	logger.Debug("test log")

	require.Eventually(func() bool {
		return len(logCh) == 1
	}, 3*time.Second, 100*time.Millisecond, "expected logCh to have 1 log in it")

	m.Stop()

	// This log line should not be output to the log channel
	logger.Debug("After Stop")

	for {
		select {
		case log := <-logCh:
			if string(log) != "" {
				require.Contains(string(log), "[DEBUG] test log")
			} else {
				return
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Expected to receive from log channel")
		}
	}
}

func TestMonitor_DroppedMessages(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Warn,
	})

	mcfg := Config{
		BufferSize: 5,
		Logger:     logger,
		LoggerOptions: &log.LoggerOptions{
			Level: log.Debug,
		},
	}
	m := New(mcfg)

	doneCh := make(chan struct{})
	defer close(doneCh)

	logCh := m.Start()

	// Overflow the configured buffer size before we attempt to receive from the
	// logCh. We choose (2*bufSize+1) to account for:
	// - buffer size of internal write channel
	// - buffer size of channel returned from Start()
	// - A message guaranteed to be dropped because all buffers are full
	for i := 0; i <= 2*mcfg.BufferSize+1; i++ {
		logger.Debug(fmt.Sprintf("test message %d", i))
	}

	// Make sure we do not stop before the goroutines have time to process.
	require.Eventually(func() bool {
		return len(logCh) == mcfg.BufferSize
	}, 3*time.Second, 100*time.Millisecond, "expected logCh to have a full log buffer")

	dropped := m.Stop()

	// The number of dropped messages is non-deterministic, so we only assert
	// that we dropped at least 1.
	require.GreaterOrEqual(dropped, 1)
}

func TestMonitor_ZeroBufSizeDefault(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Error,
	})

	m := New(Config{
		BufferSize: 0,
		Logger:     logger,
		LoggerOptions: &log.LoggerOptions{
			Level: log.Debug,
		},
	})
	logCh := m.Start()
	defer m.Stop()

	logger.Debug("test log")

	// If we do not default the buffer size, the monitor will be unable to buffer
	// a log line.
	require.Eventually(func() bool {
		return len(logCh) == 1
	}, 3*time.Second, 100*time.Millisecond, "expected logCh to have 1 log buffered")

	for {
		select {
		case log := <-logCh:
			require.Contains(string(log), "[DEBUG] test log")
			return
		case <-time.After(3 * time.Second):
			t.Fatal("Expected to receive from log channel")
		}
	}
}

func TestMonitor_WriteStopped(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	logger := log.NewInterceptLogger(&log.LoggerOptions{
		Level: log.Error,
	})

	mwriter := &monitor{
		logger: logger,
		doneCh: make(chan struct{}, 1),
	}

	mwriter.Stop()
	n, err := mwriter.Write([]byte("write after close"))
	require.Equal(n, 0)
	require.EqualError(err, "monitor stopped")
}
