package testutil

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/go-hclog"
)

func Logger(t TestingTB) hclog.InterceptLogger {
	return LoggerWithOutput(t, NewLogBuffer(t))
}

func LoggerWithOutput(t TestingTB, output io.Writer) hclog.InterceptLogger {
	return hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:   t.Name(),
		Level:  hclog.Trace,
		Output: output,
	})
}

var sendTestLogsToStdout = os.Getenv("NOLOGBUFFER") == "1"
var testLogOnlyFailed = os.Getenv("TEST_LOGGING_ONLY_FAILED") == "1"

// NewLogBuffer returns an io.Writer which buffers all writes. When the test
// ends, t.Failed is checked. If the test has failed or has been run in verbose
// mode all log output is printed to stdout.
//
// Set the env var NOLOGBUFFER=1 to disable buffering, resulting in all log
// output being written immediately to stdout.
//
// Typically log output is written either for failed tests or when go test
// is running with the verbose flag (-v) set. Setting TEST_LOGGING_ONLY_FAILED=1
// will prevent logs being output when the verbose flag is set if the test
// case is successful.
func NewLogBuffer(t TestingTB) io.Writer {
	if sendTestLogsToStdout {
		return os.Stdout
	}
	buf := &logBuffer{buf: new(bytes.Buffer)}
	t.Cleanup(func() {
		if t.Failed() || (!testLogOnlyFailed && testing.Verbose()) {
			buf.Lock()
			defer buf.Unlock()
			buf.buf.WriteTo(os.Stdout)
		}
	})
	return buf
}

type logBuffer struct {
	buf *bytes.Buffer
	sync.Mutex
}

func (lb *logBuffer) Write(p []byte) (n int, err error) {
	lb.Lock()
	defer lb.Unlock()
	return lb.buf.Write(p)
}
