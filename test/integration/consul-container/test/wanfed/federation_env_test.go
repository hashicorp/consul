// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package wanfed

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

const (
	primaryDC   = "primary"
	secondaryDC = "secondary"
)

// federation is two single-server datacenters joined over the WAN.
type federation struct {
	primary   libcluster.Agent
	secondary libcluster.Agent
	// pc and sc are clients to the primary and secondary servers. When ACLs are
	// enabled they carry the primary datacenter's bootstrap token, which is
	// valid in both datacenters once ACLs are replicated.
	pc *api.Client
	sc *api.Client
}

// newFederation creates a primary and a secondary datacenter, federates them
// and waits until each can see the other.
func newFederation(t *testing.T) *federation {
	t.Helper()

	_, primary := createCluster(t, primaryDC, func(c *libcluster.ConfigBuilder) {
		c.Set("primary_datacenter", primaryDC)
	})
	_, secondary := createCluster(t, secondaryDC, func(c *libcluster.ConfigBuilder) {
		c.Set("primary_datacenter", primaryDC)
		c.Set("retry_join_wan", []string{primary.GetIP()})
	})

	f := &federation{primary: primary, secondary: secondary, pc: primary.GetClient(), sc: secondary.GetClient()}
	f.waitForFederation(t)
	return f
}

// newACLFederation is newFederation with default-deny ACLs and full ACL
// replication (policies, roles and tokens) from the primary to the secondary.
func newACLFederation(t *testing.T) *federation {
	t.Helper()

	aclConfig := func(c *libcluster.ConfigBuilder) {
		c.Set("primary_datacenter", primaryDC)
		c.Set("acl.enabled", true)
		c.Set("acl.default_policy", "deny")
		c.Set("acl.enable_token_persistence", true)
		c.Set("acl.enable_token_replication", true)
	}

	_, primary := createNonACLBootstrappedCluster(t, primaryDC, aclConfig)
	_, secondary := createNonACLBootstrappedCluster(t, secondaryDC, func(c *libcluster.ConfigBuilder) {
		aclConfig(c)
		c.Set("retry_join_wan", []string{primary.GetIP()})
	})

	rootToken := retryFunc(t, func() (*api.ACLToken, error) {
		tok, _, err := primary.GetClient().ACL().Bootstrap()
		return tok, err
	})
	for _, agent := range []libcluster.Agent{primary, secondary} {
		_, err := agent.NewClient(rootToken.SecretID, true)
		require.NoError(t, err)
		libcluster.WaitForMembers(t, agent.GetClient(), 1)
	}

	policy := retryFunc(t, func() (*api.ACLPolicy, error) {
		p, _, err := primary.GetClient().ACL().PolicyCreate(
			&api.ACLPolicy{Name: "consul-server-replication", Rules: replicationPolicyRules}, nil)
		return p, err
	})
	replicationToken := retryFunc(t, func() (*api.ACLToken, error) {
		tok, _, err := primary.GetClient().ACL().TokenCreate(&api.ACLToken{
			Description: "Consul server replication token",
			Policies:    []*api.ACLLink{{ID: policy.ID}},
		}, nil)
		return tok, err
	})
	retryFunc(t, func() (any, error) {
		return secondary.GetClient().Agent().UpdateReplicationACLToken(replicationToken.SecretID, nil)
	})

	f := &federation{primary: primary, secondary: secondary, pc: primary.GetClient(), sc: secondary.GetClient()}
	f.waitForFederation(t)
	return f
}

// waitForFederation waits until both datacenters list each other.
func (f *federation) waitForFederation(t *testing.T) {
	t.Helper()

	for _, c := range []*api.Client{f.pc, f.sc} {
		c := c
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			dcs, err := c.Catalog().Datacenters()
			require.NoError(r, err)
			require.ElementsMatch(r, []string{primaryDC, secondaryDC}, dcs)
		})
	}
}

// tokenClient returns a client to the given datacenter's server that
// authenticates as token.
func (f *federation) tokenClient(t *testing.T, agent libcluster.Agent, token string) *api.Client {
	t.Helper()
	c, err := agent.NewClient(token, false)
	require.NoError(t, err)
	return c
}

// eventually retries check until it passes, absorbing replication lag.
func eventually(t *testing.T, check func(r *retry.R)) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, check)
}
