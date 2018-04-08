package cachetype

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestRPC returns a mock implementation of the RPC interface.
func TestRPC(t testing.T) *MockRPC {
	// This function is relatively useless but this allows us to perhaps
	// perform some initialization later.
	return &MockRPC{}
}
