package local

import (
	"os"

	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/go-testing-interface"
)

// TestState returns a configured *State for testing.
func TestState(t testing.T) *State {
	consulLogger := hclog.New(&hclog.LoggerOptions{
		Output: os.Stderr,
	})
	logger := consulLogger.StandardLogger(&hclog.StandardLoggerOptions{
		InferLevels: true,
	})
	result := NewState(Config{}, logger, consulLogger, &token.Store{})
	result.TriggerSyncChanges = func() {}
	return result
}
