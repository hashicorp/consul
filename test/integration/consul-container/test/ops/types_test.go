// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ops

import (
	"testing"

	"github.com/hashicorp/consul/api"
)

// node is one Consul agent of the datacenter under test.
type node interface {
	Client() *api.Client
	// HTTPAddr is the host:port of the agent's HTTP port as seen by the test.
	HTTPAddr() string
	// CLI runs `consul <args>` against this agent and returns the combined
	// output and the exit code.
	CLI(t *testing.T, args ...string) (string, int)
	// WriteConfig creates or replaces a file in the agent's configuration
	// directory. name must end in .hcl or .json.
	WriteConfig(t *testing.T, name, content string)
	RemoveConfig(t *testing.T, name string)
}

type env struct {
	server node
	client node
}
