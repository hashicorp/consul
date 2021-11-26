package ca

import (
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

type consulCAMockDelegate struct {
	state *state.Store
}

func (c *consulCAMockDelegate) ProviderState(id string) (*structs.CAConsulProviderState, error) {
	_, s, err := c.state.CAProviderState(id)
	return s, err
}

func (c *consulCAMockDelegate) ApplyCARequest(req *structs.CARequest) (interface{}, error) {
	return ApplyCARequestToStore(c.state, req)
}

func newMockDelegate(t *testing.T, conf *structs.CAConfiguration) *consulCAMockDelegate {
	s := state.NewStateStore(nil)
	if s == nil {
		t.Fatalf("missing state store")
	}
	if err := s.CASetConfig(conf.RaftIndex.CreateIndex, conf); err != nil {
		t.Fatalf("err: %s", err)
	}

	return &consulCAMockDelegate{s}
}

func testConsulCAConfig() *structs.CAConfiguration {
	return &structs.CAConfiguration{
		ClusterID: connect.TestClusterID,
		Provider:  "consul",
		Config: map[string]interface{}{
			// Tests duration parsing after msgpack type mangling during raft apply.
			"LeafCertTTL":         []byte("72h"),
			"IntermediateCertTTL": []byte("288h"),
			"RootCertTTL":         []byte("87600h"),
		},
	}
}

func testProviderConfig(caCfg *structs.CAConfiguration) ProviderConfig {
	return ProviderConfig{
		ClusterID:  caCfg.ClusterID,
		Datacenter: "dc1",
		IsPrimary:  true,
		RawConfig:  caCfg.Config,
	}
}

func requireNotEncoded(t *testing.T, v []byte) {
	t.Helper()
	require.False(t, connect.IsHexString(v))
}

func TestConsulCAProvider_Bootstrap(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	conf := testConsulCAConfig()
	delegate := newMockDelegate(t, conf)

	provider := TestConsulProvider(t, delegate)
	require.NoError(provider.Configure(testProviderConfig(conf)))
	require.NoError(provider.GenerateRoot())

	root, err := provider.ActiveRoot()
	require.NoError(err)

	// Intermediate should be the same cert.
	inter, err := provider.ActiveIntermediate()
	require.NoError(err)
	require.Equal(root, inter)

	// Should be a valid cert
	parsed, err := connect.ParseCert(root)
	require.NoError(err)
	require.Equal(parsed.URIs[0].String(), fmt.Sprintf("spiffe://%s.consul", conf.ClusterID))
	requireNotEncoded(t, parsed.SubjectKeyId)
	requireNotEncoded(t, parsed.AuthorityKeyId)

	// test that the root cert ttl is the same as the expected value
	// notice that we allow a margin of "error" of 10 minutes between the
	// generateCA() creation and this check
	defaultRootCertTTL, err := time.ParseDuration(structs.DefaultRootCertTTL)
	require.NoError(err)
	expectedNotAfter := time.Now().Add(defaultRootCertTTL).UTC()
	require.WithinDuration(expectedNotAfter, parsed.NotAfter, 10*time.Minute, "expected parsed cert ttl to be the same as the value configured")
}

func TestConsulCAProvider_Bootstrap_WithCert(t *testing.T) {
	t.Parallel()

	// Make sure setting a custom private key/root cert works.
	require := require.New(t)
	rootCA := connect.TestCAWithTTL(t, nil, 5*time.Hour)
	conf := testConsulCAConfig()
	conf.Config = map[string]interface{}{
		"PrivateKey": rootCA.SigningKey,
		"RootCert":   rootCA.RootCert,
	}
	delegate := newMockDelegate(t, conf)

	provider := TestConsulProvider(t, delegate)
	require.NoError(provider.Configure(testProviderConfig(conf)))
	require.NoError(provider.GenerateRoot())

	root, err := provider.ActiveRoot()
	require.NoError(err)
	require.Equal(root, rootCA.RootCert)

	// Should be a valid cert
	parsed, err := connect.ParseCert(root)
	require.NoError(err)

	// test that the default root cert ttl was not applied to the provided cert
	defaultRootCertTTL, err := time.ParseDuration(structs.DefaultRootCertTTL)
	require.NoError(err)
	defaultNotAfter := time.Now().Add(defaultRootCertTTL).UTC()
	// we can't compare given the "delta" between the time the cert is generated
	// and when we start the test; so just look at the years for now, given different years
	require.NotEqualf(defaultNotAfter.Year(), parsed.NotAfter.Year(), "parsed cert ttl expected to be different from default root cert ttl")
}

func TestConsulCAProvider_SignLeaf(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	for _, tc := range KeyTestCases {
		tc := tc
		t.Run(tc.Desc, func(t *testing.T) {
			require := require.New(t)
			conf := testConsulCAConfig()
			conf.Config["LeafCertTTL"] = "1h"
			conf.Config["PrivateKeyType"] = tc.KeyType
			conf.Config["PrivateKeyBits"] = tc.KeyBits
			delegate := newMockDelegate(t, conf)

			provider := TestConsulProvider(t, delegate)
			require.NoError(provider.Configure(testProviderConfig(conf)))
			require.NoError(provider.GenerateRoot())

			spiffeService := &connect.SpiffeIDService{
				Host:       connect.TestClusterID + ".consul",
				Namespace:  "default",
				Datacenter: "dc1",
				Service:    "foo",
			}

			// Generate a leaf cert for the service.
			{
				raw, _ := connect.TestCSR(t, spiffeService)

				csr, err := connect.ParseCSR(raw)
				require.NoError(err)

				cert, err := provider.Sign(csr)
				require.NoError(err)
				requireTrailingNewline(t, cert)
				parsed, err := connect.ParseCert(cert)
				require.NoError(err)
				require.Equal(spiffeService.URI(), parsed.URIs[0])
				require.Empty(parsed.Subject.CommonName)
				require.Equal(uint64(2), parsed.SerialNumber.Uint64())
				subjectKeyID, err := connect.KeyId(csr.PublicKey)
				require.NoError(err)
				require.Equal(subjectKeyID, parsed.SubjectKeyId)
				requireNotEncoded(t, parsed.SubjectKeyId)
				requireNotEncoded(t, parsed.AuthorityKeyId)

				// Ensure the cert is valid now and expires within the correct limit.
				now := time.Now()
				require.True(parsed.NotAfter.Sub(now) < time.Hour)
				require.True(parsed.NotBefore.Before(now))
			}

			// Generate a new cert for another service and make sure
			// the serial number is incremented.
			spiffeService.Service = "bar"
			{
				raw, _ := connect.TestCSR(t, spiffeService)

				csr, err := connect.ParseCSR(raw)
				require.NoError(err)

				cert, err := provider.Sign(csr)
				require.NoError(err)

				parsed, err := connect.ParseCert(cert)
				require.NoError(err)
				require.Equal(spiffeService.URI(), parsed.URIs[0])
				require.Empty(parsed.Subject.CommonName)
				require.Equal(parsed.SerialNumber.Uint64(), uint64(2))
				requireNotEncoded(t, parsed.SubjectKeyId)
				requireNotEncoded(t, parsed.AuthorityKeyId)

				// Ensure the cert is valid now and expires within the correct limit.
				require.True(time.Until(parsed.NotAfter) < 3*24*time.Hour)
				require.True(parsed.NotBefore.Before(time.Now()))
			}

			spiffeAgent := &connect.SpiffeIDAgent{
				Host:       connect.TestClusterID + ".consul",
				Datacenter: "dc1",
				Agent:      "uuid",
			}
			// Generate a leaf cert for an agent.
			{
				raw, _ := connect.TestCSR(t, spiffeAgent)

				csr, err := connect.ParseCSR(raw)
				require.NoError(err)

				cert, err := provider.Sign(csr)
				require.NoError(err)

				parsed, err := connect.ParseCert(cert)
				require.NoError(err)
				require.Equal(spiffeAgent.URI(), parsed.URIs[0])
				require.Empty(parsed.Subject.CommonName)
				require.Equal(uint64(2), parsed.SerialNumber.Uint64())
				requireNotEncoded(t, parsed.SubjectKeyId)
				requireNotEncoded(t, parsed.AuthorityKeyId)

				// Ensure the cert is valid now and expires within the correct limit.
				now := time.Now()
				require.True(parsed.NotAfter.Sub(now) < time.Hour)
				require.True(parsed.NotBefore.Before(now))
			}
		})
	}
}

func TestConsulCAProvider_CrossSignCA(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	tests := CASigningKeyTypeCases()

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Desc, func(t *testing.T) {
			require := require.New(t)

			conf1 := testConsulCAConfig()
			delegate1 := newMockDelegate(t, conf1)
			provider1 := TestConsulProvider(t, delegate1)
			conf1.Config["PrivateKeyType"] = tc.SigningKeyType
			conf1.Config["PrivateKeyBits"] = tc.SigningKeyBits
			require.NoError(provider1.Configure(testProviderConfig(conf1)))
			require.NoError(provider1.GenerateRoot())

			conf2 := testConsulCAConfig()
			conf2.CreateIndex = 10
			delegate2 := newMockDelegate(t, conf2)
			provider2 := TestConsulProvider(t, delegate2)
			conf2.Config["PrivateKeyType"] = tc.CSRKeyType
			conf2.Config["PrivateKeyBits"] = tc.CSRKeyBits
			require.NoError(provider2.Configure(testProviderConfig(conf2)))
			require.NoError(provider2.GenerateRoot())

			testCrossSignProviders(t, provider1, provider2)
		})
	}
}

