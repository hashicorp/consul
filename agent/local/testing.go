package local

import (
	"log"
	"os"

	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/go-testing-interface"
)

// TestState returns a configured *State for testing.
func TestState(t testing.T) *State {
	consulLogger := hclog.New(&hclog.LoggerOptions{
		Level:  log.LstdFlags,
		Output: os.Stderr,
	})
	logger := consulLogger.StandardLogger(&hclog.StandardLoggerOptions{
		InferLevels: true,
	})
	result := NewState(Config{}, logger, &token.Store{})
	result.TriggerSyncChanges = func() {}
	return result
}
