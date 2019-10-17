package log

import (
	"bytes"
	golog "log"
	"strings"
	"testing"
)

func TestDebug(t *testing.T) {
	var f bytes.Buffer
	golog.SetOutput(&f)

	// D == false
	Debug("debug")
	if x := f.String(); x != "" {
		t.Errorf("Expected no debug logs, got %s", x)
	}
	f.Reset()

	D.Set()
	Debug("debug")
	if x := f.String(); !strings.Contains(x, debug+"debug") {
		t.Errorf("Expected debug log to be %s, got %s", debug+"debug", x)
	}
	f.Reset()

	D.Clear()
	Debug("debug")
	if x := f.String(); x != "" {
		t.Errorf("Expected no debug logs, got %s", x)
	}
}

func TestDebugx(t *testing.T) {
	var f bytes.Buffer
	golog.SetOutput(&f)

	D.Set()

	Debugf("%s", "debug")
	if x := f.String(); !strings.Contains(x, debug+"debug") {
		t.Errorf("Expected debug log to be %s, got %s", debug+"debug", x)
	}
	f.Reset()

	Debug("debug")
	if x := f.String(); !strings.Contains(x, debug+"debug") {
		t.Errorf("Expected debug log to be %s, got %s", debug+"debug", x)
	}
}

func TestLevels(t *testing.T) {
	var f bytes.Buffer
	const ts = "test"
	golog.SetOutput(&f)

	Info(ts)
	if x := f.String(); !strings.Contains(x, info+ts) {
		t.Errorf("Expected log to be %s, got %s", info+ts, x)
	}
	f.Reset()
	Warning(ts)
	if x := f.String(); !strings.Contains(x, warning+ts) {
		t.Errorf("Expected log to be %s, got %s", warning+ts, x)
	}
	f.Reset()
	Error(ts)
	if x := f.String(); !strings.Contains(x, err+ts) {
		t.Errorf("Expected log to be %s, got %s", err+ts, x)
	}
}
