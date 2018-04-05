package connect

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestReloadableTLSConfig(t *testing.T) {
	require := require.New(t)
	base := defaultTLSConfig(nil)

	c := newReloadableTLSConfig(base)

	// The dynamic config should be the one we loaded (with some different hooks)
	got := c.TLSConfig()
	expect := base.Clone()
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
	expect = new.Clone()
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

func testCertPEMBlock(t *testing.T, pemValue string) []byte {
	t.Helper()
	// The _ result below is not an error but the remaining PEM bytes.
	block, _ := pem.Decode([]byte(pemValue))
	require.NotNil(t, block)
	require.Equal(t, "CERTIFICATE", block.Type)
	return block.Bytes
}

func TestClientSideVerifier(t *testing.T) {
	ca1 := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, ca1)

	webCA1PEM, _ := connect.TestLeaf(t, "web", ca1)
	webCA2PEM, _ := connect.TestLeaf(t, "web", ca2)

	webCA1 := testCertPEMBlock(t, webCA1PEM)
	xcCA2 := testCertPEMBlock(t, ca2.SigningCert)
	webCA2 := testCertPEMBlock(t, webCA2PEM)

	tests := []struct {
		name     string
		tlsCfg   *tls.Config
		rawCerts [][]byte
		wantErr  string
	}{
		{
			name:     "ok service ca1",
			tlsCfg:   TestTLSConfig(t, "web", ca1),
			rawCerts: [][]byte{webCA1},
			wantErr:  "",
		},
		{
			name:     "untrusted CA",
			tlsCfg:   TestTLSConfig(t, "web", ca2), // only trust ca2
			rawCerts: [][]byte{webCA1},             // present ca1
			wantErr:  "unknown authority",
		},
		{
			name:     "cross signed intermediate",
			tlsCfg:   TestTLSConfig(t, "web", ca1), // only trust ca1
			rawCerts: [][]byte{webCA2, xcCA2},      // present ca2 signed cert, and xc
			wantErr:  "",
		},
		{
			name:     "cross signed without intermediate",
			tlsCfg:   TestTLSConfig(t, "web", ca1), // only trust ca1
			rawCerts: [][]byte{webCA2},             // present ca2 signed cert only
			wantErr:  "unknown authority",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			err := clientSideVerifier(tt.tlsCfg, tt.rawCerts)
			if tt.wantErr == "" {
				require.Nil(err)
			} else {
				require.NotNil(err)
				require.Contains(err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServerSideVerifier(t *testing.T) {
	ca1 := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, ca1)

	webCA1PEM, _ := connect.TestLeaf(t, "web", ca1)
	webCA2PEM, _ := connect.TestLeaf(t, "web", ca2)

	apiCA1PEM, _ := connect.TestLeaf(t, "api", ca1)
	apiCA2PEM, _ := connect.TestLeaf(t, "api", ca2)

	webCA1 := testCertPEMBlock(t, webCA1PEM)
	xcCA2 := testCertPEMBlock(t, ca2.SigningCert)
	webCA2 := testCertPEMBlock(t, webCA2PEM)

	apiCA1 := testCertPEMBlock(t, apiCA1PEM)
	apiCA2 := testCertPEMBlock(t, apiCA2PEM)

	// Setup a local test agent to query
	agent := agent.NewTestAgent("test-consul", "")
	defer agent.Shutdown()

	cfg := api.DefaultConfig()
	cfg.Address = agent.HTTPAddr()
	client, err := api.NewClient(cfg)
	require.Nil(t, err)

	// Setup intentions to validate against. We actually default to allow so first
	// setup a blanket deny rule for db, then only allow web.
	connect := client.Connect()
	ixn := &api.Intention{
		SourceNS:        "default",
		SourceName:      "*",
		DestinationNS:   "default",
		DestinationName: "db",
		Action:          api.IntentionActionDeny,
		SourceType:      api.IntentionSourceConsul,
		Meta:            map[string]string{},
	}
	id, _, err := connect.IntentionCreate(ixn, nil)
	require.Nil(t, err)
	require.NotEmpty(t, id)

	ixn = &api.Intention{
		SourceNS:        "default",
		SourceName:      "web",
		DestinationNS:   "default",
		DestinationName: "db",
		Action:          api.IntentionActionAllow,
		SourceType:      api.IntentionSourceConsul,
		Meta:            map[string]string{},
	}
	id, _, err = connect.IntentionCreate(ixn, nil)
	require.Nil(t, err)
	require.NotEmpty(t, id)

	tests := []struct {
		name     string
		service  string
		tlsCfg   *tls.Config
		rawCerts [][]byte
		wantErr  string
	}{
		{
			name:     "ok service ca1, allow",
			service:  "db",
			tlsCfg:   TestTLSConfig(t, "db", ca1),
			rawCerts: [][]byte{webCA1},
			wantErr:  "",
		},
		{
			name:     "untrusted CA",
			service:  "db",
			tlsCfg:   TestTLSConfig(t, "db", ca2), // only trust ca2
			rawCerts: [][]byte{webCA1},            // present ca1
			wantErr:  "unknown authority",
		},
		{
			name:     "cross signed intermediate, allow",
			service:  "db",
			tlsCfg:   TestTLSConfig(t, "db", ca1), // only trust ca1
			rawCerts: [][]byte{webCA2, xcCA2},     // present ca2 signed cert, and xc
			wantErr:  "",
		},
		{
			name:     "cross signed without intermediate",
			service:  "db",
			tlsCfg:   TestTLSConfig(t, "db", ca1), // only trust ca1
			rawCerts: [][]byte{webCA2},            // present ca2 signed cert only
			wantErr:  "unknown authority",
		},
		{
			name:     "ok service ca1, deny",
			service:  "db",
			tlsCfg:   TestTLSConfig(t, "db", ca1),
			rawCerts: [][]byte{apiCA1},
			wantErr:  "denied",
		},
		{
			name:     "cross signed intermediate, deny",
			service:  "db",
			tlsCfg:   TestTLSConfig(t, "db", ca1), // only trust ca1
			rawCerts: [][]byte{apiCA2, xcCA2},     // present ca2 signed cert, and xc
			wantErr:  "denied",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newServerSideVerifier(client, tt.service)
			err := v(tt.tlsCfg, tt.rawCerts)
			if tt.wantErr == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
