package haproxy2consul

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
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

func registerServiceWithSidecar(t *testing.T, client *api.Client, serviceID string, ports []int) {
	t.Helper()
	upstreams := make([]api.Upstream, 1)
	upstreams[0] = api.Upstream{
		Datacenter:       "dc1",
		DestinationName:  "consul-agent-http",
		LocalBindAddress: "127.0.1.1",
		LocalBindPort:    ports[1],
	}
	reg := &api.AgentServiceRegistration{
		Name: "consul-agent-http",
		ID:   serviceID,
		Tags: []string{"bar", "baz"},
		// Service is the HTTP API of Consul
		Port: ports[0],
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Port: ports[1],
				Proxy: &api.AgentServiceConnectProxyConfig{
					Config: map[string]interface{}{
						"handshake_timeout_ms": 999,
					},

					Upstreams: upstreams,
				},
			},
		},
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://127.0.0.1:%d/", ports[0]),
			Interval: "5s",
			Status:   api.HealthPassing,
		},
	}
	if err := client.Agent().ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func startHTTPServerServer(wg *sync.WaitGroup, port int) *http.Server {
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port)}
	wg.Add(1)

	go func() {
		defer wg.Done() // let main know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// returning reference so caller can call Shutdown()
	return srv
}

func tcpAddr(ip string, port int) *net.TCPAddr {
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
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
	t.Parallel()
	client, srvVerify := makeClientWithConfig(t, func(conf *testutil.TestServerConfig) {
	})
	defer srvVerify.Stop()

	// We reserve 1 port for the sidecar proxy
	ports, err := freeport.Take(4)
	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer wg.Wait()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello world\n")
	})
	listener0 := startHTTPServerServer(wg, ports[0])
	defer listener0.Shutdown(ctx)

	listener1, err := net.ListenTCP("tcp", tcpAddr("127.0.0.1", ports[1]))
	assert.NoError(t, err)
	defer listener1.Close()

	listener2 := startHTTPServerServer(wg, ports[2])
	defer listener2.Shutdown(ctx)

	listener3, err := net.ListenTCP("tcp", tcpAddr("127.0.0.1", ports[3]))
	assert.NoError(t, err)
	defer listener3.Close()

	registerServiceWithSidecar(t, client, "consul-agent-http-001", ports)

	log := NewTestingLogger(t)
	// consul has no connect configuration, should fail
	watcher := New("consul", client, log)
	err = watcher.Run()
	if err == nil {
		t.Fatal("No Sidecar should be registered, should create an error")
	}
	assert.Equal(t, "No sidecar proxy registered for consul", err.Error())

	// consul-agent-http has config, watch should work and receive first config
	watcher = New("consul-agent-http-001", client, log)
	go func() {
		err := watcher.Run()
		assert.NoError(t, err)
		assert.Fail(t, "ooops")
	}()
	ticker := time.NewTicker(time.Second * 20)
	select {
	case config := <-watcher.C:
		assert.Equal(t, "consul-agent-http", config.ServiceName)
		assert.Equal(t, "0.0.0.0", config.Downstream.LocalBindAddress)
		assert.Equal(t, "127.0.0.1", config.Downstream.TargetAddress)
		assert.NotEmpty(t, config.Downstream.TLS)
	case <-ticker.C:
		t.Fatal("Should have been called")
	}
	done := make(chan bool)
	go func() {
		intID, meta, err := client.Connect().IntentionCreate(&api.Intention{Description: "updated", SourceName: "*", DestinationName: "consul-agent-http", Action: api.IntentionActionAllow}, &api.WriteOptions{})
		assert.NoError(t, err)
		assert.NotEmpty(t, intID)
		assert.NotNil(t, meta)
		done <- true
	}()
	select {
	case <-done:
		fmt.Println("okay")
	case <-ticker.C:
		t.Fatal("timeout")
	}
	retryForChecksToBeUp := func() *retry.Timer {
		return &retry.Timer{Timeout: time.Duration(21 * time.Second), Wait: time.Duration(1 * time.Second)}
	}
	retry.RunWith(retryForChecksToBeUp(), t, func(r *retry.R) {
		select {
		case config := <-watcher.C:
			assert.Equal(r, "consul-agent-http", config.ServiceName)
			assert.Equal(r, "0.0.0.0", config.Downstream.LocalBindAddress)
			assert.Equal(r, "127.0.0.1", config.Downstream.TargetAddress)
			assert.NotEmpty(r, config.Downstream.TLS)
			assert.Equal(r, 1, len(config.Upstreams))
		case <-ticker.C:
			r.Fatal("Should have been called")
		}
	})
	retry.RunWith(retryForChecksToBeUp(), t, func(r *retry.R) {
		svc, _, err := client.Health().Service("consul-agent-http", "", false, &api.QueryOptions{})
		assert.NoError(r, err)
		assert.Equal(r, 1, len(svc))
	})
	retry.RunWith(retryForChecksToBeUp(), t, func(r *retry.R) {
		svc, _, err := client.Health().Connect("consul-agent-http", "", true, &api.QueryOptions{})
		assert.NoError(r, err)
		assert.Equal(r, 1, len(svc))
	})

	registerServiceWithSidecar(t, client, "consul-agent-http-002", ports[2:])
	retry.RunWith(retryForChecksToBeUp(), t, func(r *retry.R) {
		svc, _, err := client.Health().Service("consul-agent-http", "", true, &api.QueryOptions{})
		assert.NoError(r, err)
		assert.Equal(r, 2, len(svc))
	})
	retry.RunWith(retryForChecksToBeUp(), t, func(r *retry.R) {
		svc, _, err := client.Health().Connect("consul-agent-http", "", true, &api.QueryOptions{})
		assert.NoError(r, err)
		assert.Equal(r, 2, len(svc))
	})
	// Wait for 2nd instance to be visible
	retry.RunWith(retryForChecksToBeUp(), t, func(r *retry.R) {
		select {
		case config := <-watcher.C:
			fmt.Printf("Upstreams are: %#v\n", config.Upstreams)
			assert.Equal(r, 2, len(config.Upstreams[0].Nodes))
		case <-ticker.C:
			r.Fatal("Should have been called")
		}
	})
}
