package connect

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

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
			// Could happen during migration of secondary DC to multi-DC. Trust domain
			// validity is enforced with x509 name constraints where needed.
			name:     "different trust-domain allowed",
			certs:    TestPeerCertificates(t, "web", ca1),
			expected: connect.TestSpiffeIDServiceWithHost(t, "web", "other.consul"),
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
			err := clientSideVerifier(tt.tlsCfg, tt.rawCerts)
			if tt.wantErr == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServerSideVerifier(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	agent := agent.StartTestAgent(t, agent.TestAgent{Name: "test-consul"})
	defer agent.Shutdown()
	testrpc.WaitForTestAgent(t, agent.RPC, "dc1")

	cfg := api.DefaultConfig()
	cfg.Address = agent.HTTPAddr()
	client, err := api.NewClient(cfg)
	require.NoError(t, err)

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
	//nolint:staticcheck
	id, _, err := connect.IntentionCreate(ixn, nil)
	require.NoError(t, err)
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
	//nolint:staticcheck
	id, _, err = connect.IntentionCreate(ixn, nil)
	require.NoError(t, err)
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
			v := newServerSideVerifier(testutil.Logger(t), client, tt.service)
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

// requireEqualTLSConfig compares tlsConfig fields we care about. Equal and even
// cmp.Diff fail on tls.Config due to unexported fields in each. expectLeaf
// allows expecting a leaf cert different from the one in expect
func requireEqualTLSConfig(t *testing.T, expect, got *tls.Config) {
	require.Equal(t, expect.RootCAs, got.RootCAs)
	prototest.AssertDeepEqual(t, expect.ClientCAs, got.ClientCAs, cmpCertPool)
	require.Equal(t, expect.InsecureSkipVerify, got.InsecureSkipVerify)
	require.Equal(t, expect.MinVersion, got.MinVersion)
	require.Equal(t, expect.CipherSuites, got.CipherSuites)
	require.NotNil(t, got.GetCertificate)
	require.NotNil(t, got.GetClientCertificate)
	require.NotNil(t, got.GetConfigForClient)
	require.Contains(t, got.NextProtos, "h2")

	var expectLeaf *tls.Certificate
	var err error
	if expect.GetCertificate != nil {
		expectLeaf, err = expect.GetCertificate(nil)
		require.Nil(t, err)
	} else if len(expect.Certificates) > 0 {
		expectLeaf = &expect.Certificates[0]
	}

	gotLeaf, err := got.GetCertificate(nil)
	require.Nil(t, err)
	require.Equal(t, expectLeaf, gotLeaf)

	gotLeaf, err = got.GetClientCertificate(nil)
	require.Nil(t, err)
	require.Equal(t, expectLeaf, gotLeaf)
}

// cmpCertPool is a custom comparison for x509.CertPool, because CertPool.lazyCerts
// has a func field which can't be compared.
// lazyCerts has a func field which can't be compared.
var cmpCertPool = cmp.Options{
	cmpopts.IgnoreFields(x509.CertPool{}, "lazyCerts"),
	cmp.AllowUnexported(x509.CertPool{}),
}

// requireCorrectVerifier invokes got.VerifyPeerCertificate and expects the
// tls.Config arg to be returned on the provided channel. This ensures the
// correct verifier func was attached to got.
//
// It then ensures that the tls.Config passed to the verifierFunc was actually
// the same as the expected current value.
func requireCorrectVerifier(t *testing.T, expect, got *tls.Config,
	ch chan *tls.Config) {

	err := got.VerifyPeerCertificate(nil, nil)
	require.Nil(t, err)
	verifierCfg := <-ch
	// The tls.Cfg passed to verifyFunc should be the expected (current) value.
	requireEqualTLSConfig(t, expect, verifierCfg)
}

func TestDynamicTLSConfig(t *testing.T) {

	ca1 := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, nil)
	baseCfg := TestTLSConfig(t, "web", ca1)
	newCfg := TestTLSConfig(t, "web", ca2)

	c := newDynamicTLSConfig(baseCfg, nil)

	// Should set them from the base config
	require.Equal(t, c.Leaf(), &baseCfg.Certificates[0])
	require.Equal(t, c.Roots(), baseCfg.RootCAs)

	// Create verifiers we can assert are set and run correctly.
	v1Ch := make(chan *tls.Config, 1)
	v2Ch := make(chan *tls.Config, 1)
	v3Ch := make(chan *tls.Config, 1)
	verify1 := func(cfg *tls.Config, rawCerts [][]byte) error {
		v1Ch <- cfg
		return nil
	}
	verify2 := func(cfg *tls.Config, rawCerts [][]byte) error {
		v2Ch <- cfg
		return nil
	}
	verify3 := func(cfg *tls.Config, rawCerts [][]byte) error {
		v3Ch <- cfg
		return nil
	}

	// The dynamic config should be the one we loaded (with some different hooks)
	gotBefore := c.Get(verify1)
	requireEqualTLSConfig(t, baseCfg, gotBefore)
	requireCorrectVerifier(t, baseCfg, gotBefore, v1Ch)

	// Now change the roots as if we just loaded new roots from Consul
	err := c.SetRoots(newCfg.RootCAs)
	require.Nil(t, err)

	// The dynamic config should have the new roots, but old leaf
	gotAfter := c.Get(verify2)
	expect := newCfg.Clone()
	expect.GetCertificate = func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return &baseCfg.Certificates[0], nil
	}
	requireEqualTLSConfig(t, expect, gotAfter)
	requireCorrectVerifier(t, expect, gotAfter, v2Ch)

	// The old config fetched before should still call it's own verify func, but
	// that verifier should be passed the new config (expect).
	requireCorrectVerifier(t, expect, gotBefore, v1Ch)

	// Now change the leaf
	err = c.SetLeaf(&newCfg.Certificates[0])
	require.Nil(t, err)

	// The dynamic config should have the new roots, AND new leaf
	gotAfterLeaf := c.Get(verify3)
	requireEqualTLSConfig(t, newCfg, gotAfterLeaf)
	requireCorrectVerifier(t, newCfg, gotAfterLeaf, v3Ch)

	// Both older configs should still call their own verify funcs, but those
	// verifiers should be passed the new config.
	requireCorrectVerifier(t, newCfg, gotBefore, v1Ch)
	requireCorrectVerifier(t, newCfg, gotAfter, v2Ch)
}

