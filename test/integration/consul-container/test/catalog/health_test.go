// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
)

// TestHealthChecks verifies that each check type drives the service status
// served by the servers, and that the status feeds passing-only health queries
// and the health state endpoints.
func TestHealthChecks(t *testing.T) {
	t.Parallel()

	cluster, server, client := newCluster(t)

	passingInstances := func(r retry.TestingTB, service string) int {
		entries, _, err := server.Health().Service(service, "", true, nil)
		require.NoError(r, err)
		return len(entries)
	}
	waitStatus := func(t *testing.T, service, serviceID, want string) {
		t.Helper()
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Equal(r, want, healthStatus(r, server, service, "service:"+serviceID))
		})
	}

	t.Run("ttl check transitions", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID:   "ttl-1",
			Name: "ttl-svc",
			Port: 9001,
			Check: &api.AgentServiceCheck{
				TTL:    "10m",
				Status: api.HealthCritical,
			},
		}))
		waitStatus(t, "ttl-svc", "ttl-1", api.HealthCritical)
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Equal(r, 0, passingInstances(r, "ttl-svc"))
		})

		for _, step := range []struct {
			status string
			output string
		}{
			{api.HealthPassing, "all good"},
			{api.HealthWarning, "degraded"},
			{api.HealthCritical, "broken"},
			{api.HealthPassing, "recovered"},
		} {
			require.NoError(t, client.Agent().UpdateTTL("service:ttl-1", step.output, step.status))

			retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
				checks, _, err := server.Health().Checks("ttl-svc", nil)
				require.NoError(r, err)
				require.Len(r, checks, 1)
				require.Equal(r, step.status, checks[0].Status)
				require.Equal(r, step.output, checks[0].Output)

				// Warning still counts as "not passing" for passing-only queries.
				want := 0
				if step.status == api.HealthPassing {
					want = 1
				}
				require.Equal(r, want, passingInstances(r, "ttl-svc"))

				inState, _, err := server.Health().State(step.status, nil)
				require.NoError(r, err)
				var found bool
				for _, chk := range inState {
					if chk.CheckID == "service:ttl-1" {
						found = true
					}
				}
				require.True(r, found, "check missing from /v1/health/state/%s", step.status)
			})
		}
	})

	t.Run("ttl check expires to critical", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID:    "ttl-exp-1",
			Name:  "ttl-expiry",
			Port:  9002,
			Check: &api.AgentServiceCheck{TTL: "5s"},
		}))
		require.NoError(t, client.Agent().UpdateTTL("service:ttl-exp-1", "alive", api.HealthPassing))
		waitStatus(t, "ttl-expiry", "ttl-exp-1", api.HealthPassing)

		// No further updates: the TTL lapses and the check turns critical.
		waitStatus(t, "ttl-expiry", "ttl-exp-1", api.HealthCritical)
	})

	t.Run("http and tcp checks", func(t *testing.T) {
		t.Parallel()

		// The agent's own HTTP API is a convenient always-listening target; port 1 is never open.
		const (
			goodHTTP = "http://127.0.0.1:8500/v1/status/leader"
			badHTTP  = "http://127.0.0.1:1/"
			goodTCP  = "127.0.0.1:8500"
			badTCP   = "127.0.0.1:1"
		)
		for _, tc := range []struct {
			id    string
			check api.AgentServiceCheck
		}{
			{"http-ok", api.AgentServiceCheck{HTTP: goodHTTP}},
			{"http-bad", api.AgentServiceCheck{HTTP: badHTTP}},
			{"tcp-ok", api.AgentServiceCheck{TCP: goodTCP}},
			{"tcp-bad", api.AgentServiceCheck{TCP: badTCP}},
		} {
			check := tc.check
			check.Interval = "2s"
			check.Timeout = "1s"
			require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
				ID:    tc.id,
				Name:  tc.id,
				Port:  9003,
				Check: &check,
			}))
		}

		for _, tc := range []struct{ id, want string }{
			{"http-ok", api.HealthPassing},
			{"http-bad", api.HealthCritical},
			{"tcp-ok", api.HealthPassing},
			{"tcp-bad", api.HealthCritical},
		} {
			waitStatus(t, tc.id, tc.id, tc.want)
			retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
				want := 0
				if tc.want == api.HealthPassing {
					want = 1
				}
				require.Equal(r, want, passingInstances(r, tc.id))
			})
		}
	})

	t.Run("http check recovers when target comes back", func(t *testing.T) {
		t.Parallel()

		// A TTL-controlled flip is deterministic; here the check target is
		// swapped by re-registering the service with a healthy check.
		require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID:   "flap-1",
			Name: "flap",
			Port: 9004,
			Check: &api.AgentServiceCheck{
				HTTP: "http://127.0.0.1:1/", Interval: "2s", Timeout: "1s",
			},
		}))
		waitStatus(t, "flap", "flap-1", api.HealthCritical)

		require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID:   "flap-1",
			Name: "flap",
			Port: 9004,
			Check: &api.AgentServiceCheck{
				HTTP: "http://127.0.0.1:8500/v1/status/leader", Interval: "2s", Timeout: "1s",
			},
		}))
		waitStatus(t, "flap", "flap-1", api.HealthPassing)
	})

	t.Run("alias check follows its target", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID:    "backend-1",
			Name:  "backend",
			Port:  9005,
			Check: &api.AgentServiceCheck{TTL: "10m", Status: api.HealthPassing},
		}))
		require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID:    "frontend-1",
			Name:  "frontend",
			Port:  9006,
			Check: &api.AgentServiceCheck{AliasService: "backend-1"},
		}))
		waitStatus(t, "frontend", "frontend-1", api.HealthPassing)

		require.NoError(t, client.Agent().UpdateTTL("service:backend-1", "down", api.HealthCritical))
		waitStatus(t, "frontend", "frontend-1", api.HealthCritical)

		require.NoError(t, client.Agent().UpdateTTL("service:backend-1", "up", api.HealthPassing))
		waitStatus(t, "frontend", "frontend-1", api.HealthPassing)
	})

	t.Run("grpc check", func(t *testing.T) {
		t.Parallel()

		// fortio serves the standard gRPC health protocol for the "ping"
		// service (SERVING) and "ping_down" (NOT_SERVING).
		app, err := libservice.NewExampleService(context.Background(), "grpc-health", 8080, 8079, cluster.Agents[1])
		require.NoError(t, err)
		t.Cleanup(func() { _ = app.Terminate() })

		for _, tc := range []struct{ id, target, want string }{
			{"grpc-ok", "127.0.0.1:8079/ping", api.HealthPassing},
			{"grpc-down", "127.0.0.1:8079/ping_down", api.HealthCritical},
		} {
			require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
				ID:    tc.id,
				Name:  tc.id,
				Port:  8079,
				Check: &api.AgentServiceCheck{GRPC: tc.target, Interval: "2s", Timeout: "1s"},
			}))
			waitStatus(t, tc.id, tc.id, tc.want)
		}
	})

	t.Run("standalone node check", func(t *testing.T) {
		t.Parallel()

		nodeName, err := client.Agent().NodeName()
		require.NoError(t, err)

		require.NoError(t, client.Agent().CheckRegister(&api.AgentCheckRegistration{
			ID:   "node-disk",
			Name: "disk space",
			AgentServiceCheck: api.AgentServiceCheck{
				TTL: "10m",
			},
		}))
		require.NoError(t, client.Agent().UpdateTTL("node-disk", "low", api.HealthWarning))

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			nodeChecks, _, err := server.Health().Node(nodeName, nil)
			require.NoError(r, err)
			var found bool
			for _, chk := range nodeChecks {
				if chk.CheckID == "node-disk" {
					found = true
					require.Equal(r, api.HealthWarning, chk.Status)
					require.Equal(r, "low", chk.Output)
					require.Empty(r, chk.ServiceID)
				}
			}
			require.True(r, found)
		})

		require.NoError(t, client.Agent().CheckDeregister("node-disk"))
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			nodeChecks, _, err := server.Health().Node(nodeName, nil)
			require.NoError(r, err)
			for _, chk := range nodeChecks {
				require.NotEqual(r, "node-disk", chk.CheckID)
			}
		})
	})

}
