package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/assert"
)

func TestCAProvider_Bootstrap(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	provider := s1.getCAProvider()

	root, err := provider.ActiveRoot()
	assert.NoError(err)

	// Intermediate should be the same cert.
	inter, err := provider.ActiveIntermediate()
	assert.NoError(err)

	// Make sure we initialize without errors and that the
	// root cert gets set to the active cert.
	state := s1.fsm.State()
	_, activeRoot, err := state.CARootActive(nil)
	assert.NoError(err)
	assert.Equal(root, activeRoot.RootCert)
	assert.Equal(inter, activeRoot.RootCert)
}

func TestCAProvider_Bootstrap_WithCert(t *testing.T) {
	t.Parallel()

	// Make sure setting a custom private key/root cert works.
	assert := assert.New(t)
	rootCA := connect.TestCA(t, nil)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.CAConfig.Config["PrivateKey"] = rootCA.SigningKey
		c.CAConfig.Config["RootCert"] = rootCA.RootCert
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	provider := s1.getCAProvider()

	root, err := provider.ActiveRoot()
	assert.NoError(err)

	// Make sure we initialize without errors and that the
	// root cert we provided gets set to the active cert.
	state := s1.fsm.State()
	_, activeRoot, err := state.CARootActive(nil)
	assert.NoError(err)
	assert.Equal(root, activeRoot.RootCert)
	assert.Equal(rootCA.RootCert, activeRoot.RootCert)
}

func TestCAProvider_SignLeaf(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	provider := s1.getCAProvider()

	spiffeService := &connect.SpiffeIDService{
		Host:       s1.config.NodeName,
		Namespace:  "default",
		Datacenter: s1.config.Datacenter,
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
		assert.Equal(parsed.SerialNumber.Uint64(), uint64(1))

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

	// Make sure setting a custom private key/root cert works.
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	provider := s1.getCAProvider()

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
