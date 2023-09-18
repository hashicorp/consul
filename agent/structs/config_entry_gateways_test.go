package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIngressGatewayConfigEntry(t *testing.T) {
	defaultMeta := DefaultEnterpriseMetaInDefaultPartition()

	cases := map[string]configEntryTestcase{
		"normalize: empty protocol": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "",
						Services: []IngressService{},
					},
				},
			},
			expected: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{},
					},
				},
				EnterpriseMeta: *defaultMeta,
			},
			normalizeOnly: true,
		},
		"normalize: lowercase protocols": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "TCP",
						Services: []IngressService{},
					},
					{
						Port:     1112,
						Protocol: "HtTP",
						Services: []IngressService{},
					},
				},
			},
			expected: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{},
					},
					{
						Port:     1112,
						Protocol: "http",
						Services: []IngressService{},
					},
				},
				EnterpriseMeta: *defaultMeta,
			},
			normalizeOnly: true,
		},
		"port conflict": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "mysql",
							},
						},
					},
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "postgres",
							},
						},
					},
				},
			},
			validateErr: "port 1111 declared on two listeners",
		},
		"http features: wildcard": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name: "*",
							},
						},
					},
				},
			},
		},
		"http features: wildcard service on invalid protocol": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "*",
							},
						},
					},
				},
			},
			validateErr: "Wildcard service name is only valid for protocol",
		},
		"http features: multiple services": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name: "db1",
							},
							{
								Name: "db2",
							},
						},
					},
				},
			},
		},
		"http features: multiple services on tcp listener": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "db1",
							},
							{
								Name: "db2",
							},
						},
					},
				},
			},
			validateErr: "Multiple services per listener are only supported for L7",
		},
		// ==========================
		"tcp listener requires a defined service": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{},
					},
				},
			},
			validateErr: "No service declared for listener with port 1111",
		},
		"http listener requires a defined service": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{},
					},
				},
			},
			validateErr: "No service declared for listener with port 1111",
		},
		"empty service name not supported": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{},
						},
					},
				},
			},
			validateErr: "Service name cannot be blank",
		},
		"protocol validation": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "asdf",
						Services: []IngressService{
							{
								Name: "db",
							},
						},
					},
				},
			},
			validateErr: "protocol must be 'tcp', 'http', 'http2', or 'grpc'. 'asdf' is an unsupported protocol",
		},
		"hosts cannot be set on a tcp listener": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name:  "db",
								Hosts: []string{"db.example.com"},
							},
						},
					},
				},
			},
			validateErr: "Associating hosts to a service is not supported for the tcp protocol",
		},
		"hosts cannot be set on a wildcard specifier": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "*",
								Hosts: []string{"db.example.com"},
							},
						},
					},
				},
			},
			validateErr: "Associating hosts to a wildcard service is not supported",
		},
		"hosts must be unique per listener": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "db",
								Hosts: []string{"test.example.com"},
							},
							{
								Name:  "api",
								Hosts: []string{"test.example.com"},
							},
						},
					},
				},
			},
			validateErr: "Hosts must be unique within a specific listener",
		},
		"hosts must be a valid DNS name": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "db",
								Hosts: []string{"example..com"},
							},
						},
					},
				},
			},
			validateErr: `Host "example..com" must be a valid DNS hostname`,
		},
		"wildcard specifier is only allowed in the leftmost label": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "db",
								Hosts: []string{"*.example.com"},
							},
						},
					},
				},
			},
		},
		"wildcard specifier is not allowed in non-leftmost labels": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "db",
								Hosts: []string{"example.*.com"},
							},
						},
					},
				},
			},
			validateErr: `Host "example.*.com" is not valid, a wildcard specifier is only allowed as the leftmost label`,
		},
		"wildcard specifier is not allowed in leftmost labels as a partial": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "db",
								Hosts: []string{"*-test.example.com"},
							},
						},
					},
				},
			},
			validateErr: `Host "*-test.example.com" is not valid, a wildcard specifier is only allowed as the leftmost label`,
		},
		"wildcard specifier is allowed for hosts when TLS is disabled": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "db",
								Hosts: []string{"*"},
							},
						},
					},
				},
			},
		},
		"wildcard specifier is not allowed for hosts when TLS is enabled": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				TLS: GatewayTLSConfig{
					Enabled: true,
				},
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "db",
								Hosts: []string{"*"},
							},
						},
					},
				},
			},
			validateErr: `Host '*' is not allowed when TLS is enabled, all hosts must be valid DNS records to add as a DNSSAN`,
		},
		"request header manip allowed for http(ish) protocol": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name: "web",
								RequestHeaders: &HTTPHeaderModifiers{
									Set: map[string]string{"x-foo": "bar"},
								},
							},
						},
					},
					{
						Port:     2222,
						Protocol: "http2",
						Services: []IngressService{
							{
								Name: "web2",
								ResponseHeaders: &HTTPHeaderModifiers{
									Set: map[string]string{"x-foo": "bar"},
								},
							},
						},
					},
					{
						Port:     3333,
						Protocol: "grpc",
						Services: []IngressService{
							{
								Name: "api",
								ResponseHeaders: &HTTPHeaderModifiers{
									Remove: []string{"x-grpc-internal"},
								},
							},
						},
					},
				},
			},
			expectUnchanged: true,
		},
		"request header manip not allowed for non-http protocol": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "db",
								RequestHeaders: &HTTPHeaderModifiers{
									Set: map[string]string{"x-foo": "bar"},
								},
							},
						},
					},
				},
			},
			validateErr: "request headers only valid for http",
		},
		"response header manip not allowed for non-http protocol": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "db",
								ResponseHeaders: &HTTPHeaderModifiers{
									Remove: []string{"x-foo"},
								},
							},
						},
					},
				},
			},
			validateErr: "response headers only valid for http",
		},
		"duplicate services not allowed": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name: "web",
							},
							{
								Name: "web",
							},
						},
					},
				},
			},
			// Match only the last part of the exected error because the service name
			// differs between Ent and CE default/default/web vs web
			validateErr: "cannot be added multiple times (listener on port 1111)",
		},
		"TLS.SDS kitchen sink": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				TLS: GatewayTLSConfig{
					SDS: &GatewayTLSSDSConfig{
						ClusterName:  "secret-service1",
						CertResource: "some-ns/ingress-default",
					},
				},
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						TLS: &GatewayTLSConfig{
							SDS: &GatewayTLSSDSConfig{
								ClusterName:  "secret-service2",
								CertResource: "some-ns/ingress-1111",
							},
						},
						Services: []IngressService{
							{
								Name:  "web",
								Hosts: []string{"*"},
								TLS: &GatewayServiceTLSConfig{
									SDS: &GatewayTLSSDSConfig{
										ClusterName:  "secret-service3",
										CertResource: "some-ns/web",
									},
								},
							},
						},
					},
				},
			},
		},
		"TLS.SDS gateway-level": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				TLS: GatewayTLSConfig{
					SDS: &GatewayTLSSDSConfig{
						ClusterName:  "secret-service1",
						CertResource: "some-ns/ingress-default",
					},
				},
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "db",
							},
						},
					},
				},
			},
			expectUnchanged: true,
		},
		"TLS.SDS listener-level": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						TLS: &GatewayTLSConfig{
							SDS: &GatewayTLSSDSConfig{
								ClusterName:  "secret-service1",
								CertResource: "some-ns/db1",
							},
						},
						Services: []IngressService{
							{
								Name: "db1",
							},
						},
					},
					{
						Port:     2222,
						Protocol: "tcp",
						TLS: &GatewayTLSConfig{
							SDS: &GatewayTLSSDSConfig{
								ClusterName:  "secret-service2",
								CertResource: "some-ns/db2",
							},
						},
						Services: []IngressService{
							{
								Name: "db2",
							},
						},
					},
				},
			},
			expectUnchanged: true,
		},
		"TLS.SDS gateway-level cluster only": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				TLS: GatewayTLSConfig{
					SDS: &GatewayTLSSDSConfig{
						ClusterName: "secret-service",
					},
				},
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						TLS: &GatewayTLSConfig{
							SDS: &GatewayTLSSDSConfig{
								CertResource: "some-ns/db1",
							},
						},
						Services: []IngressService{
							{
								Name: "db1",
							},
						},
					},
					{
						Port:     2222,
						Protocol: "tcp",
						TLS: &GatewayTLSConfig{
							SDS: &GatewayTLSSDSConfig{
								CertResource: "some-ns/db2",
							},
						},
						Services: []IngressService{
							{
								Name: "db2",
							},
						},
					},
				},
			},
			expectUnchanged: true,
		},
		"TLS.SDS mixed TLS and non-TLS listeners": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				// No Gateway level TLS.Enabled or SDS config
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						TLS: &GatewayTLSConfig{
							SDS: &GatewayTLSSDSConfig{
								ClusterName:  "sds-cluster",
								CertResource: "some-ns/db1",
							},
						},
						Services: []IngressService{
							{
								Name: "db1",
							},
						},
					},
					{
						Port:     2222,
						Protocol: "tcp",
						// No TLS config
						Services: []IngressService{
							{
								Name: "db2",
							},
						},
					},
				},
			},
			expectUnchanged: true,
		},
		"TLS.SDS only service-level mixed": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				// No Gateway level TLS.Enabled or SDS config
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						// No TLS config
						Services: []IngressService{
							{
								Name:  "web",
								Hosts: []string{"www.example.com"},
								TLS: &GatewayServiceTLSConfig{
									SDS: &GatewayTLSSDSConfig{
										ClusterName:  "sds-cluster",
										CertResource: "web-cert",
									},
								},
							},
							{
								Name:  "api",
								Hosts: []string{"api.example.com"},
								TLS: &GatewayServiceTLSConfig{
									SDS: &GatewayTLSSDSConfig{
										ClusterName:  "sds-cluster",
										CertResource: "api-cert",
									},
								},
							},
						},
					},
					{
						Port:     2222,
						Protocol: "http",
						// No TLS config
						Services: []IngressService{
							{
								Name: "db2",
							},
						},
					},
				},
			},
			expectUnchanged: true,
		},
		"TLS.SDS requires cluster if gateway-level cert specified": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				TLS: GatewayTLSConfig{
					SDS: &GatewayTLSSDSConfig{
						CertResource: "foo",
					},
				},
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "db",
							},
						},
					},
				},
			},
			validateErr: "TLS.SDS.ClusterName is required if CertResource is set",
		},
		"TLS.SDS listener requires cluster if there is no gateway-level one": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",

				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						TLS: &GatewayTLSConfig{
							SDS: &GatewayTLSSDSConfig{
								CertResource: "foo",
							},
						},
						Services: []IngressService{
							{
								Name: "db",
							},
						},
					},
				},
			},
			validateErr: "TLS.SDS.ClusterName is required if CertResource is set",
		},
		"TLS.SDS listener requires a cert resource if gw ClusterName set": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				TLS: GatewayTLSConfig{
					SDS: &GatewayTLSSDSConfig{
						ClusterName: "foo",
					},
				},
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "db",
							},
						},
					},
				},
			},
			validateErr: "TLS.SDS.CertResource is required if ClusterName is set for gateway (listener on port 1111)",
		},
		"TLS.SDS listener requires a cert resource if listener ClusterName set": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",

				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						TLS: &GatewayTLSConfig{
							SDS: &GatewayTLSSDSConfig{
								ClusterName: "foo",
							},
						},
						Services: []IngressService{
							{
								Name: "db",
							},
						},
					},
				},
			},
			validateErr: "TLS.SDS.CertResource is required if ClusterName is set for listener (listener on port 1111)",
		},
		"TLS.SDS at service level is not supported without Hosts set": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",

				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name: "*",
								TLS: &GatewayServiceTLSConfig{
									SDS: &GatewayTLSSDSConfig{
										CertResource: "foo",
										ClusterName:  "sds-cluster",
									},
								},
							},
						},
					},
				},
			},
			// Note we don't assert the last part `(service \"*\" on listener on port 1111)`
			// since the service name is normalized differently on CE and Ent
			validateErr: "A service specifying TLS.SDS.CertResource must have at least one item in Hosts",
		},
		"TLS.SDS at service level needs a cluster from somewhere": {
			entry: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",

				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "foo",
								Hosts: []string{"foo.example.com"},
								TLS: &GatewayServiceTLSConfig{
									SDS: &GatewayTLSSDSConfig{
										CertResource: "foo",
									},
								},
							},
						},
					},
				},
			},
			// Note we don't assert the last part `(service \"foo\" on listener on port 1111)`
			// since the service name is normalized differently on CE and Ent
			validateErr: "TLS.SDS.ClusterName is required if CertResource is set",
		},
	}

	testConfigEntryNormalizeAndValidate(t, cases)
}

