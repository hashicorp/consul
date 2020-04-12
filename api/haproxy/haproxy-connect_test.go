package haproxy

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/assert"
)

func makeClientWithConfig(
	t *testing.T,
	cb2 testutil.ServerConfigCallback) (*api.Client, *testutil.TestServer) {

	// Make client config
	conf := api.DefaultConfig()

	// Create server
	var server *testutil.TestServer
	var err error
	retry.RunWith(retry.ThreeTimes(), t, func(r *retry.R) {
		server, err = testutil.NewTestServerConfigT(t, cb2)
		if err != nil {
			r.Fatal(err)
		}
	})
	if server.Config.Bootstrap {
		server.WaitForLeader(t)
	}

	conf.Address = server.HTTPAddr

	// Create client
	client, err := api.NewClient(conf)
	if err != nil {
		server.Stop()
		t.Fatalf("err: %v", err)
	}

	return client, server
}

// Here we test the order of the calls used in HAProxy Connect
// This test ensure the datastructures are properly filled
// With the the required fields.
// This is testing:
// * `/v1/agent/service/<service>`
// * `/v1/health/connect/<service>`
// * `/v1/agent/connect/ca/leaf/<serviceid>`
// * `/v1/agent/connect/ca/leaf/`
func Test_HAProxyConnect_TestEndToEnd(t *testing.T) {
	client, srvVerify := makeClientWithConfig(t, func(conf *testutil.TestServerConfig) {
	})
	defer srvVerify.Stop()

	reg := &api.AgentServiceRegistration{
		Name:  "consul-agent-http",
		ID:    "consul-agent-http",
		Tags:  []string{"bar", "baz"},
		Port:  8500,
		Proxy: &api.AgentServiceConnectProxyConfig{},
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{},
		},
		Check: &api.AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := client.Agent().ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	// consul has no connect configuration, shouuld fail
	watcher := NewWatcher("consul", client)
	err := watcher.Run()
	if err == nil {
		t.Fatal("No Sidecar should be registered, should create an error")
	}
	assert.Equal(t, "No sidecar proxy registered for consul", err.Error())
	// consul-agent-http has config, watch should workd
	watcher = NewWatcher("consul-agent-http", client)
	go func() {
		err := watcher.Run()
		assert.NoError(t, err)
	}()
	done := make(chan bool)
	select {
	case config := <-watcher.C:
		fmt.Println("received message", config)
		assert.Equal(t, "consul-agent-http", config.ServiceName)
	case <-done:
		t.Fatal("Should have been called")
	}
}
