// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"sync"

	"github.com/hashicorp/go-hclog"
)

// startupLogger is a shim that allows signal handling (and anything else) to
// log to an appropriate output throughout several startup phases. Initially
// when bootstrapping from HCP we need to log caught signals direct to the UI
// output since logging is not setup yet and won't be if we are interrupted
// before we try to start the agent itself. Later, during agent.Start we could
// block retrieving auto TLS or auto-config from servers so need to handle
// signals, but in this case logging has already started so we should log the
// signal event to the logger.
type startupLogger struct {
	mu     sync.Mutex
	logger hclog.Logger
}

func newStartupLogger() *startupLogger {
	return &startupLogger{
		// Start off just using defaults for hclog since this is too early to have
		// parsed logging config even and we just want to get _something_ out to the
		// user.
		logger: hclog.New(&hclog.LoggerOptions{
			Name: "agent.startup",
			// Nothing else output in UI has a time prefix until logging is properly
			// setup so use the same prefix as other "Info" lines to make it look less
			// strange. Note one less space than in PrefixedUI since hclog puts a
			// space between the time prefix and log line already.
			TimeFormat: "   ",
		}),
	}
}

func (l *startupLogger) SetLogger(logger hclog.Logger) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger = logger
}

func (l *startupLogger) Info(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger.Info(msg, args...)
}

func (l *startupLogger) Warn(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger.Warn(msg, args...)
}

func (l *startupLogger) Error(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger.Error(msg, args...)
}