func TestIngressConfigEntry_ListRelatedServices(t *testing.T) {
	type testcase struct {
		entry          IngressGatewayConfigEntry
		expectServices []ServiceID
	}

	cases := map[string]testcase{
		"one exact": {
			entry: IngressGatewayConfigEntry{
				Kind: IngressGateway,
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{Name: "web"},
						},
					},
				},
			},
			expectServices: []ServiceID{NewServiceID("web", nil)},
		},
		"one wild": {
			entry: IngressGatewayConfigEntry{
				Kind: IngressGateway,
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{Name: "*"},
						},
					},
				},
			},
			expectServices: nil,
		},
		"kitchen sink": {
			entry: IngressGatewayConfigEntry{
				Kind: IngressGateway,
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{Name: "api"},
							{Name: "web"},
						},
					},
					{
						Port:     2222,
						Protocol: "tcp",
						Services: []IngressService{
							{Name: "web"},
							{Name: "*"},
							{Name: "db"},
							{Name: "blah"},
						},
					},
				},
			},
			expectServices: []ServiceID{
				NewServiceID("api", nil),
				NewServiceID("blah", nil),
				NewServiceID("db", nil),
				NewServiceID("web", nil),
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got := tc.entry.ListRelatedServices()
			require.Equal(t, tc.expectServices, got)
		})
	}
}