func testCrossSignProviders(t *testing.T, provider1, provider2 Provider) {
	require := require.New(t)

	// Get the root from the new provider to be cross-signed.
	newRootPEM, err := provider2.ActiveRoot()
	require.NoError(err)
	newRoot, err := connect.ParseCert(newRootPEM)
	require.NoError(err)
	oldSubject := newRoot.Subject.CommonName
	requireNotEncoded(t, newRoot.SubjectKeyId)
	requireNotEncoded(t, newRoot.AuthorityKeyId)

	newInterPEM, err := provider2.ActiveIntermediate()
	require.NoError(err)
	newIntermediate, err := connect.ParseCert(newInterPEM)
	require.NoError(err)
	requireNotEncoded(t, newIntermediate.SubjectKeyId)
	requireNotEncoded(t, newIntermediate.AuthorityKeyId)

	// Have provider1 cross sign our new root cert.
	xcPEM, err := provider1.CrossSignCA(newRoot)
	require.NoError(err)
	xc, err := connect.ParseCert(xcPEM)
	require.NoError(err)
	requireNotEncoded(t, xc.SubjectKeyId)
	requireNotEncoded(t, xc.AuthorityKeyId)

	oldRootPEM, err := provider1.ActiveRoot()
	require.NoError(err)
	oldRoot, err := connect.ParseCert(oldRootPEM)
	require.NoError(err)
	requireNotEncoded(t, oldRoot.SubjectKeyId)
	requireNotEncoded(t, oldRoot.AuthorityKeyId)

	// AuthorityKeyID should now be the signing root's, SubjectKeyId should be kept.
	require.Equal(oldRoot.SubjectKeyId, xc.AuthorityKeyId,
		"newSKID=%x\nnewAKID=%x\noldSKID=%x\noldAKID=%x\nxcSKID=%x\nxcAKID=%x",
		newRoot.SubjectKeyId, newRoot.AuthorityKeyId,
		oldRoot.SubjectKeyId, oldRoot.AuthorityKeyId,
		xc.SubjectKeyId, xc.AuthorityKeyId)
	require.Equal(newRoot.SubjectKeyId, xc.SubjectKeyId)

	// Subject name should not have changed.
	require.Equal(oldSubject, xc.Subject.CommonName)

	// Issuer should be the signing root.
	require.Equal(oldRoot.Issuer.CommonName, xc.Issuer.CommonName)

	// Get a leaf cert so we can verify against the cross-signed cert.
	spiffeService := &connect.SpiffeIDService{
		Host:       connect.TestClusterID + ".consul",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	}
	raw, _ := connect.TestCSR(t, spiffeService)

	leafCsr, err := connect.ParseCSR(raw)
	require.NoError(err)

	leafPEM, err := provider2.Sign(leafCsr)
	require.NoError(err)

	cert, err := connect.ParseCert(leafPEM)
	require.NoError(err)
	requireNotEncoded(t, cert.SubjectKeyId)
	requireNotEncoded(t, cert.AuthorityKeyId)

	// Check that the leaf signed by the new cert can be verified by either root
	// certificate by using the new intermediate + cross-signed cert.
	intermediatePool := x509.NewCertPool()
	intermediatePool.AddCert(newIntermediate)
	intermediatePool.AddCert(xc)

	for _, root := range []*x509.Certificate{oldRoot, newRoot} {
		rootPool := x509.NewCertPool()
		rootPool.AddCert(root)

		_, err = cert.Verify(x509.VerifyOptions{
			Intermediates: intermediatePool,
			Roots:         rootPool,
		})
		require.NoError(err)
	}
}

