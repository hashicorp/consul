package trace

import (
	"testing"

	"github.com/mholt/caddy"
)

// createTestTrace creates a trace plugin to be used in tests
func createTestTrace(config string) (*caddy.Controller, *trace, error) {
	c := caddy.NewTestController("dns", config)
	m, err := traceParse(c)
	return c, m, err
}

func TestTrace(t *testing.T) {
	_, m, err := createTestTrace(`trace`)
	if err != nil {
		t.Errorf("Error parsing test input: %s", err)
		return
	}
	if m.Name() != "trace" {
		t.Errorf("Wrong name from GetName: %s", m.Name())
	}
	err = m.OnStartup()
	if err != nil {
		t.Errorf("Error starting tracing plugin: %s", err)
		return
	}
	if m.Tracer() == nil {
		t.Errorf("Error, no tracer created")
	}
}
