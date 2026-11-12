// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package catalog

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

// TestCatalog_ServiceRegistrationAndQueries registers services through a
// client agent and verifies the catalog and health views served by the
// servers: tags, meta, tagged addresses, filters and node listings.
func TestCatalog_ServiceRegistrationAndQueries(t *testing.T) {
	t.Parallel()

	_, server, client := newCluster(t)
	nodeName, err := client.Agent().NodeName()
	require.NoError(t, err)

	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:      "web-1",
		Name:    "web",
		Tags:    []string{"primary", "v1"},
		Port:    8080,
		Address: "10.0.0.10",
		Meta:    map[string]string{"env": "prod"},
		TaggedAddresses: map[string]api.ServiceAddress{
			"lan_ipv4": {Address: "10.0.0.10", Port: 8080},
			"wan_ipv4": {Address: "203.0.113.10", Port: 18080},
		},
		Weights: &api.AgentWeights{Passing: 5, Warning: 1},
	}))
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:   "web-2",
		Name: "web",
		Tags: []string{"secondary", "v1"},
		Port: 8081,
		Meta: map[string]string{"env": "staging"},
	}))

	t.Run("service attributes are served by the servers", func(t *testing.T) {
		// Query a server so we know the registration was synced, not just stored locally.
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			byID := map[string]*api.CatalogService{}
			for _, s := range serviceInstances(r, server, "web", nil) {
				byID[s.ServiceID] = s
			}
			require.Len(r, byID, 2)

			web1 := byID["web-1"]
			require.Equal(r, nodeName, web1.Node)
			require.Equal(r, "10.0.0.10", web1.ServiceAddress)
			require.Equal(r, 8080, web1.ServicePort)
			require.ElementsMatch(r, []string{"primary", "v1"}, web1.ServiceTags)
			require.Equal(r, map[string]string{"env": "prod"}, web1.ServiceMeta)
			require.Equal(r, 5, web1.ServiceWeights.Passing)
			require.Equal(r, 1, web1.ServiceWeights.Warning)
			require.Equal(r, api.ServiceAddress{Address: "203.0.113.10", Port: 18080}, web1.ServiceTaggedAddresses["wan_ipv4"])
		})
	})

	t.Run("tag and filter expressions narrow results", func(t *testing.T) {
		byTag, _, err := server.Catalog().Service("web", "primary", nil)
		require.NoError(t, err)
		require.Len(t, byTag, 1)
		require.Equal(t, "web-1", byTag[0].ServiceID)

		byFilter := serviceInstances(t, server, "web", &api.QueryOptions{Filter: `ServiceMeta["env"] == "staging"`})
		require.Len(t, byFilter, 1)
		require.Equal(t, "web-2", byFilter[0].ServiceID)

		healthy, _, err := server.Health().Service("web", "", false, &api.QueryOptions{Filter: `"secondary" in Service.Tags`})
		require.NoError(t, err)
		require.Len(t, healthy, 1)
		require.Equal(t, "web-2", healthy[0].Service.ID)
	})

	t.Run("services and node listings reflect registrations", func(t *testing.T) {
		services, _, err := server.Catalog().Services(nil)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"primary", "secondary", "v1"}, services["web"])

		catalogNode, _, err := server.Catalog().Node(nodeName, nil)
		require.NoError(t, err)
		require.NotNil(t, catalogNode)
		require.Contains(t, catalogNode.Services, "web-1")
		require.Contains(t, catalogNode.Services, "web-2")

		nodes, _, err := server.Catalog().Nodes(nil)
		require.NoError(t, err)
		var names []string
		for _, n := range nodes {
			names = append(names, n.Node)
		}
		require.Contains(t, names, nodeName)
	})

	t.Run("agent deregistration removes the instance from the catalog", func(t *testing.T) {
		require.NoError(t, client.Agent().ServiceDeregister("web-2"))

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			svcs := serviceInstances(r, server, "web", nil)
			require.Len(r, svcs, 1)
			require.Equal(r, "web-1", svcs[0].ServiceID)
		})
	})
}

