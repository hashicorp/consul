// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package kv

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/serf/coordinate"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func coordinateNodes(t retry.TestingTB, c *api.Client) map[string]*api.CoordinateEntry {
	entries, _, err := c.Coordinate().Nodes(nil)
	require.NoError(t, err)

	out := map[string]*api.CoordinateEntry{}
	for _, entry := range entries {
		out[entry.Node] = entry
	}
	return out
}

// TestCoordinates verifies the network coordinates Consul maintains for every
// node, which drive nearest-first sorting and `consul rtt`.
func TestCoordinates(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})
	serverNode, err := e.server.Agent().NodeName()
	require.NoError(t, err)
	clientNode, err := e.client.Agent().NodeName()
	require.NoError(t, err)

	t.Run("every agent publishes a coordinate", func(t *testing.T) {
		// Coordinates are batched before they reach the servers, so allow for the
		// update period.
		eventually(t, func(r *retry.R) {
			nodes := coordinateNodes(r, e.client)
			for _, name := range []string{serverNode, clientNode} {
				entry, ok := nodes[name]
				require.Truef(r, ok, "no coordinate for %s yet", name)
				require.Empty(r, entry.Segment)
				require.Len(r, entry.Coord.Vec, 8, "coordinates are 8-dimensional")
				require.True(r, entry.Coord.IsValid(), "coordinate %+v is not valid", entry.Coord)
			}
		})
	})

	t.Run("a single node can be read", func(t *testing.T) {
		entries, _, err := e.client.Coordinate().Node(clientNode, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, clientNode, entries[0].Node)

		_, _, err = e.client.Coordinate().Node("no-such-node", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "404")
	})

	t.Run("the datacenter view lists the servers on the WAN", func(t *testing.T) {
		dcs, err := e.client.Coordinate().Datacenters()
		require.NoError(t, err)
		require.Len(t, dcs, 1)
		require.Equal(t, "dc1", dcs[0].Datacenter)
		require.Equal(t, "wan", dcs[0].AreaID)
		require.Len(t, dcs[0].Coordinates, 1, "only servers are in the WAN pool")
		require.Equal(t, serverNode, dcs[0].Coordinates[0].Node)
	})

	t.Run("a coordinate can be written for a node in the catalog", func(t *testing.T) {
		_, err := e.server.Catalog().Register(&api.CatalogRegistration{Node: "external-node", Address: "10.5.5.5"}, nil)
		require.NoError(t, err)

		coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
		for i := range coord.Vec {
			coord.Vec[i] = 0.25
		}
		_, err = e.server.Coordinate().Update(&api.CoordinateEntry{Node: "external-node", Coord: coord}, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			entry, ok := coordinateNodes(r, e.client)["external-node"]
			require.True(r, ok)
			require.InDeltaSlice(r, coord.Vec, entry.Coord.Vec, 1e-9)
		})
	})

	t.Run("a coordinate with the wrong dimensions is rejected", func(t *testing.T) {
		bad := &coordinate.Coordinate{Vec: []float64{0.1, 0.2, 0.3}, Error: 1, Height: 0.001}
		_, err := e.server.Coordinate().Update(&api.CoordinateEntry{Node: "external-node", Coord: bad}, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "incompatible coordinate")
	})

	t.Run("catalog queries can be sorted by distance from a node", func(t *testing.T) {
		for _, name := range []string{serverNode, clientNode} {
			nodes, _, err := e.client.Catalog().Nodes(&api.QueryOptions{Near: name})
			require.NoError(t, err)
			require.NotEmpty(t, nodes)
			require.Equal(t, name, nodes[0].Node, "the node itself is the nearest")
		}
	})
}

// TestCoordinates_RTTCLI covers `consul rtt`.
func TestCoordinates_RTTCLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})
	serverNode, err := e.server.Agent().NodeName()
	require.NoError(t, err)
	clientNode, err := e.client.Agent().NodeName()
	require.NoError(t, err)

	estimate := regexp.MustCompile(`Estimated (\S+) <-> (\S+) rtt: [0-9.]+ ms \(using LAN coordinates\)`)

	t.Run("the estimated round trip time between two nodes is reported", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			res := e.shell(t, "consul rtt "+serverNode+" "+clientNode)
			require.Equal(r, 0, res.ExitCode, res.Output)

			match := estimate.FindStringSubmatch(res.Output)
			require.NotNil(r, match, res.Output)
			require.Equal(r, serverNode, match[1])
			require.Equal(r, clientNode, match[2])
		})
	})

	t.Run("the estimate is symmetric", func(t *testing.T) {
		forward := e.shell(t, "consul rtt "+serverNode+" "+clientNode)
		backward := e.shell(t, "consul rtt "+clientNode+" "+serverNode)

		require.Equal(t, 0, forward.ExitCode, forward.Output)
		require.Equal(t, 0, backward.ExitCode, backward.Output)
		f := regexp.MustCompile(`rtt: ([0-9.]+) ms`).FindStringSubmatch(forward.Output)
		b := regexp.MustCompile(`rtt: ([0-9.]+) ms`).FindStringSubmatch(backward.Output)
		require.NotNil(t, f)
		require.NotNil(t, b)
		require.Equal(t, f[1], b[1])
	})

	t.Run("a node without a coordinate is an error", func(t *testing.T) {
		res := e.shell(t, "consul rtt "+serverNode+" no-such-node")

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, `Could not find a coordinate for node "no-such-node"`)
	})

	t.Run("server nodes can be compared over the WAN pool", func(t *testing.T) {
		res := e.shell(t, "consul rtt -wan "+serverNode+".dc1 "+serverNode+".dc1")

		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "using WAN coordinates")
	})
}

// TestCoordinates_ACLs verifies that coordinates are guarded by node ACLs.
func TestCoordinates_ACLs(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{acl: true})
	serverNode, err := e.server.Agent().NodeName()
	require.NoError(t, err)

	allNodes := e.newToken(t, "coord-all-nodes", `node_prefix "" { policy = "read" }`)
	oneNode := e.newToken(t, "coord-one-node", `node "`+serverNode+`" { policy = "read" }`)
	nothing := e.newToken(t, "coord-nothing", `service_prefix "" { policy = "read" }`)

	t.Run("only coordinates of readable nodes are listed", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			require.GreaterOrEqual(r, len(coordinateNodes(r, allNodes)), 2)
		})

		visible := coordinateNodes(t, oneNode)
		require.Len(t, visible, 1)
		require.Contains(t, visible, serverNode)

		require.Empty(t, coordinateNodes(t, nothing))
	})

	t.Run("writing a coordinate needs node:write", func(t *testing.T) {
		entry := &api.CoordinateEntry{Node: serverNode, Coord: coordinate.NewCoordinate(coordinate.DefaultConfig())}

		_, err := allNodes.Coordinate().Update(entry, nil)
		requireDenied(t, err, "a read-only token")
	})
}
