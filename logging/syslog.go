package logging

import (
	"bytes"

	gsyslog "github.com/hashicorp/go-syslog"
)

// levelPriority is used to map a log level to a
// syslog priority level
var levelPriority = map[string]gsyslog.Priority{
	"TRACE": gsyslog.LOG_DEBUG,
	"DEBUG": gsyslog.LOG_INFO,
	"INFO":  gsyslog.LOG_NOTICE,
	"WARN":  gsyslog.LOG_WARNING,
	"ERROR": gsyslog.LOG_ERR,
	"CRIT":  gsyslog.LOG_CRIT,
}

// SyslogWrapper is used to cleanup log messages before
// writing them to a Syslogger. Implements the io.Writer
// interface.
type SyslogWrapper struct {
	l gsyslog.Syslogger
}

// Write is used to implement io.Writer
func (s *SyslogWrapper) Write(p []byte) (int, error) {
	// Extract log level
	var level string
	afterLevel := p
	x := bytes.IndexByte(p, '[')
	if x >= 0 {
		y := bytes.IndexByte(p[x:], ']')
		if y >= 0 {
			level = string(p[x+1 : x+y])
			afterLevel = bytes.TrimLeft(p[x+y+2:], " ")
		}
	}

	// Each log level will be handled by a specific syslog priority
	priority, ok := levelPriority[level]
	if !ok {
		priority = gsyslog.LOG_NOTICE
	}

	// Attempt the write
	err := s.l.WriteLevel(priority, afterLevel)
	return len(p), err
}
