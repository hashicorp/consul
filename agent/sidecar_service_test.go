package agent

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestAgent_sidecarServiceFromNodeService(t *testing.T) {
	tests := []struct {
		name              string
		maxPort           int
		preRegister       *structs.ServiceDefinition
		sd                *structs.ServiceDefinition
		token             string
		autoPortsDisabled bool
		wantNS            *structs.NodeService
		wantChecks        []*structs.CheckType
		wantToken         string
		wantErr           string
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
				Kind:                       structs.ServiceKindConnectProxy,
				ID:                         "web1-sidecar-proxy",
				Service:                    "web-sidecar-proxy",
				Port:                       2222,
				LocallyRegisteredAsSidecar: true,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web1",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1111,
				},
			},
			wantChecks: []*structs.CheckType{
				&structs.CheckType{
					Name:     "Connect Sidecar Listening",
					TCP:      "127.0.0.1:2222",
					Interval: 10 * time.Second,
				},
				&structs.CheckType{
					Name:         "Connect Sidecar Aliasing web1",
					AliasService: "web1",
				},
			},
			wantToken: "foo",
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
							Upstreams:              structs.TestUpstreams(t),
						},
					},
				},
			},
			token: "foo",
			wantNS: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "web1-sidecar-proxy",
				Service: "motorbike1",
				Port:    3333,
				Tags:    []string{"foo", "bar"},
				Address: "127.127.127.127",
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
					Upstreams:              structs.TestAddDefaultsToUpstreams(t, structs.TestUpstreams(t)),
				},
			},
			wantChecks: []*structs.CheckType{
				&structs.CheckType{
					ScriptArgs: []string{"sleep", "1"},
					Interval:   999 * time.Second,
				},
			},
			wantToken: "custom-token",
		},
		{
			name: "no auto ports available",
			// register another sidecar consuming our 1 and only allocated auto port.
			preRegister: &structs.ServiceDefinition{
				Kind: structs.ServiceKindConnectProxy,
				Name: "api-proxy-sidecar",
				Port: 2222, // Consume the one available auto-port
				Proxy: &structs.ConnectProxyConfig{
					DestinationServiceName: "api",
				},
			},
			sd: &structs.ServiceDefinition{
				ID:   "web1",
				Name: "web",
				Port: 1111,
				Connect: &structs.ServiceConnect{
					SidecarService: &structs.ServiceDefinition{},
				},
			},
			token:   "foo",
			wantErr: "none left in the configured range [2222, 2222]",
		},
		{
			name:              "auto ports disabled",
			autoPortsDisabled: true,
			sd: &structs.ServiceDefinition{
				ID:   "web1",
				Name: "web",
				Port: 1111,
				Connect: &structs.ServiceConnect{
					SidecarService: &structs.ServiceDefinition{},
				},
			},
			token:   "foo",
			wantErr: "auto-assignment disabled in config",
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
				Kind:                       structs.ServiceKindConnectProxy,
				ID:                         "web1-sidecar-proxy",
				Service:                    "web-sidecar-proxy",
				Port:                       2222,
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
			wantChecks: []*structs.CheckType{
				&structs.CheckType{
					Name:     "Connect Sidecar Listening",
					TCP:      "127.0.0.1:2222",
					Interval: 10 * time.Second,
				},
				&structs.CheckType{
					Name:         "Connect Sidecar Aliasing web1",
					AliasService: "web1",
				},
			},
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
		{
			name: "re-registering same sidecar with no port should pick same one",
			// Allow multiple ports to be sure we get the right one
			maxPort: 2500,
			// Pre register the sidecar we want
			preRegister: &structs.ServiceDefinition{
				Kind: structs.ServiceKindConnectProxy,
				ID:   "web1-sidecar-proxy",
				Name: "web-sidecar-proxy",
				Port: 2222,
				Proxy: &structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web1",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1111,
				},
			},
			// Register same again but with different service port
			sd: &structs.ServiceDefinition{
				ID:   "web1",
				Name: "web",
				Port: 1112,
				Connect: &structs.ServiceConnect{
					SidecarService: &structs.ServiceDefinition{},
				},
			},
			token: "foo",
			wantNS: &structs.NodeService{
				Kind:                       structs.ServiceKindConnectProxy,
				ID:                         "web1-sidecar-proxy",
				Service:                    "web-sidecar-proxy",
				Port:                       2222, // Should claim the same port as before
				LocallyRegisteredAsSidecar: true,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web1",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1112,
				},
			},
			wantChecks: []*structs.CheckType{
				&structs.CheckType{
					Name:     "Connect Sidecar Listening",
					TCP:      "127.0.0.1:2222",
					Interval: 10 * time.Second,
				},
				&structs.CheckType{
					Name:         "Connect Sidecar Aliasing web1",
					AliasService: "web1",
				},
			},
			wantToken: "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set port range to be tiny (one availabl) to test consuming all of it.
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

			require := require.New(t)
			a := NewTestAgent(t, "jones", hcl)
			defer a.Shutdown()

			if tt.preRegister != nil {
				err := a.AddService(tt.preRegister.NodeService(), nil, false, "", ConfigSourceLocal)
				require.NoError(err)
			}

			ns := tt.sd.NodeService()
			err := ns.Validate()
			require.NoError(err, "Invalid test case - NodeService must validate")

			gotNS, gotChecks, gotToken, err := a.sidecarServiceFromNodeService(ns, tt.token)
			if tt.wantErr != "" {
				require.Error(err)
				require.Contains(err.Error(), tt.wantErr)
				return
			}

			require.NoError(err)
			require.Equal(tt.wantNS, gotNS)
			require.Equal(tt.wantChecks, gotChecks)
			require.Equal(tt.wantToken, gotToken)
		})
	}
}
