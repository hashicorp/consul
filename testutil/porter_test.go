package testutil

import (
	"testing"

	"github.com/hashicorp/consul/test/porter"
)

func Test_PorterIsRunning(t *testing.T) {
	if _, err := porter.RandomPorts(1); err != nil {
		t.Fatalf("Porter must be running for Consul's unit tests")
	}
}
