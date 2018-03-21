package consul

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Register the test RPC endpoint
	TestEndpoint()

	os.Exit(m.Run())
}
