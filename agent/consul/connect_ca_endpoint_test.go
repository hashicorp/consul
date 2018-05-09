package consul

import (
	"crypto/x509"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	ca "github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
)

func testParseCert(t *testing.T, pemValue string) *x509.Certificate {
	cert, err := connect.ParseCert(pemValue)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// Test listing root CAs.
func TestConnectCARoots(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Insert some CAs
	state := s1.fsm.State()
	ca1 := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, nil)
	ca2.Active = false
	idx, _, err := state.CARoots(nil)
	require.NoError(err)
	ok, err := state.CARootSetCAS(idx, idx, []*structs.CARoot{ca1, ca2})
	assert.True(ok)
	require.NoError(err)
	_, caCfg, err := state.CAConfig()
	require.NoError(err)

	// Request
	args := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.IndexedCARoots
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", args, &reply))

	// Verify
	assert.Equal(ca1.ID, reply.ActiveRootID)
	assert.Len(reply.Roots, 2)
	for _, r := range reply.Roots {
		// These must never be set, for security
		assert.Equal("", r.SigningCert)
		assert.Equal("", r.SigningKey)
	}
	assert.Equal(fmt.Sprintf("%s.consul", caCfg.ClusterID), reply.TrustDomain)
}

func TestConnectCAConfig_GetSet(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Get the starting config
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.CAConfiguration
		assert.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply))

		actual, err := ca.ParseConsulCAConfig(reply.Config)
		assert.NoError(err)
		expected, err := ca.ParseConsulCAConfig(s1.config.CAConfig.Config)
		assert.NoError(err)
		assert.Equal(reply.Provider, s1.config.CAConfig.Provider)
		assert.Equal(actual, expected)
	}

	// Update a config value
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey":     "",
			"RootCert":       "",
			"RotationPeriod": 180 * 24 * time.Hour,
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		assert.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Verify the new config was set
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.CAConfiguration
		assert.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply))

		actual, err := ca.ParseConsulCAConfig(reply.Config)
		assert.NoError(err)
		expected, err := ca.ParseConsulCAConfig(newConfig.Config)
		assert.NoError(err)
		assert.Equal(reply.Provider, newConfig.Provider)
		assert.Equal(actual, expected)
	}
}

func TestConnectCAConfig_TriggerRotation(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Store the current root
	rootReq := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var rootList structs.IndexedCARoots
	assert.Nil(msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
	assert.Len(rootList.Roots, 1)
	oldRoot := rootList.Roots[0]

	// Update the provider config to use a new private key, which should
	// cause a rotation.
	_, newKey, err := connect.GeneratePrivateKey()
	assert.NoError(err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey":     newKey,
			"RootCert":       "",
			"RotationPeriod": 90 * 24 * time.Hour,
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		assert.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Make sure the new root has been added along with an intermediate
	// cross-signed by the old root.
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.IndexedCARoots
		assert.Nil(msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", args, &reply))
		assert.Len(reply.Roots, 2)

		for _, r := range reply.Roots {
			if r.ID == oldRoot.ID {
				// The old root should no longer be marked as the active root,
				// and none of its other fields should have changed.
				assert.False(r.Active)
				assert.Equal(r.Name, oldRoot.Name)
				assert.Equal(r.RootCert, oldRoot.RootCert)
				assert.Equal(r.SigningCert, oldRoot.SigningCert)
				assert.Equal(r.IntermediateCerts, oldRoot.IntermediateCerts)
			} else {
				// The new root should have a valid cross-signed cert from the old
				// root as an intermediate.
				assert.True(r.Active)
				assert.Len(r.IntermediateCerts, 1)

				xc := testParseCert(t, r.IntermediateCerts[0])
				oldRootCert := testParseCert(t, oldRoot.RootCert)
				newRootCert := testParseCert(t, r.RootCert)

				// Should have the authority/subject key IDs and signature algo of the
				// (old) signing CA.
				assert.Equal(xc.AuthorityKeyId, oldRootCert.AuthorityKeyId)
				assert.Equal(xc.SubjectKeyId, oldRootCert.SubjectKeyId)
				assert.Equal(xc.SignatureAlgorithm, oldRootCert.SignatureAlgorithm)

				// The common name and SAN should not have changed.
				assert.Equal(xc.Subject.CommonName, newRootCert.Subject.CommonName)
				assert.Equal(xc.URIs, newRootCert.URIs)
			}
		}
	}

	// Verify the new config was set.
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.CAConfiguration
		assert.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply))

		actual, err := ca.ParseConsulCAConfig(reply.Config)
		assert.NoError(err)
		expected, err := ca.ParseConsulCAConfig(newConfig.Config)
		assert.NoError(err)
		assert.Equal(reply.Provider, newConfig.Provider)
		assert.Equal(actual, expected)
	}
}

// Test CA signing
func TestConnectCASign(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Generate a CSR and request signing
	spiffeId := connect.TestSpiffeIDService(t, "web")
	spiffeId.Host = testGetClusterTrustDomain(t, s1)
	csr, _ := connect.TestCSR(t, spiffeId)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}
	var reply structs.IssuedCert
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply))

	// Get the current CA
	state := s1.fsm.State()
	_, ca, err := state.CARootActive(nil)
	require.NoError(err)

	// Verify that the cert is signed by the CA
	roots := x509.NewCertPool()
	assert.True(roots.AppendCertsFromPEM([]byte(ca.RootCert)))
	leaf, err := connect.ParseCert(reply.CertPEM)
	require.NoError(err)
	_, err = leaf.Verify(x509.VerifyOptions{
		Roots: roots,
	})
	require.NoError(err)

	// Verify other fields
	assert.Equal("web", reply.Service)
	assert.Equal(spiffeId.URI().String(), reply.ServiceURI)
}

func testGetClusterTrustDomain(t *testing.T, s *Server) string {
	t.Helper()
	state := s.fsm.State()
	_, config, err := state.CAConfig()
	require.NoError(t, err)
	// Build TrustDomain based on the ClusterID stored.
	signingID := connect.SpiffeIDSigningForCluster(config)
	return signingID.Host()
}

func TestConnectCASignValidation(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL token with service:write for web*
	var webToken string
	{
		arg := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name: "User token",
				Type: structs.ACLTypeClient,
				Rules: `
				service "web" {
					policy = "write"
				}`,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &webToken))
	}

	trustDomain := testGetClusterTrustDomain(t, s1)

	tests := []struct {
		name    string
		id      connect.CertURI
		wantErr string
	}{
		{
			name: "different cluster",
			id: &connect.SpiffeIDService{
				"55555555-4444-3333-2222-111111111111.consul",
				"default", "dc1", "web"},
			wantErr: "different trust domain",
		},
		{
			name: "same cluster should validate",
			id: &connect.SpiffeIDService{
				trustDomain,
				"default", "dc1", "web"},
			wantErr: "",
		},
		{
			name: "same cluster, CSR for a different DC should NOT validate",
			id: &connect.SpiffeIDService{
				trustDomain,
				"default", "dc2", "web"},
			wantErr: "different datacenter",
		},
		{
			name: "same cluster and DC, different service should not have perms",
			id: &connect.SpiffeIDService{
				trustDomain,
				"default", "dc1", "db"},
			wantErr: "Permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csr, _ := connect.TestCSR(t, tt.id)
			args := &structs.CASignRequest{
				Datacenter:   "dc1",
				CSR:          csr,
				WriteRequest: structs.WriteRequest{Token: webToken},
			}
			var reply structs.IssuedCert
			err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply)
			if tt.wantErr == "" {
				require.NoError(t, err)
				// No other validation that is handled in different tests
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
