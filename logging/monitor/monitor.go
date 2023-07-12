package monitor

import (
	"errors"
	"sync"

	log "github.com/hashicorp/go-hclog"
)

// Monitor provides a mechanism to stream logs using go-hclog
// InterceptLogger and SinkAdapter. It allows streaming of logs
// at a different log level than what is set on the logger.
type Monitor interface {
	// Start returns a channel of log messages which are sent
	// ever time a log message occurs
	Start() <-chan []byte

	// Stop deregisters the sink from the InterceptLogger and closes the log
	// channels. This returns a count of the number of log messages that were
	// dropped during streaming.
	Stop() int
}

// monitor implements the Monitor interface
type monitor struct {
	sink log.SinkAdapter

	// logger is the logger we will be monitoring
	logger log.InterceptLogger

	// logCh is a buffered chan where we send logs when streaming
	logCh chan []byte

	// droppedCount is the current count of messages
	// that were dropped from the logCh buffer.
	droppedCount int

	// doneCh coordinates the shutdown of logCh
	doneCh chan struct{}

	// Defaults to 512.
	bufSize int
}

type Config struct {
	BufferSize    int
	Logger        log.InterceptLogger
	LoggerOptions *log.LoggerOptions
}

// New creates a new Monitor. Start must be called in order to actually start
// streaming logs
func New(cfg Config) Monitor {
	bufSize := cfg.BufferSize
	if bufSize == 0 {
		bufSize = 512
	}

	sw := &monitor{
		logger:  cfg.Logger,
		logCh:   make(chan []byte, bufSize),
		doneCh:  make(chan struct{}, 1),
		bufSize: bufSize,
	}

	cfg.LoggerOptions.Output = sw
	sink := log.NewSinkAdapter(cfg.LoggerOptions)
	sw.sink = sink

	return sw
}

// Stop deregisters the sink and stops the monitoring process
func (d *monitor) Stop() int {
	d.logger.DeregisterSink(d.sink)
	close(d.doneCh)
	return d.droppedCount
}

// Start registers a sink on the monitor's logger and starts sending
// received log messages over the returned channel.
func (d *monitor) Start() <-chan []byte {
	// register our sink with the logger
	streamCh := make(chan []byte, d.bufSize)
	// run a go routine that listens for streamed
	// log messages and sends them to streamCh

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer close(streamCh)
		wg.Done()
		for {
			select {
			case logLine := <-d.logCh:
				select {
				case <-d.doneCh:
					return
				case streamCh <- logLine:
				}
			case <-d.doneCh:
				return
			}
		}
	}()

	//wait for the consumer loop to start before registering the sink to avoid filling the log channel
	wg.Wait()
	d.logger.RegisterSink(d.sink)
	return streamCh
}

// Write attempts to send latest log to logCh
// it drops the log if channel is unavailable to receive
func (d *monitor) Write(p []byte) (int, error) {
	// Check whether we have been stopped
	select {
	case <-d.doneCh:
		return 0, errors.New("monitor stopped")
	default:
	}

	bytes := make([]byte, len(p))
	copy(bytes, p)

	select {
	case d.logCh <- bytes:
	default:
		d.droppedCount++
	}

	return len(p), nil
}
