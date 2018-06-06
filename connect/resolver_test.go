package connect

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestStaticResolver_Resolve(t *testing.T) {
	type fields struct {
		Addr    string
		CertURI connect.CertURI
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name:   "simples",
			fields: fields{"1.2.3.4:80", connect.TestSpiffeIDService(t, "foo")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := StaticResolver{
				Addr:    tt.fields.Addr,
				CertURI: tt.fields.CertURI,
			}
			addr, certURI, err := sr.Resolve(context.Background())
			require := require.New(t)
			require.Nil(err)
			require.Equal(sr.Addr, addr)
			require.Equal(sr.CertURI, certURI)
		})
	}
}

func TestConsulResolver_Resolve(t *testing.T) {
	// Setup a local test agent to query
	agent := agent.NewTestAgent("test-consul", "")
	defer agent.Shutdown()

	cfg := api.DefaultConfig()
	cfg.Address = agent.HTTPAddr()
	client, err := api.NewClient(cfg)
	require.Nil(t, err)

	// Setup a service with a connect proxy instance
	regSrv := &api.AgentServiceRegistration{
		Name: "web",
		Port: 8080,
	}
	err = client.Agent().ServiceRegister(regSrv)
	require.Nil(t, err)

	regProxy := &api.AgentServiceRegistration{
		Kind:             "connect-proxy",
		Name:             "web-proxy",
		Port:             9090,
		ProxyDestination: "web",
	}
	err = client.Agent().ServiceRegister(regProxy)
	require.Nil(t, err)

	// And another proxy so we can test handling with multiple endpoints returned
	regProxy.Port = 9091
	regProxy.ID = "web-proxy-2"
	err = client.Agent().ServiceRegister(regProxy)
	require.Nil(t, err)

	// Add a native service
	{
		regSrv := &api.AgentServiceRegistration{
			Name: "db",
			Port: 8080,
			Connect: &api.AgentServiceConnect{
				Native: true,
			},
		}
		require.NoError(t, client.Agent().ServiceRegister(regSrv))
	}

	// Add a prepared query
	queryId, _, err := client.PreparedQuery().Create(&api.PreparedQueryDefinition{
		Name: "test-query",
		Service: api.ServiceQuery{
			Service: "web",
			Connect: true,
		},
	}, nil)
	require.NoError(t, err)

	proxyAddrs := []string{
		agent.Config.AdvertiseAddrLAN.String() + ":9090",
		agent.Config.AdvertiseAddrLAN.String() + ":9091",
	}

	type fields struct {
		Namespace  string
		Name       string
		Type       int
		Datacenter string
	}
	tests := []struct {
		name        string
		fields      fields
		timeout     time.Duration
		wantAddr    string
		wantCertURI connect.CertURI
		wantErr     bool
		addrs       []string
	}{
		{
			name: "basic service discovery",
			fields: fields{
				Namespace: "default",
				Name:      "web",
				Type:      ConsulResolverTypeService,
			},
			wantCertURI: connect.TestSpiffeIDService(t, "web"),
			wantErr:     false,
			addrs:       proxyAddrs,
		},
		{
			name: "basic service with native service",
			fields: fields{
				Namespace: "default",
				Name:      "db",
				Type:      ConsulResolverTypeService,
			},
			wantCertURI: connect.TestSpiffeIDService(t, "db"),
			wantErr:     false,
		},
		{
			name: "Bad Type errors",
			fields: fields{
				Namespace: "default",
				Name:      "web",
				Type:      123,
			},
			wantErr: true,
		},
		{
			name: "Non-existent service errors",
			fields: fields{
				Namespace: "default",
				Name:      "foo",
				Type:      ConsulResolverTypeService,
			},
			wantErr: true,
		},
		{
			name: "timeout errors",
			fields: fields{
				Namespace: "default",
				Name:      "web",
				Type:      ConsulResolverTypeService,
			},
			timeout: 1 * time.Nanosecond,
			wantErr: true,
		},
		{
			name: "prepared query by id",
			fields: fields{
				Name: queryId,
				Type: ConsulResolverTypePreparedQuery,
			},
			wantCertURI: connect.TestSpiffeIDService(t, "web"),
			wantErr:     false,
			addrs:       proxyAddrs,
		},
		{
			name: "prepared query by name",
			fields: fields{
				Name: "test-query",
				Type: ConsulResolverTypePreparedQuery,
			},
			wantCertURI: connect.TestSpiffeIDService(t, "web"),
			wantErr:     false,
			addrs:       proxyAddrs,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			cr := &ConsulResolver{
				Client:     client,
				Namespace:  tt.fields.Namespace,
				Name:       tt.fields.Name,
				Type:       tt.fields.Type,
				Datacenter: tt.fields.Datacenter,
			}
			// WithCancel just to have a cancel func in scope to assign in the if
			// clause.
			ctx, cancel := context.WithCancel(context.Background())
			if tt.timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, tt.timeout)
			}
			defer cancel()
			gotAddr, gotCertURI, err := cr.Resolve(ctx)
			if tt.wantErr {
				require.NotNil(err)
				return
			}

			require.Nil(err)
			require.Equal(tt.wantCertURI, gotCertURI)
			if len(tt.addrs) > 0 {
				require.Contains(tt.addrs, gotAddr)
			}
		})
	}
}