func TestConsulProvider_SignIntermediate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	tests := CASigningKeyTypeCases()

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Desc, func(t *testing.T) {
			require := require.New(t)

			conf1 := testConsulCAConfig()
			delegate1 := newMockDelegate(t, conf1)
			provider1 := TestConsulProvider(t, delegate1)
			conf1.Config["PrivateKeyType"] = tc.SigningKeyType
			conf1.Config["PrivateKeyBits"] = tc.SigningKeyBits
			require.NoError(provider1.Configure(testProviderConfig(conf1)))
			require.NoError(provider1.GenerateRoot())

			conf2 := testConsulCAConfig()
			conf2.CreateIndex = 10
			delegate2 := newMockDelegate(t, conf2)
			provider2 := TestConsulProvider(t, delegate2)
			conf2.Config["PrivateKeyType"] = tc.CSRKeyType
			conf2.Config["PrivateKeyBits"] = tc.CSRKeyBits
			cfg := testProviderConfig(conf2)
			cfg.IsPrimary = false
			cfg.Datacenter = "dc2"
			require.NoError(provider2.Configure(cfg))

			testSignIntermediateCrossDC(t, provider1, provider2)
		})
	}

}

func testSignIntermediateCrossDC(t *testing.T, provider1, provider2 Provider) {
	require := require.New(t)

	// Get the intermediate CSR from provider2.
	csrPEM, err := provider2.GenerateIntermediateCSR()
	require.NoError(err)
	csr, err := connect.ParseCSR(csrPEM)
	require.NoError(err)

	// Sign the CSR with provider1.
	intermediatePEM, err := provider1.SignIntermediate(csr)
	require.NoError(err)
	rootPEM, err := provider1.ActiveRoot()
	require.NoError(err)

	// Give the new intermediate to provider2 to use.
	require.NoError(provider2.SetIntermediate(intermediatePEM, rootPEM))

	// Have provider2 sign a leaf cert and make sure the chain is correct.
	spiffeService := &connect.SpiffeIDService{
		Host:       connect.TestClusterID + ".consul",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	}
	raw, _ := connect.TestCSR(t, spiffeService)

	leafCsr, err := connect.ParseCSR(raw)
	require.NoError(err)

	leafPEM, err := provider2.Sign(leafCsr)
	require.NoError(err)

	cert, err := connect.ParseCert(leafPEM)
	require.NoError(err)
	requireNotEncoded(t, cert.SubjectKeyId)
	requireNotEncoded(t, cert.AuthorityKeyId)

	// Check that the leaf signed by the new cert can be verified using the
	// returned cert chain (signed intermediate + remote root).
	intermediatePool := x509.NewCertPool()
	intermediatePool.AppendCertsFromPEM([]byte(intermediatePEM))
	rootPool := x509.NewCertPool()
	rootPool.AppendCertsFromPEM([]byte(rootPEM))

	_, err = cert.Verify(x509.VerifyOptions{
		Intermediates: intermediatePool,
		Roots:         rootPool,
	})
	require.NoError(err)
}

func TestConsulCAProvider_MigrateOldID(t *testing.T) {
	cases := []struct {
		name  string
		oldID string
	}{
		{
			name:  "original-unhashed",
			oldID: ",",
		},
		{
			name:  "hash-v1",
			oldID: hexStringHash(",,true"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conf := testConsulCAConfig()
			delegate := newMockDelegate(t, conf)

			// Create an entry with an old-style ID.
			_, err := delegate.ApplyCARequest(&structs.CARequest{
				Op: structs.CAOpSetProviderState,
				ProviderState: &structs.CAConsulProviderState{
					ID: tc.oldID,
				},
			})
			require.NoError(t, err)
			_, providerState, err := delegate.state.CAProviderState(tc.oldID)
			require.NoError(t, err)
			require.NotNil(t, providerState)

			provider := TestConsulProvider(t, delegate)
			require.NoError(t, provider.Configure(testProviderConfig(conf)))
			require.NoError(t, provider.GenerateRoot())

			// After running Configure, the old ID entry should be gone.
			_, providerState, err = delegate.state.CAProviderState(tc.oldID)
			require.NoError(t, err)
			require.Nil(t, providerState)
		})
	}
}
