package log

import (
	"bytes"
	golog "log"
	"strings"
	"testing"
)

func TestPlugins(t *testing.T) {
	var f bytes.Buffer
	const ts = "test"
	golog.SetOutput(&f)

	lg := NewWithPlugin("testplugin")

	lg.Info(ts)
	if x := f.String(); !strings.Contains(x, "plugin/testplugin") {
		t.Errorf("Expected log to be %s, got %s", info+ts, x)
	}
}
