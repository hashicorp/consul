package middleware

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
)

type invalidAuthInfo struct{}

func (i invalidAuthInfo) AuthType() string {
	return "invalid."
}

func TestGRPCMiddleware_restrictPeeringEndpoints(t *testing.T) {

	tests := []struct {
		name       string
		authInfo   credentials.AuthInfo
		peeringSNI string
		endpoint   string
		expectErr  string
	}{
		{
			name:       "plaintext_always_allowed",
			authInfo:   nil,
			peeringSNI: "expected-server-sni",
			endpoint:   "/hashicorp.consul.internal.peerstream.PeerStreamService/SomeEndpoint",
		},
		{
			name:       "peering_not_enabled",
			authInfo:   nil,
			peeringSNI: "",
			endpoint:   "/hashicorp.consul.internal.peerstream.PeerStreamService/SomeEndpoint",
		},
		{
			name:       "deny_invalid_credentials",
			authInfo:   invalidAuthInfo{},
			peeringSNI: "expected-server-sni",
			expectErr:  "invalid transport credentials",
		},
		{
			name: "peering_sni_with_invalid_endpoint",
			authInfo: credentials.TLSInfo{
				State: tls.ConnectionState{
					ServerName: "peering-sni",
				},
			},
			peeringSNI: "peering-sni",
			endpoint:   "/some-invalid-endpoint",
			expectErr:  "invalid permissions to the specified endpoint",
		},
		{
			name: "peering_sni_with_valid_endpoint",
			authInfo: credentials.TLSInfo{
				State: tls.ConnectionState{
					ServerName: "peering-sni",
				},
			},
			peeringSNI: "peering-sni",
			endpoint:   "/hashicorp.consul.internal.peerstream.PeerStreamService/SomeEndpoint",
		},
		{
			name: "non_peering_sni_always_allowed",
			authInfo: credentials.TLSInfo{
				State: tls.ConnectionState{
					ServerName: "non-peering-sni",
				},
			},
			peeringSNI: "peering-sni",
			endpoint:   "/some-non-peering-endpoint",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := restrictPeeringEndpoints(tc.authInfo, tc.peeringSNI, tc.endpoint)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			}
		})
	}
}
