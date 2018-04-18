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

	D = true
	Debug("debug")
	if x := f.String(); !strings.Contains(x, debug+"debug") {
		t.Errorf("Expected debug log to be %s, got %s", debug+"debug", x)
	}
}

func TestDebugx(t *testing.T) {
	var f bytes.Buffer
	golog.SetOutput(&f)

	D = true

	Debugf("%s", "debug")
	if x := f.String(); !strings.Contains(x, debug+"debug") {
		t.Errorf("Expected debug log to be %s, got %s", debug+"debug", x)
	}

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
	Warning(ts)
	if x := f.String(); !strings.Contains(x, warning+ts) {
		t.Errorf("Expected log to be %s, got %s", warning+ts, x)
	}
	Error(ts)
	if x := f.String(); !strings.Contains(x, err+ts) {
		t.Errorf("Expected log to be %s, got %s", err+ts, x)
	}
}
