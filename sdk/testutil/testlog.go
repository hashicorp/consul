package testutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
)

var sendTestLogsToStdout bool

func init() {
	sendTestLogsToStdout = os.Getenv("NOLOGBUFFER") == "1"
}

func TestHcLog(t testing.TB) *log.Logger {
	consulLogger := hclog.New(&hclog.LoggerOptions{
		Name:   t.Name(),
		Output: TestWriter(t),
	})
	return consulLogger.StandardLogger(&hclog.StandardLoggerOptions{
		InferLevels: true,
	})
}

func NewDiscardLogger() *log.Logger {
	consulLogger := hclog.New(&hclog.LoggerOptions{
		Level:  0,
		Output: ioutil.Discard,
	})
	return consulLogger.StandardLogger(&hclog.StandardLoggerOptions{
		InferLevels: true,
	})
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
	if tw.t != nil {
		tw.t.Helper()
	}
	if sendTestLogsToStdout || tw.t == nil {
		fmt.Fprint(os.Stdout, strings.TrimSpace(string(p))+"\n")
	} else {
		defer func() {
			if r := recover(); r != nil {
				if sr, ok := r.(string); ok {
					if strings.HasPrefix(sr, "Log in goroutine after ") {
						// These sorts of panics are undesirable, but requires
						// total control over goroutine lifetimes to correct.
						fmt.Fprint(os.Stdout, "SUPPRESSED PANIC: "+sr+"\n")
						return
					}
				}
				panic(r)
			}
		}()
		tw.t.Log(strings.TrimSpace(string(p)))
	}
	return len(p), nil
}
