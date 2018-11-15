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

func TestPluginsDateTime(t *testing.T) {
	var f bytes.Buffer
	const ts = "test"
	golog.SetFlags(0) // Set to 0 because we're doing our own time, with timezone
	golog.SetOutput(&f)

	lg := NewWithPlugin("testplugin")

	lg.Info(ts)
	// rude check if the date/time is there
	str := f.String()
	if str[4] != '-' || str[7] != '-' || str[10] != 'T' {
		t.Errorf("Expected date got %s...", str[:15])
	}
}
