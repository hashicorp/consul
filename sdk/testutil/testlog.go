package testutil

import (
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/hashicorp/go-hclog"
)

var sendTestLogsToStdout bool

func init() {
	sendTestLogsToStdout = os.Getenv("NOLOGBUFFER") == "1"
}

func NewDiscardLogger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Level:  0,
		Output: ioutil.Discard,
	})
}

func Logger(t testing.TB) hclog.InterceptLogger {
	return LoggerWithOutput(t, os.Stdout)
}

func LoggerWithOutput(t testing.TB, output io.Writer) hclog.InterceptLogger {
	return hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:       t.Name(),
		Level:      hclog.Trace,
		Output:     output,
		TimeFormat: "04:05.000",
	})
}

func LoggerWithName(name string) hclog.InterceptLogger {
	return hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:       "test[" + name + "]",
		Level:      hclog.Debug,
		Output:     os.Stdout,
		TimeFormat: "04:05.000",
	})
}
