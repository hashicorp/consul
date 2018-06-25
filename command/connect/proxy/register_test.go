package proxy

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestRegisterMonitor_good(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	m, service := testMonitor(t, client)
	defer m.Close()

	// Verify the settings
	require.Equal(api.ServiceKindConnectProxy, service.Kind)
	require.Equal("foo", service.ProxyDestination)
	require.Equal("127.0.0.1", service.Address)
	require.Equal(1234, service.Port)

	// Stop should deregister the service
	require.NoError(m.Close())
	services, err := client.Agent().Services()
	require.NoError(err)
	require.NotContains(services, m.serviceID())
}

func TestRegisterMonitor_heartbeat(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	m, _ := testMonitor(t, client)
	defer m.Close()

	// Get the check and verify that it is passing
	checks, err := client.Agent().Checks()
	require.NoError(err)
	require.Contains(checks, m.checkID())
	require.Equal("passing", checks[m.checkID()].Status)

	// Purposely fail the TTL check, verify it becomes healthy again
	require.NoError(client.Agent().FailTTL(m.checkID(), ""))
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
	m := NewRegisterMonitor()
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
