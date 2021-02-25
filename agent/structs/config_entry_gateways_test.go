package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIngressConfigEntry_Normalize(t *testing.T) {

	cases := []struct {
		name     string
		entry    IngressGatewayConfigEntry
		expected IngressGatewayConfigEntry
	}{
		{
			name: "empty protocol",
			entry: IngressGatewayConfigEntry{
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
			expected: IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{},
					},
				},
				EnterpriseMeta: *DefaultEnterpriseMeta(),
			},
		},
		{
			name: "lowercase protocols",
			entry: IngressGatewayConfigEntry{
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
			expected: IngressGatewayConfigEntry{
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
				EnterpriseMeta: *DefaultEnterpriseMeta(),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			require.NoError(t, err)
			require.Equal(t, tc.expected, tc.entry)
		})
	}
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

func TestIngressConfigEntry_Validate(t *testing.T) {

	cases := []struct {
		name      string
		entry     IngressGatewayConfigEntry
		expectErr string
	}{
		{
			name: "port conflict",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "port 1111 declared on two listeners",
		},
		{
			name: "http features: wildcard",
			entry: IngressGatewayConfigEntry{
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
		{
			name: "http features: wildcard service on invalid protocol",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "Wildcard service name is only valid for protocol",
		},
		{
			name: "http features: multiple services",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "multiple services per listener are only supported for protocol",
		},
		{
			name: "tcp listener requires a defined service",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "no service declared for listener with port 1111",
		},
		{
			name: "http listener requires a defined service",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "no service declared for listener with port 1111",
		},
		{
			name: "empty service name not supported",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "Service name cannot be blank",
		},
		{
			name: "protocol validation",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "protocol must be 'tcp', 'http', 'http2', or 'grpc'. 'asdf' is an unsupported protocol",
		},
		{
			name: "hosts cannot be set on a tcp listener",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "Associating hosts to a service is not supported for the tcp protocol",
		},
		{
			name: "hosts cannot be set on a wildcard specifier",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "Associating hosts to a wildcard service is not supported",
		},
		{
			name: "hosts must be unique per listener",
			entry: IngressGatewayConfigEntry{
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
			expectErr: "Hosts must be unique within a specific listener",
		},
		{
			name: "hosts must be a valid DNS name",
			entry: IngressGatewayConfigEntry{
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
			expectErr: `Host "example..com" must be a valid DNS hostname`,
		},
		{
			name: "wildcard specifier is only allowed in the leftmost label",
			entry: IngressGatewayConfigEntry{
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
		{
			name: "wildcard specifier is not allowed in non-leftmost labels",
			entry: IngressGatewayConfigEntry{
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
			expectErr: `Host "example.*.com" is not valid, a wildcard specifier is only allowed as the leftmost label`,
		},
		{
			name: "wildcard specifier is not allowed in leftmost labels as a partial",
			entry: IngressGatewayConfigEntry{
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
			expectErr: `Host "*-test.example.com" is not valid, a wildcard specifier is only allowed as the leftmost label`,
		},
		{
			name: "wildcard specifier is allowed for hosts when TLS is disabled",
			entry: IngressGatewayConfigEntry{
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
		{
			name: "wildcard specifier is not allowed for hosts when TLS is enabled",
			entry: IngressGatewayConfigEntry{
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
			expectErr: `Host '*' is not allowed when TLS is enabled, all hosts must be valid DNS records to add as a DNSSAN`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expectErr != "" {
				require.Error(t, err)
				requireContainsLower(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTerminatingConfigEntry_Validate(t *testing.T) {

	cases := []struct {
		name      string
		entry     TerminatingGatewayConfigEntry
		expectErr string
	}{
		{
			name: "service conflict",
			entry: TerminatingGatewayConfigEntry{
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
			expectErr: "specified more than once",
		},
		{
			name: "blank service name",
			entry: TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name: "",
					},
				},
			},
			expectErr: "Service name cannot be blank.",
		},
		{
			name: "not all TLS options provided-1",
			entry: TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name:     "web",
						CertFile: "client.crt",
					},
				},
			},
			expectErr: "must have a CertFile, CAFile, and KeyFile",
		},
		{
			name: "not all TLS options provided-2",
			entry: TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name:    "web",
						KeyFile: "tls.key",
					},
				},
			},
			expectErr: "must have a CertFile, CAFile, and KeyFile",
		},
		{
			name: "all TLS options provided",
			entry: TerminatingGatewayConfigEntry{
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
		{
			name: "only providing ca file is allowed",
			entry: TerminatingGatewayConfigEntry{
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

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			err := tc.entry.Validate()
			if tc.expectErr != "" {
				require.Error(t, err)
				requireContainsLower(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
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