func TestTerminatingGatewayConfigEntry(t *testing.T) {
	cases := map[string]configEntryTestcase{
		"service conflict": {
			entry: &TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name: "foo",
					},
					{
						Name: "foo",
					},
				},
			},
			validateErr: "specified more than once",
		},
		"blank service name": {
			entry: &TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name: "",
					},
				},
			},
			validateErr: "Service name cannot be blank.",
		},
		"not all TLS options provided-1": {
			entry: &TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name:     "web",
						CertFile: "client.crt",
					},
				},
			},
			validateErr: "must have a CertFile, CAFile, and KeyFile",
		},
		"not all TLS options provided-2": {
			entry: &TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name:    "web",
						KeyFile: "tls.key",
					},
				},
			},
			validateErr: "must have a CertFile, CAFile, and KeyFile",
		},
		"all TLS options provided": {
			entry: &TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name:     "web",
						CAFile:   "ca.crt",
						CertFile: "client.crt",
						KeyFile:  "tls.key",
					},
				},
			},
		},
		"only providing ca file is allowed": {
			entry: &TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name:   "web",
						CAFile: "ca.crt",
					},
				},
			},
		},
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}

func TestGatewayService_Addresses(t *testing.T) {
	cases := []struct {
		name     string
		input    GatewayService
		argument []string
		expected []string
	}{
		{
			name:     "port is zero",
			input:    GatewayService{},
			expected: nil,
		},
		{
			name: "no hosts with empty array",
			input: GatewayService{
				Port: 8080,
			},
			expected: nil,
		},
		{
			name: "no hosts with default",
			input: GatewayService{
				Port: 8080,
			},
			argument: []string{
				"service.ingress.dc.domain",
				"service.ingress.dc.alt.domain",
				"service.ingress.dc.alt.domain.",
			},
			expected: []string{
				"service.ingress.dc.domain:8080",
				"service.ingress.dc.alt.domain:8080",
				"service.ingress.dc.alt.domain:8080",
			},
		},
		{
			name: "user-defined hosts",
			input: GatewayService{
				Port:  8080,
				Hosts: []string{"*.test.example.com", "other.example.com", "other.example.com."},
			},
			argument: []string{
				"service.ingress.dc.domain",
				"service.ingress.alt.domain",
			},
			expected: []string{"*.test.example.com:8080", "other.example.com:8080", "other.example.com:8080"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addresses := tc.input.Addresses(tc.argument)
			require.ElementsMatch(t, tc.expected, addresses)
		})
	}
}

