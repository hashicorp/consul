package haproxy2consul

// This file comes from https://github.com/haproxytech/haproxy-consul-connect/
// Please don't modify it without syncing it with its origin
import "testing"

type testingLogger struct {
	t *testing.T
}

// Debugf Display debug message
func (l *testingLogger) Debugf(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

// Infof Display info message
func (l *testingLogger) Infof(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

// Warnf Display warning message
func (l *testingLogger) Warnf(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

// Errorf Display error message
func (l *testingLogger) Errorf(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

// NewTestingLogger creates a Logger for testing.T
func NewTestingLogger(t *testing.T) Logger {
	return &testingLogger{t: t}
}
