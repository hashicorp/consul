// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func TestAgent_sidecarServiceFromNodeService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		name       string
		sd         *structs.ServiceDefinition
		token      string
		wantNS     *structs.NodeService
		wantChecks []*structs.CheckType
		wantToken  string
		wantErr    string
	}{
		{
			name: "no sidecar",
			sd: &structs.ServiceDefinition{
				Name: "web",
				Port: 1111,
			},
			token:      "foo",
			wantNS:     nil,
			wantChecks: nil,
			wantToken:  "",
			wantErr:    "", // Should NOT error
		},
		{
			name: "all the defaults",
			sd: &structs.ServiceDefinition{
				ID:   "web1",
				Name: "web",
				Port: 1111,
				Connect: &structs.ServiceConnect{
					SidecarService: &structs.ServiceDefinition{},
				},
			},
			token: "foo",
			wantNS: &structs.NodeService{
				EnterpriseMeta:             *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Kind:                       structs.ServiceKindConnectProxy,
				ID:                         "web1-sidecar-proxy",
				Service:                    "web-sidecar-proxy",
				Port:                       0,
				LocallyRegisteredAsSidecar: true,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web1",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1111,
				},
			},
			wantChecks: nil,
			wantToken:  "foo",
		},
		{
			name: "all the allowed overrides",
			sd: &structs.ServiceDefinition{
				ID:   "web1",
				Name: "web",
				Port: 1111,
				Tags: []string{"baz"},
				Meta: map[string]string{"foo": "baz"},
				Connect: &structs.ServiceConnect{
					SidecarService: &structs.ServiceDefinition{
						Name:    "motorbike1",
						Port:    3333,
						Tags:    []string{"foo", "bar"},
						Address: "127.127.127.127",
						Meta:    map[string]string{"foo": "bar"},
						Check: structs.CheckType{
							ScriptArgs: []string{"sleep", "1"},
							Interval:   999 * time.Second,
						},
						Token:             "custom-token",
						EnableTagOverride: true,
						Proxy: &structs.ConnectProxyConfig{
							DestinationServiceName: "web",
							DestinationServiceID:   "web1",
							LocalServiceAddress:    "127.0.127.0",
							LocalServicePort:       9999,
							Config:                 map[string]interface{}{"baz": "qux"},
							Upstreams:              structs.TestUpstreams(t, false),
						},
					},
				},
			},
			token: "foo",
			wantNS: &structs.NodeService{
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Kind:           structs.ServiceKindConnectProxy,
				ID:             "web1-sidecar-proxy",
				Service:        "motorbike1",
				Port:           3333,
				Tags:           []string{"foo", "bar"},
				Address:        "127.127.127.127",
				Meta: map[string]string{
					"foo": "bar",
				},
				LocallyRegisteredAsSidecar: true,
				EnableTagOverride:          true,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web1",
					LocalServiceAddress:    "127.0.127.0",
					LocalServicePort:       9999,
					Config:                 map[string]interface{}{"baz": "qux"},
					Upstreams: structs.TestAddDefaultsToUpstreams(t, structs.TestUpstreams(t, false),
						*structs.DefaultEnterpriseMetaInDefaultPartition()),
				},
			},
			wantChecks: []*structs.CheckType{
				{
					ScriptArgs: []string{"sleep", "1"},
					Interval:   999 * time.Second,
				},
			},
			wantToken: "custom-token",
		},
		{
			name: "inherit tags and meta",
			sd: &structs.ServiceDefinition{
				ID:   "web1",
				Name: "web",
				Port: 1111,
				Tags: []string{"foo"},
				Meta: map[string]string{"foo": "bar"},
				Connect: &structs.ServiceConnect{
					SidecarService: &structs.ServiceDefinition{},
				},
			},
			wantNS: &structs.NodeService{
				EnterpriseMeta:             *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Kind:                       structs.ServiceKindConnectProxy,
				ID:                         "web1-sidecar-proxy",
				Service:                    "web-sidecar-proxy",
				Port:                       0,
				Tags:                       []string{"foo"},
				Meta:                       map[string]string{"foo": "bar"},
				LocallyRegisteredAsSidecar: true,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web1",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1111,
				},
			},
			wantChecks: nil,
		},
		{
			name: "invalid check type",
			sd: &structs.ServiceDefinition{
				ID:   "web1",
				Name: "web",
				Port: 1111,
				Connect: &structs.ServiceConnect{
					SidecarService: &structs.ServiceDefinition{
						Check: structs.CheckType{
							TCP: "foo",
							// Invalid since no interval specified
						},
					},
				},
			},
			token:   "foo",
			wantErr: "Interval must be > 0",
		},
		{
			name: "invalid meta",
			sd: &structs.ServiceDefinition{
				ID:   "web1",
				Name: "web",
				Port: 1111,
				Connect: &structs.ServiceConnect{
					SidecarService: &structs.ServiceDefinition{
						Meta: map[string]string{
							"consul-reserved-key-should-be-rejected": "true",
						},
					},
				},
			},
			token:   "foo",
			wantErr: "reserved for internal use",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.sd.NodeService()
			err := ns.Validate()
			require.NoError(t, err, "Invalid test case - NodeService must validate")

			gotNS, gotChecks, gotToken, err := sidecarServiceFromNodeService(ns, tt.token)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantNS, gotNS)
			require.Equal(t, tt.wantChecks, gotChecks)
			require.Equal(t, tt.wantToken, gotToken)
		})
	}
}

