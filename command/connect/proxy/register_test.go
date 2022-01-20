package proxy

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestRegisterMonitor_good(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	m, service := testMonitor(t, client)
	defer m.Close()

	// Verify the settings
	require.Equal(t, api.ServiceKindConnectProxy, service.Kind)
	require.Equal(t, "foo", service.Proxy.DestinationServiceName)
	require.Equal(t, "127.0.0.1", service.Address)
	require.Equal(t, 1234, service.Port)

	// Stop should deregister the service
	require.NoError(t, m.Close())
	services, err := client.Agent().Services()
	require.NoError(t, err)
	require.NotContains(t, services, m.serviceID())
}

func TestRegisterMonitor_heartbeat(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	m, _ := testMonitor(t, client)
	defer m.Close()
	retry.Run(t, func(r *retry.R) {
		// Get the check and verify that it is passing
		checks, err := client.Agent().Checks()
		require.NoError(r, err)
		require.Contains(r, checks, m.checkID())
		require.Equal(r, "passing", checks[m.checkID()].Status)
		// Purposely fail the TTL check, verify it becomes healthy again
		require.NoError(r, client.Agent().FailTTL(m.checkID(), ""))
	})

	retry.Run(t, func(r *retry.R) {

		checks, err := client.Agent().Checks()
		if err != nil {
			r.Fatalf("err: %s", err)
		}

		check, ok := checks[m.checkID()]
		if !ok {
			r.Fatal("check not found")
		}

		if check.Status != "passing" {
			r.Fatalf("check status is bad: %s", check.Status)
		}
	})
}

// testMonitor creates a RegisterMonitor, configures it, and starts it.
// It waits until the service appears in the catalog and then returns.
func testMonitor(t *testing.T, client *api.Client) (*RegisterMonitor, *api.AgentService) {
	// Setup the monitor
	m := NewRegisterMonitor(testutil.Logger(t))
	m.Client = client
	m.Service = "foo"
	m.LocalAddress = "127.0.0.1"
	m.LocalPort = 1234

	// We want shorter periods so we can test things
	m.ReconcilePeriod = 400 * time.Millisecond
	m.TTLPeriod = 200 * time.Millisecond

	// Start the monitor
	go m.Run()

	// The service should be registered
	var service *api.AgentService
	retry.Run(t, func(r *retry.R) {
		services, err := client.Agent().Services()
		if err != nil {
			r.Fatalf("err: %s", err)
		}

		var ok bool
		service, ok = services[m.serviceID()]
		if !ok {
			r.Fatal("service not found")
		}
	})

	return m, service
}
