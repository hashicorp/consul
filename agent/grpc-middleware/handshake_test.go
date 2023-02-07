package middleware

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
)

type fakeTransportCredentials struct {
	credentials.TransportCredentials
	callback func(conn net.Conn) (net.Conn, credentials.AuthInfo, error)
}

func (f fakeTransportCredentials) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return f.callback(conn)
}

func TestGRPCMiddleware_optionalTransportCredentials_ServerHandshake(t *testing.T) {

	plainConn := LabelledConn{protocol: ProtocolPlaintext}
	tlsConn := LabelledConn{protocol: ProtocolTLS}
	tests := []struct {
		name           string
		conn           net.Conn
		callback       func(conn net.Conn) (net.Conn, credentials.AuthInfo, error)
		expectConn     net.Conn
		expectAuthInfo credentials.AuthInfo
		expectErr      string
	}{
		{
			name:           "plaintext_noop",
			conn:           plainConn,
			expectConn:     plainConn,
			expectAuthInfo: nil,
		},
		{
			name: "tls_with_missing_auth",
			conn: tlsConn,
			callback: func(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
				return conn, nil, nil
			},
			expectConn:     nil,
			expectAuthInfo: nil,
			expectErr:      "missing auth info after handshake",
		},
		{
			name: "tls_success",
			conn: tlsConn,
			callback: func(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
				return conn, credentials.TLSInfo{}, nil
			},
			expectConn:     tlsConn,
			expectAuthInfo: credentials.TLSInfo{},
			expectErr:      "",
		},
		{
			name:           "invalid_protocol",
			conn:           LabelledConn{protocol: -1},
			expectConn:     nil,
			expectAuthInfo: nil,
			expectErr:      "invalid protocol for grpc connection",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			creds := optionalTransportCredentials{
				TransportCredentials: fakeTransportCredentials{
					callback: tc.callback,
				},
			}
			conn, authInfo, err := creds.ServerHandshake(tc.conn)
			require.Equal(t, tc.expectConn, conn)
			require.Equal(t, tc.expectAuthInfo, authInfo)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			}
		})
	}
}