// TestCatalog_ExternalNodeRegistration covers registering services against
// nodes that do not run a Consul agent, using the catalog endpoints directly.
func TestCatalog_ExternalNodeRegistration(t *testing.T) {
	t.Parallel()

	_, server, _ := newCluster(t)

	_, err := server.Catalog().Register(&api.CatalogRegistration{
		Node:       "external-db",
		Address:    "192.0.2.50",
		NodeMeta:   map[string]string{"rack": "r1"},
		Datacenter: "dc1",
		Service: &api.AgentService{
			ID:      "db-1",
			Service: "db",
			Port:    5432,
			Tags:    []string{"primary"},
		},
		Check: &api.AgentCheck{
			Node:      "external-db",
			CheckID:   "db-1-check",
			Name:      "db health",
			Status:    api.HealthPassing,
			ServiceID: "db-1",
		},
	}, nil)
	require.NoError(t, err)

	node, _, err := server.Catalog().Node("external-db", nil)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.Equal(t, "192.0.2.50", node.Node.Address)
	require.Equal(t, "r1", node.Node.Meta["rack"])
	require.Contains(t, node.Services, "db-1")

	entries, _, err := server.Health().Service("db", "", true, nil)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "192.0.2.50", entries[0].Node.Address)

	// Flip the check to critical through the catalog; the instance must drop
	// out of passing-only queries but still be listed.
	_, err = server.Catalog().Register(&api.CatalogRegistration{
		Node:    "external-db",
		Address: "192.0.2.50",
		Check: &api.AgentCheck{
			Node:      "external-db",
			CheckID:   "db-1-check",
			Name:      "db health",
			Status:    api.HealthCritical,
			ServiceID: "db-1",
		},
	}, nil)
	require.NoError(t, err)

	passing, _, err := server.Health().Service("db", "", true, nil)
	require.NoError(t, err)
	require.Empty(t, passing)
	all, _, err := server.Health().Service("db", "", false, nil)
	require.NoError(t, err)
	require.Len(t, all, 1)

	_, err = server.Catalog().Deregister(&api.CatalogDeregistration{Node: "external-db", ServiceID: "db-1"}, nil)
	require.NoError(t, err)
	require.Empty(t, serviceInstances(t, server, "db", nil))

	_, err = server.Catalog().Deregister(&api.CatalogDeregistration{Node: "external-db"}, nil)
	require.NoError(t, err)
	node, _, err = server.Catalog().Node("external-db", nil)
	require.NoError(t, err)
	require.Nil(t, node)
}

// TestCatalog_Maintenance verifies service and node maintenance mode: the
// instance is marked critical and drops out of passing-only queries until
// maintenance is disabled.
func TestCatalog_Maintenance(t *testing.T) {
	t.Parallel()

	_, server, client := newCluster(t)
	nodeName, err := client.Agent().NodeName()
	require.NoError(t, err)

	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID: "api-1", Name: "api", Port: 9000,
	}))

	passingCount := func(r retry.TestingTB) int {
		entries, _, err := server.Health().Service("api", "", true, nil)
		require.NoError(r, err)
		return len(entries)
	}

	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		require.Equal(r, 1, passingCount(r))
	})

	t.Run("service maintenance", func(t *testing.T) {
		require.NoError(t, client.Agent().EnableServiceMaintenance("api-1", "upgrade"))
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Equal(r, 0, passingCount(r))
			require.Equal(r, api.HealthCritical, healthStatus(r, server, "api", "_service_maintenance:api-1"))
		})

		require.NoError(t, client.Agent().DisableServiceMaintenance("api-1"))
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Equal(r, 1, passingCount(r))
			require.Empty(r, healthStatus(r, server, "api", "_service_maintenance:api-1"))
		})
	})

	t.Run("node maintenance", func(t *testing.T) {
		require.NoError(t, client.Agent().EnableNodeMaintenance("host patching"))
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Equal(r, 0, passingCount(r))
			nodeChecks, _, err := server.Health().Node(nodeName, nil)
			require.NoError(r, err)
			var found bool
			for _, chk := range nodeChecks {
				if chk.CheckID == "_node_maintenance" {
					found = true
					require.Equal(r, api.HealthCritical, chk.Status)
					require.Equal(r, "host patching", chk.Notes)
				}
			}
			require.True(r, found, "node maintenance check missing")
		})

		require.NoError(t, client.Agent().DisableNodeMaintenance())
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Equal(r, 1, passingCount(r))
		})
	})
}

// TestCatalog_AntiEntropy verifies that the client agent is the source of
// truth for its services: an out-of-band removal from the catalog is repaired
// by anti-entropy, and an agent-side deregistration is propagated.
func TestCatalog_AntiEntropy(t *testing.T) {
	t.Parallel()

	_, server, client := newCluster(t)
	nodeName, err := client.Agent().NodeName()
	require.NoError(t, err)

	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID: "cache-1", Name: "cache", Port: 6379, Tags: []string{"v1"},
	}))
	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		require.Len(r, serviceInstances(r, server, "cache", nil), 1)
	})

	// Remove the instance behind the agent's back.
	_, err = server.Catalog().Deregister(&api.CatalogDeregistration{Node: nodeName, ServiceID: "cache-1"}, nil)
	require.NoError(t, err)
	require.Empty(t, serviceInstances(t, server, "cache", nil))

	// The next anti-entropy full sync restores it with the original attributes.
	retry.RunWith(aeTimer, t, func(r *retry.R) {
		svcs := serviceInstances(r, server, "cache", nil)
		require.Len(r, svcs, 1)
		require.Equal(r, 6379, svcs[0].ServicePort)
		require.Equal(r, []string{"v1"}, svcs[0].ServiceTags)
	})

	// Changing the registration on the agent is propagated.
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID: "cache-1", Name: "cache", Port: 6380, Tags: []string{"v2"},
	}))
	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		svcs := serviceInstances(r, server, "cache", nil)
		require.Len(r, svcs, 1)
		require.Equal(r, 6380, svcs[0].ServicePort)
		require.Equal(r, []string{"v2"}, svcs[0].ServiceTags)
	})
}
