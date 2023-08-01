// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

import (
	"io"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func Test_GetPolicyIDByName_Builtins(t *testing.T) {
	t.Parallel()

	a := agent.StartTestAgent(t,
		agent.TestAgent{
			LogOutput: io.Discard,
			HCL: `
				primary_datacenter = "dc1"
				acl {
					enabled = true
					tokens {
						initial_management = "root"
					}
				}
			`,
		},
	)

	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WithToken("root"))

	client := a.Client()
	client.AddHeader("X-Consul-Token", "root")

	t.Run("global management policy", func(t *testing.T) {
		id, err := GetPolicyIDByName(client, structs.ACLPolicyGlobalManagementName)
		require.NoError(t, err)
		require.Equal(t, structs.ACLPolicyGlobalManagementID, id)
	})

	t.Run("global read-only policy", func(t *testing.T) {
		id, err := GetPolicyIDByName(client, structs.ACLPolicyGlobalReadOnlyName)
		require.NoError(t, err)
		require.Equal(t, structs.ACLPolicyGlobalReadOnlyID, id)
	})
}

func Test_GetPolicyIDFromPartial_Builtins(t *testing.T) {
	t.Parallel()

	a := agent.StartTestAgent(t,
		agent.TestAgent{
			LogOutput: io.Discard,
			HCL: `
				primary_datacenter = "dc1"
				acl {
					enabled = true
					tokens {
						initial_management = "root"
					}
				}
			`,
		},
	)

	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WithToken("root"))

	client := a.Client()
	client.AddHeader("X-Consul-Token", "root")

	t.Run("global management policy", func(t *testing.T) {
		id, err := GetPolicyIDFromPartial(client, structs.ACLPolicyGlobalManagementName)
		require.NoError(t, err)
		require.Equal(t, structs.ACLPolicyGlobalManagementID, id)
	})

	t.Run("global read-only policy", func(t *testing.T) {
		id, err := GetPolicyIDFromPartial(client, structs.ACLPolicyGlobalReadOnlyName)
		require.NoError(t, err)
		require.Equal(t, structs.ACLPolicyGlobalReadOnlyID, id)
	})
}
