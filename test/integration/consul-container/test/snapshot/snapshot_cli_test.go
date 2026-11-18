// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package snapshot

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

func eventually(t *testing.T, check func(r *retry.R)) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, check)
}

// state is the part of the datacenter's data that these tests compare.
type state struct {
	KV            map[string]string
	Nodes         []string
	Services      map[string][]string
	ServiceConfig []string
	Queries       []string
}

func capture(t retry.TestingTB, c *api.Client) state {
	t.Helper()
	s := state{KV: map[string]string{}}

	pairs, _, err := c.KV().List("", nil)
	if err != nil {
		require.Contains(t, err.Error(), "404")
	}
	for _, p := range pairs {
		s.KV[p.Key] = string(p.Value)
	}

	// The server registers itself in the catalog at its own pace, so only the
	// nodes created by the tests are compared.
	self, err := c.Agent().NodeName()
	require.NoError(t, err)
	nodes, _, err := c.Catalog().Nodes(nil)
	require.NoError(t, err)
	for _, n := range nodes {
		if n.Node != self {
			s.Nodes = append(s.Nodes, n.Node)
		}
	}

	s.Services, _, err = c.Catalog().Services(nil)
	require.NoError(t, err)
	delete(s.Services, "consul")

	entries, _, err := c.ConfigEntries().List(api.ServiceDefaults, nil)
	require.NoError(t, err)
	for _, e := range entries {
		s.ServiceConfig = append(s.ServiceConfig, e.GetName())
	}

	queries, _, err := c.PreparedQuery().List(nil)
	require.NoError(t, err)
	for _, q := range queries {
		s.Queries = append(s.Queries, q.Name)
	}
	return s
}

// populate writes data of several kinds and returns what the datacenter holds
// afterwards.
func populate(t *testing.T, c *api.Client) state {
	t.Helper()

	for k, v := range map[string]string{"app/a": "1", "app/b": "2", "app/c": "3"} {
		_, err := c.KV().Put(&api.KVPair{Key: k, Value: []byte(v)}, nil)
		require.NoError(t, err)
	}

	_, err := c.Catalog().Register(&api.CatalogRegistration{
		Node:    "external-node",
		Address: "192.0.2.10",
		Service: &api.AgentService{ID: "ext-1", Service: "ext", Tags: []string{"v1"}, Port: 8080},
		Check: &api.AgentCheck{
			Node: "external-node", CheckID: "ext-check", Name: "ext-check", Status: api.HealthPassing, ServiceID: "ext-1",
		},
	}, nil)
	require.NoError(t, err)

	ok, _, err := c.ConfigEntries().Set(&api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "web", Protocol: "http"}, nil)
	require.NoError(t, err)
	require.True(t, ok)

	_, _, err = c.PreparedQuery().Create(&api.PreparedQueryDefinition{
		Name:    "find-ext",
		Service: api.ServiceQuery{Service: "ext"},
	}, nil)
	require.NoError(t, err)

	return capture(t, c)
}

// kvRows returns the number of KVS rows that `snapshot inspect` reports.
func kvRows(t *testing.T, inspect string) string {
	for _, line := range strings.Split(inspect, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "KVS" {
			return fields[1]
		}
	}
	require.FailNow(t, "no KVS row in snapshot inspect output", inspect)
	return ""
}

// TestSnapshot_CLI saves, inspects and restores a snapshot with the CLI and
// checks that restoring returns every kind of data to what it was when the
// snapshot was taken.
func TestSnapshot_CLI(t *testing.T) {
	e := newEnv(t, false)
	c := e.admin
	snap := e.file(t, "state.snap")

	before := populate(t, c)
	require.Len(t, before.KV, 3)

	t.Run("save writes and verifies a snapshot", func(t *testing.T) {
		out, code := e.cli(t, "", "snapshot", "save", snap)
		require.Equal(t, 0, code, out)
		require.Contains(t, out, "Saved and verified snapshot to index")
	})

	t.Run("inspect describes the snapshot", func(t *testing.T) {
		out, code := e.cli(t, "", "snapshot", "inspect", snap)
		require.Equal(t, 0, code, out)
		for _, want := range []string{"ID", "Size", "Index", "Term", "Version", "KVS", "Register", "Total"} {
			require.Contains(t, out, want)
		}
		require.Equal(t, "3", kvRows(t, out))
	})

	t.Run("restore brings back what was deleted and drops what was added", func(t *testing.T) {
		_, err := c.KV().Delete("app/a", nil)
		require.NoError(t, err)
		_, err = c.KV().Put(&api.KVPair{Key: "app/new", Value: []byte("x")}, nil)
		require.NoError(t, err)
		_, err = c.Catalog().Deregister(&api.CatalogDeregistration{Node: "external-node"}, nil)
		require.NoError(t, err)
		_, err = c.Catalog().Register(&api.CatalogRegistration{
			Node: "late-node", Address: "192.0.2.20", Service: &api.AgentService{ID: "late-1", Service: "late"},
		}, nil)
		require.NoError(t, err)
		_, err = c.ConfigEntries().Delete(api.ServiceDefaults, "web", nil)
		require.NoError(t, err)
		queries, _, err := c.PreparedQuery().List(nil)
		require.NoError(t, err)
		for _, q := range queries {
			_, err = c.PreparedQuery().Delete(q.ID, nil)
			require.NoError(t, err)
		}
		require.NotEqual(t, before, capture(t, c), "the data was changed after the snapshot")

		out, code := e.cli(t, "", "snapshot", "restore", snap)
		require.Equal(t, 0, code, out)
		require.Contains(t, out, "Restored snapshot")

		eventually(t, func(r *retry.R) {
			require.Equal(r, before, capture(r, c))
		})
	})

	t.Run("a snapshot can be restored more than once", func(t *testing.T) {
		_, err := c.KV().Put(&api.KVPair{Key: "app/a", Value: []byte("changed")}, nil)
		require.NoError(t, err)

		out, code := e.cli(t, "", "snapshot", "restore", snap)
		require.Equal(t, 0, code, out)

		eventually(t, func(r *retry.R) {
			require.Equal(r, before, capture(r, c))
		})
	})

	t.Run("a second save to the same file replaces it", func(t *testing.T) {
		_, err := c.KV().Put(&api.KVPair{Key: "app/d", Value: []byte("4")}, nil)
		require.NoError(t, err)

		out, code := e.cli(t, "", "snapshot", "save", snap)
		require.Equal(t, 0, code, out)

		out, code = e.cli(t, "", "snapshot", "inspect", snap)
		require.Equal(t, 0, code, out)
		require.Equal(t, "4", kvRows(t, out))
	})

	t.Run("bad input is refused", func(t *testing.T) {
		garbage := e.file(t, "garbage.snap")
		e.writeFile(t, garbage, "not-a-snapshot")

		out, code := e.cli(t, "", "snapshot", "inspect", garbage)
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "Error reading snapshot")

		out, code = e.cli(t, "", "snapshot", "restore", garbage)
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "Error restoring snapshot")

		out, code = e.cli(t, "", "snapshot", "restore", e.file(t, "missing.snap"))
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "Error opening snapshot file")

		out, code = e.cli(t, "", "snapshot", "save")
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "Missing FILE argument")

		after := capture(t, c)
		require.Equal(t, "4", after.KV["app/d"], "refused restores change nothing")
	})
}
