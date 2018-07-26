package ca

import (
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
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
		Config:    map[string]interface{}{},
	}
}

func TestConsulCAProvider_Bootstrap(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	conf := testConsulCAConfig()
	delegate := newMockDelegate(t, conf)

	provider, err := NewConsulProvider(conf.Config, delegate)
	assert.NoError(err)

	root, err := provider.ActiveRoot()
	assert.NoError(err)

	// Intermediate should be the same cert.
	inter, err := provider.ActiveIntermediate()
	assert.NoError(err)
	assert.Equal(root, inter)

	// Should be a valid cert
	parsed, err := connect.ParseCert(root)
	assert.NoError(err)
	assert.Equal(parsed.URIs[0].String(), fmt.Sprintf("spiffe://%s.consul", conf.ClusterID))
}

func TestConsulCAProvider_Bootstrap_WithCert(t *testing.T) {
	t.Parallel()

	// Make sure setting a custom private key/root cert works.
	assert := assert.New(t)
	rootCA := connect.TestCA(t, nil)
	conf := testConsulCAConfig()
	conf.Config = map[string]interface{}{
		"PrivateKey": rootCA.SigningKey,
		"RootCert":   rootCA.RootCert,
	}
	delegate := newMockDelegate(t, conf)

	provider, err := NewConsulProvider(conf.Config, delegate)
	assert.NoError(err)

	root, err := provider.ActiveRoot()
	assert.NoError(err)
	assert.Equal(root, rootCA.RootCert)
}

func TestConsulCAProvider_SignLeaf(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	conf := testConsulCAConfig()
	conf.Config["LeafCertTTL"] = "1h"
	delegate := newMockDelegate(t, conf)

	provider, err := NewConsulProvider(conf.Config, delegate)
	require.NoError(err)

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

	conf1 := testConsulCAConfig()
	delegate1 := newMockDelegate(t, conf1)
	provider1, err := NewConsulProvider(conf1.Config, delegate1)
	require.NoError(t, err)

	conf2 := testConsulCAConfig()
	conf2.CreateIndex = 10
	delegate2 := newMockDelegate(t, conf2)
	provider2, err := NewConsulProvider(conf2.Config, delegate2)
	require.NoError(t, err)

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