func TestAgent_SidecarPortFromServiceID(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		name              string
		autoPortsDisabled bool
		enterpriseMeta    acl.EnterpriseMeta
		maxPort           int
		port              int
		preRegister       []*structs.ServiceDefinition
		serviceID         string
		wantPort          int
		wantErr           string
	}{
		{
			name:      "use auto ports",
			serviceID: "web1",
			wantPort:  2222,
		},
		{
			name: "re-registering same sidecar with no port should pick same one",
			// Allow multiple ports to be sure we get the right one
			maxPort: 2500,
			// Pre register the main service and sidecar we want
			preRegister: []*structs.ServiceDefinition{{
				Kind:           structs.ServiceKindConnectProxy,
				ID:             "web1",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Name:           "web",
				Port:           2221,
				Proxy: &structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web1",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1110,
				},
			}, {
				Kind:           structs.ServiceKindConnectProxy,
				ID:             "web1-sidecar-proxy",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Name:           "web-sidecar-proxy",
				Port:           2222,
				Proxy: &structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web1",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1111,
				},
			}},
			// Register same sidecar again
			serviceID:      "web1-sidecar-proxy",
			enterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			wantPort:       2222, // Should claim the same port as before
		},
		{
			name: "all auto ports already taken",
			// register another service with sidecar consuming our 1 and only allocated auto port.
			preRegister: []*structs.ServiceDefinition{{
				Kind:           structs.ServiceKindConnectProxy,
				ID:             "api1",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Name:           "api",
				Port:           2221,
				Proxy: &structs.ConnectProxyConfig{
					DestinationServiceName: "api",
					DestinationServiceID:   "api1",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1110,
				},
			}, {
				Kind: structs.ServiceKindConnectProxy,
				Name: "api-proxy-sidecar",
				Port: 2222, // Consume the one available auto-port
				Proxy: &structs.ConnectProxyConfig{
					DestinationServiceID:   "api1",
					DestinationServiceName: "api",
				},
			}},
			wantErr: "none left in the configured range [2222, 2222]",
		},
		{
			name:              "auto ports disabled",
			autoPortsDisabled: true,
			wantErr:           "auto-assignment disabled in config",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set port range to be tiny (one available) to test consuming all of it.
			// This allows a single assigned port at 2222 thanks to being inclusive at
			// both ends.
			if tt.maxPort == 0 {
				tt.maxPort = 2222
			}
			hcl := fmt.Sprintf(`
			ports {
				sidecar_min_port = 2222
				sidecar_max_port = %d
			}
			`, tt.maxPort)
			if tt.autoPortsDisabled {
				hcl = `
				ports {
					sidecar_min_port = 0
					sidecar_max_port = 0
				}
				`
			}
			a := NewTestAgent(t, hcl)
			defer a.Shutdown()

			if len(tt.preRegister) > 0 {
				for _, s := range tt.preRegister {
					err := a.addServiceFromSource(s.NodeService(), nil, false, "", ConfigSourceLocal)
					require.NoError(t, err)
				}
			}

			gotPort, err := a.sidecarPortFromServiceIDLocked(structs.ServiceID{ID: tt.serviceID, EnterpriseMeta: tt.enterpriseMeta})

			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantPort, gotPort)
		})
	}
}

func TestAgent_SidecarDefaultChecks(t *testing.T) {
	tests := []struct {
		name                 string
		serviceID            string
		svcAddress           string
		proxyLocalSvcAddress string
		port                 int
		wantChecks           []*structs.CheckType
	}{{
		name:                 "uses proxy address for check",
		serviceID:            "web1-1-sidecar-proxy",
		svcAddress:           "123.123.123.123",
		proxyLocalSvcAddress: "255.255.255.255",
		port:                 2222,
		wantChecks: []*structs.CheckType{
			{
				Name:     "Connect Sidecar Listening",
				TCP:      "123.123.123.123:2222",
				Interval: 10 * time.Second,
			},
			{
				Name:         "Connect Sidecar Aliasing web1-1",
				AliasService: "web1-1",
			},
		},
	},
		{
			name:                 "uses proxy.local_service_address for check if proxy address is empty",
			serviceID:            "web1-1-sidecar-proxy",
			proxyLocalSvcAddress: "1.2.3.4",
			port:                 2222,
			wantChecks: []*structs.CheckType{
				{
					Name:     "Connect Sidecar Listening",
					TCP:      "1.2.3.4:2222",
					Interval: 10 * time.Second,
				},
				{
					Name:         "Connect Sidecar Aliasing web1-1",
					AliasService: "web1-1",
				},
			},
		},
		{
			name:                 "redundant name",
			serviceID:            "1-sidecar-proxy-web1-sidecar-proxy",
			svcAddress:           "123.123.123.123",
			proxyLocalSvcAddress: "255.255.255.255",
			port:                 2222,
			wantChecks: []*structs.CheckType{
				{
					Name:     "Connect Sidecar Listening",
					TCP:      "123.123.123.123:2222",
					Interval: 10 * time.Second,
				},
				{
					Name:         "Connect Sidecar Aliasing 1-sidecar-proxy-web1",
					AliasService: "1-sidecar-proxy-web1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotChecks := sidecarDefaultChecks(tt.serviceID, tt.svcAddress, tt.proxyLocalSvcAddress, tt.port)
			require.Equal(t, tt.wantChecks, gotChecks)
		})
	}
}
