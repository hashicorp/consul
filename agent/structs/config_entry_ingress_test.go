package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
			name: "wildcard namespace not supported",
			entry: IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Listeners: []IngressListener{
					{
						Port:     1111,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name:      "foo",
								Namespace: "*",
							},
						},
					},
				},
			},
			expectErr: "Wildcard namespace is not supported for ingress services",
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
