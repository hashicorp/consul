// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package wanfed

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

const replicationPolicyRules = `
acl      = "write"
operator = "write"

service_prefix "" {
  policy     = "read"
  intentions = "read"
}
`

var retryFuncTimer = &retry.Timer{
	Timeout: 60 * time.Second,
	Wait:    2 * time.Second,
}

// retryFunc is a helper to retry the given function until it returns a nil
// error. It returns the value from the function on success.
func retryFunc[T any](t *testing.T, f func() (T, error)) T {
	var result T
	retry.RunWith(retryFuncTimer, t, func(r *retry.R) {
		val, err := f()
		require.NoError(r, err)
		result = val
	})
	return result
}

// TestWanFed_ReplicationBootstrap checks the setup procedure of two federated
// datacenters with a manual ACL bootstrap step and ACL token replication enabled.
//
//   - WAN join two clusters that are not ACL bootstrapped
//     and have ACL replication enabled
//   - ACL Bootstrap the primary datacenter
//   - Configure the replication token in each datacenter
//   - Validate ACL replication by creating a token
//     and checking it is found in each datacenter.
//
// Added to validate this issue: https://github.com/hashicorp/consul/issues/16620
func TestWanFed_ReplicationBootstrap(t *testing.T) {
	cfgFunc := func(c *libcluster.ConfigBuilder) {
		c.Set("primary_datacenter", "primary")
		c.Set("acl.enabled", true)
		c.Set("acl.default_policy", "deny")
		c.Set("acl.enable_token_persistence", true)
		c.Set("acl.enable_token_replication", true)
	}

	_, primary := createNonACLBootstrappedCluster(t, "primary", cfgFunc)
	_, secondary := createNonACLBootstrappedCluster(t, "secondary", func(c *libcluster.ConfigBuilder) {
		cfgFunc(c)
		c.Set("retry_join_wan", []string{primary.GetIP()})
	})

	// ACL bootstrap the primary
	rootToken := retryFunc(t, func() (*api.ACLToken, error) {
		t.Logf("ACL bootstrap the primary datacenter")
		tok, _, err := primary.GetClient().ACL().Bootstrap()
		return tok, err
	})

	agents := []libcluster.Agent{primary, secondary}

	for _, agent := range agents {
		// Make a new client with the bootstrap token.
		_, err := agent.NewClient(rootToken.SecretID, true)
		require.NoError(t, err)

		t.Logf("Wait for members in %s datacenter", agent.GetDatacenter())
		libcluster.WaitForMembers(t, agent.GetClient(), 1)
	}

	// Create and set the replication token
	replicationPolicy := retryFunc(t, func() (*api.ACLPolicy, error) {
		t.Logf("Create the replication policy")
		p, _, err := primary.GetClient().ACL().PolicyCreate(
			&api.ACLPolicy{
				Name:  "consul-server-replication",
				Rules: replicationPolicyRules,
			},
			nil,
		)
		return p, err
	})
	replicationToken := retryFunc(t, func() (*api.ACLToken, error) {
		t.Logf("Create the replication policy")
		tok, _, err := primary.GetClient().ACL().TokenCreate(
			&api.ACLToken{
				Description: "Consul server replication token",
				Policies:    []*api.ACLLink{{ID: replicationPolicy.ID}},
			},
			nil,
		)
		return tok, err
	})

	// Set the replication token in the secondary
	retryFunc(t, func() (any, error) {
		t.Logf("Set the replication token in the %s datacenter", secondary.GetDatacenter())
		return secondary.GetClient().Agent().UpdateReplicationACLToken(replicationToken.SecretID, nil)
	})

	// Double check that replication happens after setting the replication token.
	// - Create a token
	// - Check the token is found in each datacenter
	createdToken := retryFunc(t, func() (*api.ACLToken, error) {
		t.Logf("Create a test token to validate ACL replication in the secondary datacenter")
		tok, _, err := secondary.GetClient().ACL().TokenCreate(&api.ACLToken{
			ServiceIdentities: []*api.ACLServiceIdentity{{
				ServiceName: "test-svc",
			}},
		}, nil)
		return tok, err
	})
	for _, agent := range agents {
		tok := retryFunc(t, func() (*api.ACLToken, error) {
			t.Logf("Check the test token is found in the %s datacenter", agent.GetDatacenter())
			tok, _, err := agent.GetClient().ACL().TokenRead(createdToken.AccessorID, nil)
			return tok, err
		})
		require.NotNil(t, tok)
		require.Equal(t, tok.AccessorID, createdToken.AccessorID)
		require.Equal(t, tok.SecretID, createdToken.SecretID)
	}
}

func createNonACLBootstrappedCluster(t *testing.T, dc string, f func(c *libcluster.ConfigBuilder)) (*libcluster.Cluster, libcluster.Agent) {
	ctx := libcluster.NewBuildContext(t, libcluster.BuildOptions{Datacenter: dc})
	conf := libcluster.NewConfigBuilder(ctx).Advanced(f)

	cluster, err := libcluster.New(t, []libcluster.Config{*conf.ToAgentConfig(t)})
	require.NoError(t, err)

	client := cluster.Agents[0].GetClient()

	libcluster.WaitForLeader(t, cluster, client)
	// Note: Do not wait for members yet (not ACL bootstrapped so no perms with a default deny)

	agent, err := cluster.Leader()
	require.NoError(t, err)
	return cluster, agent
}
