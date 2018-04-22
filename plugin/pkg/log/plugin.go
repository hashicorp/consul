package log

import (
	"fmt"
	golog "log"

	"github.com/coredns/coredns/plugin"
)

// P is a logger that includes the plugin doing the logging.
type P struct {
	plugin string
}

// NewWithPlugin return a logger that shows the plugin that logs the message.
// I.e [INFO] plugin/<name>: message.
func NewWithPlugin(h plugin.Handler) P { return P{h.Name()} }

func (p P) logf(level, format string, v ...interface{}) {
	s := level + pFormat(p.plugin) + fmt.Sprintf(format, v...)
	golog.Print(s)
}

func (p P) log(level string, v ...interface{}) {
	s := level + pFormat(p.plugin) + fmt.Sprint(v...)
	golog.Print(s)
}

// Debug logs as log.Debug.
func (p P) Debug(v ...interface{}) {
	if !D {
		return
	}
	p.log(debug, v...)
}

// Debugf logs as log.Debugf.
func (p P) Debugf(format string, v ...interface{}) {
	if !D {
		return
	}
	p.logf(debug, format, v...)
}

// Info logs as log.Info.
func (p P) Info(v ...interface{}) { p.log(info, v...) }

// Infof logs as log.Infof.
func (p P) Infof(format string, v ...interface{}) { p.logf(info, format, v...) }

// Warning logs as log.Warning.
func (p P) Warning(v ...interface{}) { p.log(warning, v...) }

// Warningf logs as log.Warningf.
func (p P) Warningf(format string, v ...interface{}) { p.logf(warning, format, v...) }

// Error logs as log.Error.
func (p P) Error(v ...interface{}) { p.log(err, v...) }

// Errorf logs as log.Errorf.
func (p P) Errorf(format string, v ...interface{}) { p.logf(err, format, v...) }

func pFormat(s string) string { return "plugin/" + s + ": " }
