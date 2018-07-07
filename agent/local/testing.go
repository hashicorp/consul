package local

import (
	"log"
	"os"

	"github.com/hashicorp/consul/agent/token"
	"github.com/mitchellh/go-testing-interface"
)

// TestState returns a configured *State for testing.
func TestState(t testing.T) *State {
	result := NewState(Config{
		ProxyBindMinPort: 20000,
		ProxyBindMaxPort: 20500,
	}, log.New(os.Stderr, "", log.LstdFlags), &token.Store{})
	result.TriggerSyncChanges = func() {}
	return result
}
