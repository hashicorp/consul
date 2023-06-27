// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connect

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
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
			require.Nil(t, err)
			require.Equal(t, sr.Addr, addr)
			require.Equal(t, sr.CertURI, certURI)
		})
	}
}

func TestConsulResolver_Resolve(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Setup a local test agent to query
	agent := agent.StartTestAgent(t, agent.TestAgent{Name: "test-consul"})
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
		Kind: "connect-proxy",
		Name: "web-proxy",
		Port: 9090,
		Proxy: &api.AgentServiceConnectProxyConfig{
			DestinationServiceName: "web",
		},
		Meta: map[string]string{
			"MetaKey": "MetaValue",
		},
	}
	err = client.Agent().ServiceRegister(regProxy)
	require.Nil(t, err)

	// And another proxy so we can test handling with multiple endpoints returned
	regProxy.Port = 9091
	regProxy.ID = "web-proxy-2"
	regProxy.Meta = map[string]string{}
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
		Filter     string
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
			// Want empty host since we don't enforce trust domain outside of TLS and
			// don't need to load the current one this way.
			wantCertURI: connect.TestSpiffeIDServiceWithHost(t, "web", ""),
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
			// Want empty host since we don't enforce trust domain outside of TLS and
			// don't need to load the current one this way.
			wantCertURI: connect.TestSpiffeIDServiceWithHost(t, "db", ""),
			wantErr:     false,
		},
		{
			name: "service discovery with filter",
			fields: fields{
				Namespace: "default",
				Name:      "web",
				Type:      ConsulResolverTypeService,
				Filter:    "Service.Meta[`MetaKey`] == `MetaValue`",
			},
			// Want empty host since we don't enforce trust domain outside of TLS and
			// don't need to load the current one this way.
			wantCertURI: connect.TestSpiffeIDServiceWithHost(t, "web", ""),
			wantErr:     false,
			addrs: []string{
				agent.Config.AdvertiseAddrLAN.String() + ":9090",
			},
		},
		{
			name: "service discovery with filter",
			fields: fields{
				Namespace: "default",
				Name:      "web",
				Type:      ConsulResolverTypeService,
				Filter:    "`AnotherMetaValue` in Service.Meta.MetaKey",
			},
			wantErr: true,
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
			// Want empty host since we don't enforce trust domain outside of TLS and
			// don't need to load the current one this way.
			wantCertURI: connect.TestSpiffeIDServiceWithHost(t, "web", ""),
			wantErr:     false,
			addrs:       proxyAddrs,
		},
		{
			name: "prepared query by name",
			fields: fields{
				Name: "test-query",
				Type: ConsulResolverTypePreparedQuery,
			},
			// Want empty host since we don't enforce trust domain outside of TLS and
			// don't need to load the current one this way.
			wantCertURI: connect.TestSpiffeIDServiceWithHost(t, "web", ""),
			wantErr:     false,
			addrs:       proxyAddrs,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &ConsulResolver{
				Client:     client,
				Namespace:  tt.fields.Namespace,
				Name:       tt.fields.Name,
				Type:       tt.fields.Type,
				Datacenter: tt.fields.Datacenter,
				Filter:     tt.fields.Filter,
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
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantCertURI, gotCertURI)
			if len(tt.addrs) > 0 {
				require.Contains(t, tt.addrs, gotAddr)
			}
		})
	}
}

func TestConsulResolverFromAddrFunc(t *testing.T) {
	// Don't need an actual instance since we don't do the service discovery but
	// we do want to assert the client is pass through correctly.
	client, err := api.NewClient(api.DefaultConfig())
	require.NoError(t, err)

	tests := []struct {
		name    string
		addr    string
		want    Resolver
		wantErr string
	}{
		{
			name: "service",
			addr: "foo.service.consul",
			want: &ConsulResolver{
				Client:    client,
				Namespace: "default",
				Name:      "foo",
				Type:      ConsulResolverTypeService,
			},
		},
		{
			name: "query",
			addr: "foo.query.consul",
			want: &ConsulResolver{
				Client:    client,
				Namespace: "default",
				Name:      "foo",
				Type:      ConsulResolverTypePreparedQuery,
			},
		},
		{
			name: "service with dc",
			addr: "foo.service.dc2.consul",
			want: &ConsulResolver{
				Client:     client,
				Datacenter: "dc2",
				Namespace:  "default",
				Name:       "foo",
				Type:       ConsulResolverTypeService,
			},
		},
		{
			name: "query with dc",
			addr: "foo.query.dc2.consul",
			want: &ConsulResolver{
				Client:     client,
				Datacenter: "dc2",
				Namespace:  "default",
				Name:       "foo",
				Type:       ConsulResolverTypePreparedQuery,
			},
		},
		{
			name:    "invalid host:port",
			addr:    "%%%",
			wantErr: "invalid Consul DNS domain",
		},
		{
			name:    "custom domain",
			addr:    "foo.service.my-consul.com",
			wantErr: "invalid Consul DNS domain",
		},
		{
			name:    "unsupported query type",
			addr:    "foo.connect.consul",
			wantErr: "unsupported Consul DNS domain",
		},
		{
			name:    "unsupported query type and datacenter",
			addr:    "foo.connect.dc1.consul",
			wantErr: "unsupported Consul DNS domain",
		},
		{
			name:    "unsupported query type and datacenter",
			addr:    "foo.connect.dc1.consul",
			wantErr: "unsupported Consul DNS domain",
		},
		{
			name:    "unsupported tag filter",
			addr:    "tag1.foo.service.consul",
			wantErr: "unsupported Consul DNS domain",
		},
		{
			name:    "unsupported tag filter with DC",
			addr:    "tag1.foo.service.dc1.consul",
			wantErr: "unsupported Consul DNS domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fn := ConsulResolverFromAddrFunc(client)
			got, gotErr := fn(tt.addr)
			if tt.wantErr != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tt.wantErr)
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tt.want, got)
			}
		})
	}
}