func TestDynamicTLSConfig_Ready(t *testing.T) {

	ca1 := connect.TestCA(t, nil)
	baseCfg := TestTLSConfig(t, "web", ca1)

	c := newDynamicTLSConfig(defaultTLSConfig(), nil)
	readyCh := c.ReadyWait()
	assertBlocked(t, readyCh)
	require.False(t, c.Ready(), "no roots or leaf, should not be ready")

	err := c.SetLeaf(&baseCfg.Certificates[0])
	require.NoError(t, err)
	assertBlocked(t, readyCh)
	require.False(t, c.Ready(), "no roots, should not be ready")

	err = c.SetRoots(baseCfg.RootCAs)
	require.NoError(t, err)
	assertNotBlocked(t, readyCh)
	require.True(t, c.Ready(), "should be ready")

	ca2 := connect.TestCA(t, nil)
	ca2cfg := TestTLSConfig(t, "web", ca2)

	require.NoError(t, c.SetRoots(ca2cfg.RootCAs))
	assertNotBlocked(t, readyCh)
	require.False(t, c.Ready(), "invalid leaf, should not be ready")

	require.NoError(t, c.SetRoots(baseCfg.RootCAs))
	assertNotBlocked(t, readyCh)
	require.True(t, c.Ready(), "should be ready")
}

func assertBlocked(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatalf("want blocked chan")
	default:
		return
	}
}

func assertNotBlocked(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		return
	default:
		t.Fatalf("want unblocked chan but it blocked")
	}
}