func TestAPIGateway_Listeners(t *testing.T) {
	cases := map[string]configEntryTestcase{
		"no listeners defined": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-one",
			},
			validateErr: "api gateway must have at least one listener",
		},
		"listener name conflict": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-one",
				Listeners: []APIGatewayListener{
					{
						Port: 80,
						Name: "foo",
					},
					{
						Port: 80,
						Name: "foo",
					},
				},
			},
			validateErr: "multiple listeners with the name",
		},
		"empty listener name": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-one",
				Listeners: []APIGatewayListener{
					{
						Port:     80,
						Protocol: "tcp",
					},
				},
			},
			validateErr: "listener name \"\" is invalid, must be at least 1 character and contain only letters, numbers, or dashes",
		},
		"invalid listener name": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-one",
				Listeners: []APIGatewayListener{
					{
						Port:     80,
						Protocol: "tcp",
						Name:     "/",
					},
				},
			},
			validateErr: "listener name \"/\" is invalid, must be at least 1 character and contain only letters, numbers, or dashes",
		},
		"merged listener protocol conflict": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-two",
				Listeners: []APIGatewayListener{
					{
						Name:     "listener-one",
						Port:     80,
						Protocol: ListenerProtocolHTTP,
					},
					{
						Name:     "foo",
						Port:     80,
						Protocol: ListenerProtocolTCP,
					},
				},
			},
			validateErr: "cannot be merged",
		},
		"merged listener hostname conflict": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-three",
				Listeners: []APIGatewayListener{
					{
						Name:     "listener",
						Port:     80,
						Hostname: "host.one",
					},
					{
						Name:     "foo",
						Port:     80,
						Hostname: "host.two",
					},
				},
			},
			validateErr: "cannot be merged",
		},
		"invalid protocol": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-four",
				Listeners: []APIGatewayListener{
					{
						Name:     "listener",
						Port:     80,
						Hostname: "host.one",
						Protocol: APIGatewayListenerProtocol("UDP"),
					},
				},
			},
			validateErr: "unsupported listener protocol",
		},
		"hostname in unsupported protocol": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-five",
				Listeners: []APIGatewayListener{
					{
						Name:     "listener",
						Port:     80,
						Hostname: "host.one",
						Protocol: APIGatewayListenerProtocol("tcp"),
					},
				},
			},
			validateErr: "hostname specification is not supported",
		},
		"invalid port": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-six",
				Listeners: []APIGatewayListener{
					{
						Name:     "listener",
						Port:     -1,
						Protocol: APIGatewayListenerProtocol("tcp"),
					},
				},
			},
			validateErr: "not in the range 1-65535",
		},
		"invalid hostname": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-seven",
				Listeners: []APIGatewayListener{
					{
						Name:     "listener",
						Port:     80,
						Hostname: "*.*.host.one",
						Protocol: APIGatewayListenerProtocol("http"),
					},
				},
			},
			validateErr: "only allowed as the left-most label",
		},
		"unsupported certificate kind": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-eight",
				Listeners: []APIGatewayListener{
					{
						Name:     "listener",
						Port:     80,
						Hostname: "host.one",
						Protocol: APIGatewayListenerProtocol("http"),
						TLS: APIGatewayTLSConfiguration{
							Certificates: []ResourceReference{{
								Kind: APIGateway,
							}},
						},
					},
				},
			},
			validateErr: "unsupported certificate kind",
		},
		"unnamed certificate": {
			entry: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "api-gw-nine",
				Listeners: []APIGatewayListener{
					{
						Name:     "listener",
						Port:     80,
						Hostname: "host.one",
						Protocol: APIGatewayListenerProtocol("http"),
						TLS: APIGatewayTLSConfiguration{
							Certificates: []ResourceReference{{
								Kind: InlineCertificate,
							}},
						},
					},
				},
			},
			validateErr: "certificate reference must have a name",
		},
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}

