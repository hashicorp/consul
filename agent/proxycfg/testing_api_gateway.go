package proxycfg

import (
	"fmt"

	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
)

func TestConfigSnapshotAPIGateway(t testing.T) *ConfigSnapshot {
	roots, placeholderLeaf := TestCerts(t)

	entries := []structs.ConfigEntry{
		&structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": "tcp",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "api-gateway",
		},
	}

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: leafWatchID,
			Result:        placeholderLeaf,
		},
		{
			CorrelationID: gatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.APIGatewayConfigEntry{
					Kind: structs.APIGateway,
					Name: "api-gateway",
					Listeners: []structs.APIGatewayListener{
						{
							Name:     "",
							Hostname: "",
							Port:     8080,
							Protocol: structs.ListenerProtocolTCP,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{
									{
										Kind: structs.InlineCertificate,
										Name: "my-inline-certificate",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: gatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.BoundAPIGatewayConfigEntry{
					Kind: structs.BoundAPIGateway,
					Name: "api-gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "",
							Certificates: []structs.ResourceReference{
								{
									Kind: structs.InlineCertificate,
									Name: "my-inline-certificate",
								},
							},
							Routes: []structs.ResourceReference{
								{
									Kind: structs.TCPRoute,
									Name: "my-tcp-route",
								},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: routeConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.TCPRouteConfigEntry{
					Kind: structs.TCPRoute,
					Name: "my-tcp-route",
					Parents: []structs.ResourceReference{
						{
							Kind: structs.APIGateway,
							Name: "api-gateway",
						},
					},
					Services: []structs.TCPService{
						{Name: "my-tcp-service"},
					},
				},
			},
		},
		{
			CorrelationID: inlineCertificateConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.InlineCertificateConfigEntry{
					Kind:        structs.InlineCertificate,
					Name:        "my-inline-certificate",
					Certificate: "certificate",
					PrivateKey:  "private key",
				},
			},
		},
		{
			CorrelationID: fmt.Sprintf("discovery-chain:%s", UpstreamIDString("","","my-tcp-service",nil, "")),
			Result: &structs.DiscoveryChainResponse{
				Chain: discoverychain.TestCompileConfigEntries(t,"my-tcp-service","default","default","dc1", connect.TestClusterID+".consul",nil,entries...),
			},
		},
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:            structs.ServiceKindAPIGateway,
		Service:         "api-gateway",
		Port:            9999,
		Address:         "1.2.3.4",
		Meta:            nil,
		TaggedAddresses: nil,
	}, nil, nil, testSpliceEvents(baseEvents, nil))
}
