package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIngressConfigEntry_Normalize(t *testing.T) {
	t.Parallel()

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

	for _, test := range cases {
		// We explicitly copy the variable for the range statement so that can run
		// tests in parallel.
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.entry.Normalize()
			require.NoError(t, err)
			require.Equal(t, tc.expected, tc.entry)
		})
	}
}

func TestIngressConfigEntry_Validate(t *testing.T) {
	t.Parallel()

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
			expectErr: "Protocol must be either 'http' or 'tcp', 'asdf' is an unsupported protocol.",
		},
	}

	for _, test := range cases {
		// We explicitly copy the variable for the range statement so that can run
		// tests in parallel.
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()

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
						Name:   "web",
						CAFile: "ca.crt",
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
						Name:     "web",
						CertFile: "client.crt",
					},
				},
			},
			expectErr: "must have a CertFile, CAFile, and KeyFile",
		},
		{
			name: "not all TLS options provided-3",
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
			name: "not all TLS options provided-4",
			entry: TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name:    "web",
						CAFile:  "ca.crt",
						KeyFile: "tls.key",
					},
				},
			},
			expectErr: "must have a CertFile, CAFile, and KeyFile",
		},
		{
			name: "not all TLS options provided-5",
			entry: TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name:     "web",
						CAFile:   "ca.crt",
						CertFile: "client.crt",
					},
				},
			},
			expectErr: "must have a CertFile, CAFile, and KeyFile",
		},
		{
			name: "not all TLS options provided-6",
			entry: TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Services: []LinkedService{
					{
						Name:     "web",
						KeyFile:  "tls.key",
						CertFile: "client.crt",
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
	}

	for _, test := range cases {
		// We explicitly copy the variable for the range statement so that can run
		// tests in parallel.
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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
