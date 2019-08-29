package proxy

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/stretchr/testify/require"
)

func TestUpstreamResolverFuncFromClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  UpstreamConfig
		want *connect.ConsulResolver
	}{
		{
			name: "service",
			cfg: UpstreamConfig{
				DestinationNamespace: "foo",
				DestinationName:      "web",
				Datacenter:           "ny1",
				DestinationType:      "service",
			},
			want: &connect.ConsulResolver{
				Namespace:  "foo",
				Name:       "web",
				Datacenter: "ny1",
				Type:       connect.ConsulResolverTypeService,
			},
		},
		{
			name: "prepared_query",
			cfg: UpstreamConfig{
				DestinationNamespace: "foo",
				DestinationName:      "web",
				Datacenter:           "ny1",
				DestinationType:      "prepared_query",
			},
			want: &connect.ConsulResolver{
				Namespace:  "foo",
				Name:       "web",
				Datacenter: "ny1",
				Type:       connect.ConsulResolverTypePreparedQuery,
			},
		},
		{
			name: "unknown behaves like service",
			cfg: UpstreamConfig{
				DestinationNamespace: "foo",
				DestinationName:      "web",
				Datacenter:           "ny1",
				DestinationType:      "junk",
			},
			want: &connect.ConsulResolver{
				Namespace:  "foo",
				Name:       "web",
				Datacenter: "ny1",
				Type:       connect.ConsulResolverTypeService,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Client doesn't really matter as long as it's passed through.
			gotFn := UpstreamResolverFuncFromClient(nil)
			got, err := gotFn(tt.cfg)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAgentConfigWatcherSidecarProxy(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, "agent_smith", ``)
	defer a.Shutdown()

	client := a.Client()
	agent := client.Agent()

	// Register a local agent service with a sidecar proxy
	reg := &api.AgentServiceRegistration{
		Name: "web",
		Port: 8080,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Config: map[string]interface{}{
						"handshake_timeout_ms": 999,
					},
					Upstreams: []api.Upstream{
						{
							DestinationName: "db",
							LocalBindPort:   9191,
						},
					},
				},
			},
		},
	}
	err := agent.ServiceRegister(reg)
	require.NoError(t, err)

	w, err := NewAgentConfigWatcher(client, "web-sidecar-proxy",
		log.New(os.Stderr, "", log.LstdFlags))
	require.NoError(t, err)

	cfg := testGetConfigValTimeout(t, w, 500*time.Millisecond)

	expectCfg := &Config{
		ProxiedServiceName:      "web",
		ProxiedServiceNamespace: "default",
		PublicListener: PublicListenerConfig{
			BindAddress:           "0.0.0.0",
			BindPort:              21000,
			LocalServiceAddress:   "127.0.0.1:8080",
			HandshakeTimeoutMs:    999,
			LocalConnectTimeoutMs: 1000, // from applyDefaults
		},
		Upstreams: []UpstreamConfig{
			{
				DestinationName:      "db",
				DestinationNamespace: "default",
				DestinationType:      "service",
				LocalBindPort:        9191,
				LocalBindAddress:     "127.0.0.1",
			},
		},
	}

	require.Equal(t, expectCfg, cfg)

	// Now keep watching and update the config.
	go func() {
		// Wait for watcher to be watching
		time.Sleep(20 * time.Millisecond)
		reg.Connect.SidecarService.Proxy.Upstreams = append(reg.Connect.SidecarService.Proxy.Upstreams,
			api.Upstream{
				DestinationName:  "cache",
				LocalBindPort:    9292,
				LocalBindAddress: "127.10.10.10",
			})
		reg.Connect.SidecarService.Proxy.Config["local_connect_timeout_ms"] = 444
		err := agent.ServiceRegister(reg)
		require.NoError(t, err)
	}()

	cfg = testGetConfigValTimeout(t, w, 2*time.Second)

	expectCfg.Upstreams = append(expectCfg.Upstreams, UpstreamConfig{
		DestinationName:      "cache",
		DestinationNamespace: "default",
		DestinationType:      "service",
		LocalBindPort:        9292,
		LocalBindAddress:     "127.10.10.10",
	})
	expectCfg.PublicListener.LocalConnectTimeoutMs = 444

	assert.Equal(t, expectCfg, cfg)
}

func testGetConfigValTimeout(t *testing.T, w ConfigWatcher,
	timeout time.Duration) *Config {
	t.Helper()
	select {
	case cfg := <-w.Watch():
		return cfg
	case <-time.After(timeout):
		t.Fatalf("timeout after %s waiting for config update", timeout)
		return nil
	}
}