func TestBoundAPIGateway(t *testing.T) {
	cases := map[string]configEntryTestcase{
		"invalid certificate, no name": {
			entry: &BoundAPIGatewayConfigEntry{
				Kind: BoundAPIGateway,
				Name: "bound-api-gw-one",
				Listeners: []BoundAPIGatewayListener{
					{
						Name: "one",
						Certificates: []ResourceReference{{
							Kind: InlineCertificate,
						}},
					},
				},
			},
			validateErr: "certificate reference must have a name",
		},
		"invalid certificate, no kind": {
			entry: &BoundAPIGatewayConfigEntry{
				Kind: BoundAPIGateway,
				Name: "bound-api-gw-two",
				Listeners: []BoundAPIGatewayListener{
					{
						Name: "one",
						Certificates: []ResourceReference{{
							Name: "foo",
						}},
					},
				},
			},
			validateErr: "unsupported certificate kind",
		},
		"invalid route, no name": {
			entry: &BoundAPIGatewayConfigEntry{
				Kind: BoundAPIGateway,
				Name: "bound-api-gw-three",
				Listeners: []BoundAPIGatewayListener{
					{
						Name: "one",
						Routes: []ResourceReference{{
							Kind: TCPRoute,
						}},
					},
				},
			},
			validateErr: "route reference must have a name",
		},
		"invalid route, no kind": {
			entry: &BoundAPIGatewayConfigEntry{
				Kind: BoundAPIGateway,
				Name: "bound-api-gw-four",
				Listeners: []BoundAPIGatewayListener{
					{
						Name: "one",
						Routes: []ResourceReference{{
							Name: "foo",
						}},
					},
				},
			},
			validateErr: "unsupported route kind",
		},
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}

func TestListenerBindRoute(t *testing.T) {
	cases := map[string]struct {
		listener         BoundAPIGatewayListener
		route            BoundRoute
		expectedListener BoundAPIGatewayListener
		expectedDidBind  bool
	}{
		"Listener has no routes": {
			listener: BoundAPIGatewayListener{},
			route: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "Test Route",
			},
			expectedListener: BoundAPIGatewayListener{
				Routes: []ResourceReference{
					{
						Kind: TCPRoute,
						Name: "Test Route",
					},
				},
			},
			expectedDidBind: true,
		},
		"Listener to update existing route": {
			listener: BoundAPIGatewayListener{
				Routes: []ResourceReference{
					{
						Kind: TCPRoute,
						Name: "Test Route 1",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 2",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 3",
					},
				},
			},
			route: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "Test Route 2",
			},
			expectedListener: BoundAPIGatewayListener{
				Routes: []ResourceReference{
					{
						Kind: TCPRoute,
						Name: "Test Route 1",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 2",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 3",
					},
				},
			},
			expectedDidBind: true,
		},
		"Listener appends new route": {
			listener: BoundAPIGatewayListener{
				Routes: []ResourceReference{
					{
						Kind: TCPRoute,
						Name: "Test Route 1",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 2",
					},
				},
			},
			route: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "Test Route 3",
			},
			expectedListener: BoundAPIGatewayListener{
				Routes: []ResourceReference{
					{
						Kind: TCPRoute,
						Name: "Test Route 1",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 2",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 3",
					},
				},
			},
			expectedDidBind: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			routeRef := ResourceReference{
				Kind:           tc.route.GetKind(),
				Name:           tc.route.GetName(),
				EnterpriseMeta: *tc.route.GetEnterpriseMeta(),
			}
			actualDidBind := tc.listener.BindRoute(routeRef)
			require.Equal(t, tc.expectedDidBind, actualDidBind)
			require.Equal(t, tc.expectedListener.Routes, tc.listener.Routes)
		})
	}
}

