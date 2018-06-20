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

func TestParseConfigFile(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfigFile("testdata/config-kitchensink.hcl")
	require.Nil(t, err)

	expect := &Config{
		Token:                   "11111111-2222-3333-4444-555555555555",
		ProxiedServiceName:      "web",
		ProxiedServiceNamespace: "default",
		PublicListener: PublicListenerConfig{
			BindAddress:           "127.0.0.1",
			BindPort:              9999,
			LocalServiceAddress:   "127.0.0.1:5000",
			LocalConnectTimeoutMs: 1000,
			HandshakeTimeoutMs:    10000, // From defaults
		},
		Upstreams: []UpstreamConfig{
			{
				LocalBindAddress:     "127.0.0.1:6000",
				DestinationName:      "db",
				DestinationNamespace: "default",
				DestinationType:      "service",
				ConnectTimeoutMs:     10000,
			},
			{
				LocalBindAddress:     "127.0.0.1:6001",
				DestinationName:      "geo-cache",
				DestinationNamespace: "default",
				DestinationType:      "prepared_query",
				ConnectTimeoutMs:     10000,
			},
		},
	}

	require.Equal(t, expect, cfg)
}

func TestUpstreamResolverFromClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  UpstreamConfig
		want *connect.ConsulResolver
	}{
		{
			name: "service",
			cfg: UpstreamConfig{
				DestinationNamespace:  "foo",
				DestinationName:       "web",
				DestinationDatacenter: "ny1",
				DestinationType:       "service",
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
				DestinationNamespace:  "foo",
				DestinationName:       "web",
				DestinationDatacenter: "ny1",
				DestinationType:       "prepared_query",
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
				DestinationNamespace:  "foo",
				DestinationName:       "web",
				DestinationDatacenter: "ny1",
				DestinationType:       "junk",
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
			got := UpstreamResolverFromClient(nil, tt.cfg)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAgentConfigWatcher(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent("agent_smith", `
	connect {
		enabled = true
		proxy {
			allow_managed_api_registration = true
		}
	}
	`)
	defer a.Shutdown()

	client := a.Client()
	agent := client.Agent()

	// Register a service with a proxy
	// Register a local agent service with a managed proxy
	reg := &api.AgentServiceRegistration{
		Name: "web",
		Port: 8080,
		Connect: &api.AgentServiceConnect{
			Proxy: &api.AgentServiceConnectProxy{
				Config: map[string]interface{}{
					"bind_address":          "10.10.10.10",
					"bind_port":             1010,
					"local_service_address": "127.0.0.1:5000",
					"handshake_timeout_ms":  999,
					"upstreams": []interface{}{
						map[string]interface{}{
							"destination_name": "db",
							"local_bind_port":  9191,
						},
					},
				},
			},
		},
	}
	err := agent.ServiceRegister(reg)
	require.NoError(t, err)

	w, err := NewAgentConfigWatcher(client, "web-proxy",
		log.New(os.Stderr, "", log.LstdFlags))
	require.NoError(t, err)

	cfg := testGetConfigValTimeout(t, w, 500*time.Millisecond)

	expectCfg := &Config{
		ProxiedServiceName:      "web",
		ProxiedServiceNamespace: "default",
		PublicListener: PublicListenerConfig{
			BindAddress:           "10.10.10.10",
			BindPort:              1010,
			LocalServiceAddress:   "127.0.0.1:5000",
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
				ConnectTimeoutMs:     10000, // from applyDefaults
			},
		},
	}

	assert.Equal(t, expectCfg, cfg)

	// Now keep watching and update the config.
	go func() {
		// Wait for watcher to be watching
		time.Sleep(20 * time.Millisecond)
		upstreams := reg.Connect.Proxy.Config["upstreams"].([]interface{})
		upstreams = append(upstreams, map[string]interface{}{
			"destination_name":   "cache",
			"local_bind_port":    9292,
			"local_bind_address": "127.10.10.10",
		})
		reg.Connect.Proxy.Config["upstreams"] = upstreams
		reg.Connect.Proxy.Config["local_connect_timeout_ms"] = 444
		err := agent.ServiceRegister(reg)
		require.NoError(t, err)
	}()

	cfg = testGetConfigValTimeout(t, w, 2*time.Second)

	expectCfg.Upstreams = append(expectCfg.Upstreams, UpstreamConfig{
		DestinationName:      "cache",
		DestinationNamespace: "default",
		DestinationType:      "service",
		ConnectTimeoutMs:     10000, // from applyDefaults
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
