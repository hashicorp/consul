// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// TestACL_DefaultDenyEnforcement verifies, area by area, that with a default
// deny policy an operation succeeds only for tokens granted the matching rule.
// Each token holds exactly one rule, so every cell of the table is a statement
// about one permission.
func TestACL_DefaultDenyEnforcement(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")

	rules := map[string]string{
		"catalog-reader":  `node_prefix "" { policy = "read" } service_prefix "" { policy = "read" }`,
		"catalog-writer":  `node_prefix "" { policy = "write" } service_prefix "" { policy = "write" }`,
		"agent-reader":    `agent_prefix "" { policy = "read" }`,
		"agent-writer":    `agent_prefix "" { policy = "write" }`,
		"operator-reader": `operator = "read"`,
		"operator-writer": `operator = "write"`,
		"acl-reader":      `acl = "read"`,
		"query-writer":    `query_prefix "" { policy = "write" } service_prefix "" { policy = "read" }`,
		"intention-admin": `service "db" { policy = "read" intentions = "write" }`,
		"nothing":         `service "unrelated" { policy = "read" }`,
	}
	clients := map[string]*api.Client{}
	for name, rule := range rules {
		_, clients[name] = e.newToken(t, name, rule)
	}

	autopilot, err := e.adminClient.Operator().AutopilotGetConfiguration(nil)
	require.NoError(t, err)

	var queryCount int
	registration := &api.CatalogRegistration{Node: "external-node", Address: "10.1.1.1", Service: &api.AgentService{Service: "external", Port: 80}}

	type operation struct {
		name    string
		run     func(c *api.Client) error
		allowed []string
	}
	operations := []operation{
		{
			name: "catalog: register a service",
			run: func(c *api.Client) error {
				_, err := c.Catalog().Register(registration, nil)
				return err
			},
			allowed: []string{"catalog-writer"},
		},
		{
			name: "agent: read the agent's own configuration",
			run: func(c *api.Client) error {
				_, err := c.Agent().Self()
				return err
			},
			allowed: []string{"agent-reader", "agent-writer"},
		},
		{
			name: "agent: change the agent's default token",
			run: func(c *api.Client) error {
				_, err := c.Agent().UpdateDefaultACLToken("", nil)
				return err
			},
			allowed: []string{"agent-writer"},
		},
		{
			name: "operator: read the raft configuration",
			run: func(c *api.Client) error {
				_, err := c.Operator().RaftGetConfiguration(nil)
				return err
			},
			allowed: []string{"operator-reader", "operator-writer"},
		},
		{
			name: "operator: read usage",
			run: func(c *api.Client) error {
				_, _, err := c.Operator().Usage(nil)
				return err
			},
			allowed: []string{"operator-reader", "operator-writer"},
		},
		{
			name: "operator: change the autopilot configuration",
			run: func(c *api.Client) error {
				return c.Operator().AutopilotSetConfiguration(autopilot, nil)
			},
			allowed: []string{"operator-writer"},
		},
		{
			name: "acl: list tokens",
			run: func(c *api.Client) error {
				_, _, err := c.ACL().TokenList(nil)
				return err
			},
			allowed: []string{"acl-reader"},
		},
		{
			name: "acl: create a policy",
			run: func(c *api.Client) error {
				queryCount++
				_, _, err := c.ACL().PolicyCreate(&api.ACLPolicy{Name: fmt.Sprintf("created-by-test-%d", queryCount)}, nil)
				return err
			},
			allowed: nil,
		},
		{
			name: "prepared queries: create",
			run: func(c *api.Client) error {
				queryCount++
				_, _, err := c.PreparedQuery().Create(&api.PreparedQueryDefinition{
					Name: fmt.Sprintf("query-%d", queryCount), Service: api.ServiceQuery{Service: "external"},
				}, nil)
				return err
			},
			allowed: []string{"query-writer"},
		},
		{
			name: "intentions: write for the destination the token administers",
			run: func(c *api.Client) error {
				_, _, err := c.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
					Kind: api.ServiceIntentions, Name: "db",
					Sources: []*api.SourceIntention{{Name: "web", Action: api.IntentionActionAllow}},
				}, nil)
				return err
			},
			allowed: []string{"intention-admin"},
		},
		{
			name: "intentions: write for any other destination",
			run: func(c *api.Client) error {
				_, _, err := c.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
					Kind: api.ServiceIntentions, Name: "other",
					Sources: []*api.SourceIntention{{Name: "web", Action: api.IntentionActionAllow}},
				}, nil)
				return err
			},
			allowed: nil,
		},
		{
			name: "snapshot: save",
			run: func(c *api.Client) error {
				rc, _, err := c.Snapshot().Save(nil)
				if rc != nil {
					rc.Close()
				}
				return err
			},
			allowed: nil,
		},
	}

	names := make([]string, 0, len(clients))
	for name := range clients {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, op := range operations {
		op := op
		t.Run(op.name, func(t *testing.T) {
			allowed := map[string]bool{}
			for _, name := range op.allowed {
				allowed[name] = true
			}

			for _, name := range names {
				err := op.run(clients[name])
				if allowed[name] {
					require.NoError(t, err, "%s should be allowed", name)
				} else {
					requireDenied(t, err, name+" should be denied")
				}
			}

			// The management token can do everything.
			require.NoError(t, op.run(e.adminClient), "the bootstrap token should be allowed")
		})
	}
}

// TestACL_ReadsAreFilteredNotRefused verifies that listing operations do not
// fail for tokens with limited access; they return only what the token may see.
func TestACL_ReadsAreFilteredNotRefused(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	for _, svc := range []string{"visible", "hidden"} {
		require.NoError(t, e.adminClient.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID: svc + "-1", Name: svc, Port: 9000,
			Check: &api.AgentServiceCheck{TTL: "10m", Status: api.HealthPassing},
		}))
	}

	_, limited := e.newToken(t, "sees-visible", `service "visible" { policy = "read" } node_prefix "" { policy = "read" }`)
	_, nothing := e.newToken(t, "sees-nothing", `service "other" { policy = "read" }`)

	serviceNames := func(r retry.TestingTB, c *api.Client) []string {
		services, _, err := c.Catalog().Services(nil)
		require.NoError(r, err)
		var names []string
		for name := range services {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}

	eventually(t, func(r *retry.R) {
		require.Subset(r, serviceNames(r, e.adminClient), []string{"consul", "hidden", "visible"})
		require.Equal(r, []string{"visible"}, serviceNames(r, limited), "only the readable service is listed")
		require.Empty(r, serviceNames(r, nothing), "a token with no access sees an empty catalog, not an error")
	})

	t.Run("instances and health follow the same rule", func(t *testing.T) {
		instances, _, err := limited.Catalog().Service("visible", "", nil)
		require.NoError(t, err)
		require.Len(t, instances, 1)

		hidden, _, err := limited.Catalog().Service("hidden", "", nil)
		require.NoError(t, err)
		require.Empty(t, hidden)

		entries, _, err := limited.Health().Service("hidden", "", false, nil)
		require.NoError(t, err)
		require.Empty(t, entries)

		checks, _, err := limited.Health().State(api.HealthAny, nil)
		require.NoError(t, err)
		for _, chk := range checks {
			require.NotEqual(t, "hidden", chk.ServiceName, "checks of hidden services are filtered out")
		}
	})
}
