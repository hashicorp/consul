package haproxy2consul

// This file comes from https://github.com/haproxytech/haproxy-consul-connect/
// Please don't modify it without syncing it with its origin
// Logger Allows replacing easily the logger.
type Logger interface {
	// Debugf Display debug message
	Debugf(format string, args ...interface{})
	// Infof Display info message
	Infof(format string, args ...interface{})
	// Warnf Display warning message
	Warnf(format string, args ...interface{})
	// Errorf Display error message
	Errorf(format string, args ...interface{})
}
