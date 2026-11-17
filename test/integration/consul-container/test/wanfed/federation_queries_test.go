// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package wanfed

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// TestWanFed_CrossDatacenterQueries verifies that, once datacenters are
// federated, a client of one datacenter can read and write data in the other by
// naming it, that data stays local when it is not named, and that prepared
// queries fail over between datacenters.
func TestWanFed_CrossDatacenterQueries(t *testing.T) {
	t.Parallel()

	f := newFederation(t)
	remote := &api.QueryOptions{Datacenter: secondaryDC}

	t.Run("each datacenter knows the other, with network coordinates", func(t *testing.T) {
		for name, c := range map[string]*api.Client{"primary": f.pc, "secondary": f.sc} {
			dcs, err := c.Catalog().Datacenters()
			require.NoError(t, err, name)
			require.ElementsMatch(t, []string{primaryDC, secondaryDC}, dcs, name)
		}

		eventually(t, func(r *retry.R) {
			maps, err := f.pc.Coordinate().Datacenters()
			require.NoError(r, err)

			var dcs []string
			for _, m := range maps {
				dcs = append(dcs, m.Datacenter)
				require.NotEmpty(r, m.Coordinates, "datacenter %s has no coordinates", m.Datacenter)
			}
			require.ElementsMatch(r, []string{primaryDC, secondaryDC}, dcs)
		})
	})

	t.Run("catalog and health queries can target the other datacenter", func(t *testing.T) {
		require.NoError(t, f.sc.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID:    "remote-svc-1",
			Name:  "remote-svc",
			Port:  9000,
			Tags:  []string{"edge"},
			Check: &api.AgentServiceCheck{TTL: "10m", Status: api.HealthPassing},
		}))
		secondaryNode, err := f.sc.Agent().NodeName()
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			svcs, _, err := f.pc.Catalog().Service("remote-svc", "", remote)
			require.NoError(r, err)
			require.Len(r, svcs, 1)
			require.Equal(r, secondaryNode, svcs[0].Node)
			require.Equal(r, 9000, svcs[0].ServicePort)
			require.Equal(r, []string{"edge"}, svcs[0].ServiceTags)

			entries, _, err := f.pc.Health().Service("remote-svc", "", true, remote)
			require.NoError(r, err)
			require.Len(r, entries, 1)
			require.Equal(r, secondaryDC, entries[0].Node.Datacenter)

			all, _, err := f.pc.Catalog().Services(remote)
			require.NoError(r, err)
			require.Contains(r, all, "remote-svc")

			nodes, _, err := f.pc.Catalog().Nodes(remote)
			require.NoError(r, err)
			var names []string
			for _, n := range nodes {
				names = append(names, n.Node)
			}
			require.Contains(r, names, secondaryNode)
		})

		local, _, err := f.pc.Catalog().Service("remote-svc", "", nil)
		require.NoError(t, err)
		require.Empty(t, local, "a query that does not name a datacenter stays local")
	})

	t.Run("an unknown datacenter is an error", func(t *testing.T) {
		_, _, err := f.pc.Catalog().Nodes(&api.QueryOptions{Datacenter: "does-not-exist"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "No path to datacenter")
	})

	t.Run("KV is addressed per datacenter", func(t *testing.T) {
		const key = "app/greeting"

		_, err := f.pc.KV().Put(&api.KVPair{Key: key, Value: []byte("from-primary")}, nil)
		require.NoError(t, err)
		_, err = f.pc.KV().Put(&api.KVPair{Key: key, Value: []byte("written-remotely")}, &api.WriteOptions{Datacenter: secondaryDC})
		require.NoError(t, err)

		local, _, err := f.pc.KV().Get(key, nil)
		require.NoError(t, err)
		require.Equal(t, "from-primary", string(local.Value))

		viaPrimary, _, err := f.pc.KV().Get(key, remote)
		require.NoError(t, err)
		require.Equal(t, "written-remotely", string(viaPrimary.Value))

		fromSecondary, _, err := f.sc.KV().Get(key, nil)
		require.NoError(t, err)
		require.Equal(t, "written-remotely", string(fromSecondary.Value), "the write landed in the secondary datacenter")
	})

	t.Run("prepared queries fail over to the other datacenter", func(t *testing.T) {
		pq := f.pc.PreparedQuery()

		for name, failover := range map[string]api.QueryFailoverOptions{
			"remote-nearest":  {NearestN: 1},
			"remote-explicit": {Datacenters: []string{secondaryDC}},
		} {
			_, _, err := pq.Create(&api.PreparedQueryDefinition{
				Name:    name,
				Service: api.ServiceQuery{Service: "remote-svc", Failover: failover},
			}, nil)
			require.NoError(t, err, name)

			eventually(t, func(r *retry.R) {
				resp, _, err := pq.Execute(name, nil)
				require.NoError(r, err, name)
				require.Equal(r, secondaryDC, resp.Datacenter, name)
				require.Equal(r, 1, resp.Failovers, name)
				require.Len(r, resp.Nodes, 1, name)
			})
		}

		_, _, err := pq.Create(&api.PreparedQueryDefinition{
			Name:    "remote-no-failover",
			Service: api.ServiceQuery{Service: "remote-svc"},
		}, nil)
		require.NoError(t, err)
		resp, _, err := pq.Execute("remote-no-failover", nil)
		require.NoError(t, err)
		require.Equal(t, primaryDC, resp.Datacenter)
		require.Zero(t, resp.Failovers)
		require.Empty(t, resp.Nodes, "without failover the query only looks at the local datacenter")
	})

	t.Run("prepared queries are not replicated", func(t *testing.T) {
		_, _, err := f.sc.PreparedQuery().Execute("remote-nearest", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Query not found")
	})
}
