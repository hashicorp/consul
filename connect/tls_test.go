package connect

import (
	"crypto/x509"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/stretchr/testify/require"
)

func TestReloadableTLSConfig(t *testing.T) {
	require := require.New(t)
	verify, _ := testVerifier(t, nil)
	base := defaultTLSConfig(verify)

	c := NewReloadableTLSConfig(base)

	// The dynamic config should be the one we loaded (with some different hooks)
	got := c.TLSConfig()
	expect := *base
	// Equal and even cmp.Diff fail on tls.Config due to unexported fields in
	// each. Compare a few things to prove it's returning the bits we
	// specifically set.
	require.Equal(expect.Certificates, got.Certificates)
	require.Equal(expect.RootCAs, got.RootCAs)
	require.Equal(expect.ClientCAs, got.ClientCAs)
	require.Equal(expect.InsecureSkipVerify, got.InsecureSkipVerify)
	require.Equal(expect.MinVersion, got.MinVersion)
	require.Equal(expect.CipherSuites, got.CipherSuites)
	require.NotNil(got.GetClientCertificate)
	require.NotNil(got.GetConfigForClient)
	require.Contains(got.NextProtos, "h2")

	ca := connect.TestCA(t, nil)

	// Now change the config as if we just loaded certs from Consul
	new := TestTLSConfig(t, "web", ca)
	err := c.SetTLSConfig(new)
	require.Nil(err)

	// Change the passed config to ensure SetTLSConfig made a copy otherwise this
	// is racey.
	expect = *new
	new.Certificates = nil

	// The dynamic config should be the one we loaded (with some different hooks)
	got = c.TLSConfig()
	require.Equal(expect.Certificates, got.Certificates)
	require.Equal(expect.RootCAs, got.RootCAs)
	require.Equal(expect.ClientCAs, got.ClientCAs)
	require.Equal(expect.InsecureSkipVerify, got.InsecureSkipVerify)
	require.Equal(expect.MinVersion, got.MinVersion)
	require.Equal(expect.CipherSuites, got.CipherSuites)
	require.NotNil(got.GetClientCertificate)
	require.NotNil(got.GetConfigForClient)
	require.Contains(got.NextProtos, "h2")
}

func Test_verifyServerCertMatchesURI(t *testing.T) {
	ca1 := connect.TestCA(t, nil)

	tests := []struct {
		name     string
		certs    []*x509.Certificate
		expected connect.CertURI
		wantErr  bool
	}{
		{
			name:     "simple match",
			certs:    TestPeerCertificates(t, "web", ca1),
			expected: connect.TestSpiffeIDService(t, "web"),
			wantErr:  false,
		},
		{
			name:     "mismatch",
			certs:    TestPeerCertificates(t, "web", ca1),
			expected: connect.TestSpiffeIDService(t, "db"),
			wantErr:  true,
		},
		{
			name:     "no certs",
			certs:    []*x509.Certificate{},
			expected: connect.TestSpiffeIDService(t, "db"),
			wantErr:  true,
		},
		{
			name:     "nil certs",
			certs:    nil,
			expected: connect.TestSpiffeIDService(t, "db"),
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyServerCertMatchesURI(tt.certs, tt.expected)
			if tt.wantErr {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
