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
			validateErr: "Multiple services per listener are only supported for protocol",
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
			// Unchanged
			expected: &IngressGatewayConfigEntry{
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
