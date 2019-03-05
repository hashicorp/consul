package ca

import (
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

type consulCAMockDelegate struct {
	state *state.Store
}

func (c *consulCAMockDelegate) State() *state.Store {
	return c.state
}

func (c *consulCAMockDelegate) ApplyCARequest(req *structs.CARequest) error {
	idx, _, err := c.state.CAConfig()
	if err != nil {
		return err
	}

	switch req.Op {
	case structs.CAOpSetProviderState:
		_, err := c.state.CASetProviderState(idx+1, req.ProviderState)
		if err != nil {
			return err
		}

		return nil
	case structs.CAOpDeleteProviderState:
		if err := c.state.CADeleteProviderState(req.ProviderState.ID); err != nil {
			return err
		}

		return nil
	default:
		return fmt.Errorf("Invalid CA operation '%s'", req.Op)
	}
}

func newMockDelegate(t *testing.T, conf *structs.CAConfiguration) *consulCAMockDelegate {
	s, err := state.NewStateStore(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
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
		ClusterID: "asdf",
		Provider:  "consul",
		Config: map[string]interface{}{
			// Tests duration parsing after msgpack type mangling during raft apply.
			"LeafCertTTL": []uint8("72h"),
		},
	}
}

func TestConsulCAProvider_Bootstrap(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	conf := testConsulCAConfig()
	delegate := newMockDelegate(t, conf)

	provider := &ConsulProvider{Delegate: delegate}
	require.NoError(provider.Configure(conf.ClusterID, true, conf.Config))
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
}

func TestConsulCAProvider_Bootstrap_WithCert(t *testing.T) {
	t.Parallel()

	// Make sure setting a custom private key/root cert works.
	require := require.New(t)
	rootCA := connect.TestCA(t, nil)
	conf := testConsulCAConfig()
	conf.Config = map[string]interface{}{
		"PrivateKey": rootCA.SigningKey,
		"RootCert":   rootCA.RootCert,
	}
	delegate := newMockDelegate(t, conf)

	provider := &ConsulProvider{Delegate: delegate}
	require.NoError(provider.Configure(conf.ClusterID, true, conf.Config))
	require.NoError(provider.GenerateRoot())

	root, err := provider.ActiveRoot()
	require.NoError(err)
	require.Equal(root, rootCA.RootCert)
}

func TestConsulCAProvider_SignLeaf(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	conf := testConsulCAConfig()
	conf.Config["LeafCertTTL"] = "1h"
	delegate := newMockDelegate(t, conf)

	provider := &ConsulProvider{Delegate: delegate}
	require.NoError(provider.Configure(conf.ClusterID, true, conf.Config))
	require.NoError(provider.GenerateRoot())

	spiffeService := &connect.SpiffeIDService{
		Host:       "node1",
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

		parsed, err := connect.ParseCert(cert)
		require.NoError(err)
		require.Equal(parsed.URIs[0], spiffeService.URI())
		require.Equal(parsed.Subject.CommonName, "foo")
		require.Equal(uint64(2), parsed.SerialNumber.Uint64())

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
		require.Equal(parsed.URIs[0], spiffeService.URI())
		require.Equal(parsed.Subject.CommonName, "bar")
		require.Equal(parsed.SerialNumber.Uint64(), uint64(2))

		// Ensure the cert is valid now and expires within the correct limit.
		require.True(parsed.NotAfter.Sub(time.Now()) < 3*24*time.Hour)
		require.True(parsed.NotBefore.Before(time.Now()))
	}
}

func TestConsulCAProvider_CrossSignCA(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	conf1 := testConsulCAConfig()
	delegate1 := newMockDelegate(t, conf1)
	provider1 := &ConsulProvider{Delegate: delegate1}
	require.NoError(provider1.Configure(conf1.ClusterID, true, conf1.Config))
	require.NoError(provider1.GenerateRoot())

	conf2 := testConsulCAConfig()
	conf2.CreateIndex = 10
	delegate2 := newMockDelegate(t, conf2)
	provider2 := &ConsulProvider{Delegate: delegate2}
	require.NoError(provider2.Configure(conf2.ClusterID, true, conf2.Config))
	require.NoError(provider2.GenerateRoot())

	testCrossSignProviders(t, provider1, provider2)
}

func testCrossSignProviders(t *testing.T, provider1, provider2 Provider) {
	require := require.New(t)

	// Get the root from the new provider to be cross-signed.
	newRootPEM, err := provider2.ActiveRoot()
	require.NoError(err)
	newRoot, err := connect.ParseCert(newRootPEM)
	require.NoError(err)
	oldSubject := newRoot.Subject.CommonName

	newInterPEM, err := provider2.ActiveIntermediate()
	require.NoError(err)
	newIntermediate, err := connect.ParseCert(newInterPEM)
	require.NoError(err)

	// Have provider1 cross sign our new root cert.
	xcPEM, err := provider1.CrossSignCA(newRoot)
	require.NoError(err)
	xc, err := connect.ParseCert(xcPEM)
	require.NoError(err)

	oldRootPEM, err := provider1.ActiveRoot()
	require.NoError(err)
	oldRoot, err := connect.ParseCert(oldRootPEM)
	require.NoError(err)

	// AuthorityKeyID should now be the signing root's, SubjectKeyId should be kept.
	require.Equal(oldRoot.AuthorityKeyId, xc.AuthorityKeyId)
	require.Equal(newRoot.SubjectKeyId, xc.SubjectKeyId)

	// Subject name should not have changed.
	require.Equal(oldSubject, xc.Subject.CommonName)

	// Issuer should be the signing root.
	require.Equal(oldRoot.Issuer.CommonName, xc.Issuer.CommonName)

	// Get a leaf cert so we can verify against the cross-signed cert.
	spiffeService := &connect.SpiffeIDService{
		Host:       "node1",
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
	t.Parallel()
	require := require.New(t)

	conf1 := testConsulCAConfig()
	delegate1 := newMockDelegate(t, conf1)
	provider1 := &ConsulProvider{Delegate: delegate1}
	require.NoError(provider1.Configure(conf1.ClusterID, true, conf1.Config))
	require.NoError(provider1.GenerateRoot())

	conf2 := testConsulCAConfig()
	conf2.CreateIndex = 10
	delegate2 := newMockDelegate(t, conf2)
	provider2 := &ConsulProvider{Delegate: delegate2}
	require.NoError(provider2.Configure(conf2.ClusterID, false, conf2.Config))

	testSignIntermediateCrossDC(t, provider1, provider2)
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
		Host:       "node1",
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
	t.Parallel()

	require := require.New(t)
	conf := testConsulCAConfig()
	delegate := newMockDelegate(t, conf)

	// Create an entry with an old-style ID.
	err := delegate.ApplyCARequest(&structs.CARequest{
		Op: structs.CAOpSetProviderState,
		ProviderState: &structs.CAConsulProviderState{
			ID: ",",
		},
	})
	require.NoError(err)
	_, providerState, err := delegate.state.CAProviderState(",")
	require.NoError(err)
	require.NotNil(providerState)

	provider := &ConsulProvider{Delegate: delegate}
	require.NoError(provider.Configure(conf.ClusterID, true, conf.Config))
	require.NoError(provider.GenerateRoot())

	// After running Configure, the old ID entry should be gone.
	_, providerState, err = delegate.state.CAProviderState(",")
	require.NoError(err)
	require.Nil(providerState)
}
