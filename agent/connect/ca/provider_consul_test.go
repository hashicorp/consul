package ca

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
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
	if err := s.CASetConfig(0, conf); err != nil {
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

func TestCAProvider_Bootstrap(t *testing.T) {
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

func TestCAProvider_Bootstrap_WithCert(t *testing.T) {
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

func TestCAProvider_SignLeaf(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	conf := testConsulCAConfig()
	delegate := newMockDelegate(t, conf)

	provider, err := NewConsulProvider(conf.Config, delegate)
	assert.NoError(err)

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
		assert.NoError(err)

		cert, err := provider.Sign(csr)
		assert.NoError(err)

		parsed, err := connect.ParseCert(cert)
		assert.NoError(err)
		assert.Equal(parsed.URIs[0], spiffeService.URI())
		assert.Equal(parsed.Subject.CommonName, "foo")
		assert.Equal(uint64(2), parsed.SerialNumber.Uint64())

		// Ensure the cert is valid now and expires within the correct limit.
		assert.True(parsed.NotAfter.Sub(time.Now()) < 3*24*time.Hour)
		assert.True(parsed.NotBefore.Before(time.Now()))
	}

	// Generate a new cert for another service and make sure
	// the serial number is incremented.
	spiffeService.Service = "bar"
	{
		raw, _ := connect.TestCSR(t, spiffeService)

		csr, err := connect.ParseCSR(raw)
		assert.NoError(err)

		cert, err := provider.Sign(csr)
		assert.NoError(err)

		parsed, err := connect.ParseCert(cert)
		assert.NoError(err)
		assert.Equal(parsed.URIs[0], spiffeService.URI())
		assert.Equal(parsed.Subject.CommonName, "bar")
		assert.Equal(parsed.SerialNumber.Uint64(), uint64(2))

		// Ensure the cert is valid now and expires within the correct limit.
		assert.True(parsed.NotAfter.Sub(time.Now()) < 3*24*time.Hour)
		assert.True(parsed.NotBefore.Before(time.Now()))
	}
}

func TestCAProvider_CrossSignCA(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	conf := testConsulCAConfig()
	delegate := newMockDelegate(t, conf)
	provider, err := NewConsulProvider(conf.Config, delegate)
	assert.NoError(err)

	// Make a new CA cert to get cross-signed.
	rootCA := connect.TestCA(t, nil)
	rootPEM, err := provider.ActiveRoot()
	assert.NoError(err)
	root, err := connect.ParseCert(rootPEM)
	assert.NoError(err)

	// Have the provider cross sign our new CA cert.
	cert, err := connect.ParseCert(rootCA.RootCert)
	assert.NoError(err)
	oldSubject := cert.Subject.CommonName
	xcPEM, err := provider.CrossSignCA(cert)
	assert.NoError(err)

	xc, err := connect.ParseCert(xcPEM)
	assert.NoError(err)

	// AuthorityKeyID and SubjectKeyID should be the signing root's.
	assert.Equal(root.AuthorityKeyId, xc.AuthorityKeyId)
	assert.Equal(root.SubjectKeyId, xc.SubjectKeyId)

	// Subject name should not have changed.
	assert.NotEqual(root.Subject.CommonName, xc.Subject.CommonName)
	assert.Equal(oldSubject, xc.Subject.CommonName)

	// Issuer should be the signing root.
	assert.Equal(root.Issuer.CommonName, xc.Issuer.CommonName)
}
