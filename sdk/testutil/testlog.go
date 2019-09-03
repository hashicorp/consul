package testutil

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

var sendTestLogsToStdout bool

func init() {
	sendTestLogsToStdout = os.Getenv("NOLOGBUFFER") == "1"
}

func TestLogger(t testing.TB) *log.Logger {
	return log.New(&testWriter{t}, t.Name()+": ", log.LstdFlags)
}

func TestLoggerWithName(t testing.TB, name string) *log.Logger {
	return log.New(&testWriter{t}, "test["+name+"]: ", log.LstdFlags)
}

func TestWriter(t testing.TB) io.Writer {
	return &testWriter{t}
}

type testWriter struct {
	t testing.TB
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.t.Helper()
	if sendTestLogsToStdout {
		fmt.Fprint(os.Stdout, strings.TrimSpace(string(p))+"\n")
	} else {
		tw.t.Log(strings.TrimSpace(string(p)))
	}
	return len(p), nil
}