func TestListenerUnbindRoute(t *testing.T) {
	cases := map[string]struct {
		listener          BoundAPIGatewayListener
		route             BoundRoute
		expectedListener  BoundAPIGatewayListener
		expectedDidUnbind bool
	}{
		"Listener has no routes": {
			listener: BoundAPIGatewayListener{},
			route: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "Test Route",
			},
			expectedListener:  BoundAPIGatewayListener{},
			expectedDidUnbind: false,
		},
		"Listener to remove existing route": {
			listener: BoundAPIGatewayListener{
				Routes: []ResourceReference{
					{
						Kind: TCPRoute,
						Name: "Test Route 1",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 2",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 3",
					},
				},
			},
			route: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "Test Route 2",
			},
			expectedListener: BoundAPIGatewayListener{
				Routes: []ResourceReference{
					{
						Kind: TCPRoute,
						Name: "Test Route 1",
					},
					{
						Kind: TCPRoute,
						Name: "Test Route 3",
					},
				},
			},
			expectedDidUnbind: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			routeRef := ResourceReference{
				Kind:           tc.route.GetKind(),
				Name:           tc.route.GetName(),
				EnterpriseMeta: *tc.route.GetEnterpriseMeta(),
			}
			actualDidUnbind := tc.listener.UnbindRoute(routeRef)
			require.Equal(t, tc.expectedDidUnbind, actualDidUnbind)
			require.Equal(t, tc.expectedListener.Routes, tc.listener.Routes)
		})
	}
}
