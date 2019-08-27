package consul

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	ca "github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
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

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

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
	_, caCfg, err := state.CAConfig(nil)
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

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

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
		retry.Run(t, func(r *retry.R) {
			r.Check(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
		})
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
	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Store the current root
	rootReq := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var rootList structs.IndexedCARoots
	require.Nil(msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
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

		require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Make sure the new root has been added along with an intermediate
	// cross-signed by the old root.
	var newRootPEM string
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.IndexedCARoots
		require.Nil(msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", args, &reply))
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
				newRootPEM = r.RootCert
				// The new root should have a valid cross-signed cert from the old
				// root as an intermediate.
				assert.True(r.Active)
				assert.Len(r.IntermediateCerts, 1)

				xc := testParseCert(t, r.IntermediateCerts[0])
				oldRootCert := testParseCert(t, oldRoot.RootCert)
				newRootCert := testParseCert(t, r.RootCert)

				// Should have the authority key ID and signature algo of the
				// (old) signing CA.
				assert.Equal(xc.AuthorityKeyId, oldRootCert.AuthorityKeyId)
				assert.NotEqual(xc.SubjectKeyId, oldRootCert.SubjectKeyId)
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
		require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply))

		actual, err := ca.ParseConsulCAConfig(reply.Config)
		require.NoError(err)
		expected, err := ca.ParseConsulCAConfig(newConfig.Config)
		require.NoError(err)
		assert.Equal(reply.Provider, newConfig.Provider)
		assert.Equal(actual, expected)
	}

	// Verify that new leaf certs get the cross-signed intermediate bundled
	{
		// Generate a CSR and request signing
		spiffeId := connect.TestSpiffeIDService(t, "web")
		csr, _ := connect.TestCSR(t, spiffeId)
		args := &structs.CASignRequest{
			Datacenter: "dc1",
			CSR:        csr,
		}
		var reply structs.IssuedCert
		require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply))

		// Verify that the cert is signed by the new CA
		{
			roots := x509.NewCertPool()
			require.True(roots.AppendCertsFromPEM([]byte(newRootPEM)))
			leaf, err := connect.ParseCert(reply.CertPEM)
			require.NoError(err)
			_, err = leaf.Verify(x509.VerifyOptions{
				Roots: roots,
			})
			require.NoError(err)
		}

		// And that it validates via the intermediate
		{
			roots := x509.NewCertPool()
			assert.True(roots.AppendCertsFromPEM([]byte(oldRoot.RootCert)))
			leaf, err := connect.ParseCert(reply.CertPEM)
			require.NoError(err)

			// Make sure the intermediate was returned as well as leaf
			_, rest := pem.Decode([]byte(reply.CertPEM))
			require.NotEmpty(rest)

			intermediates := x509.NewCertPool()
			require.True(intermediates.AppendCertsFromPEM(rest))

			_, err = leaf.Verify(x509.VerifyOptions{
				Roots:         roots,
				Intermediates: intermediates,
			})
			require.NoError(err)
		}

		// Verify other fields
		assert.Equal("web", reply.Service)
		assert.Equal(spiffeId.URI().String(), reply.ServiceURI)
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
	csr, _ := connect.TestCSR(t, spiffeId)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}
	var reply structs.IssuedCert
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply))

	// Generate a second CSR and request signing
	spiffeId2 := connect.TestSpiffeIDService(t, "web2")
	csr, _ = connect.TestCSR(t, spiffeId2)
	args = &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}

	var reply2 structs.IssuedCert
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply2))
	require.True(reply2.ModifyIndex > reply.ModifyIndex)

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

// Bench how long Signing RPC takes. This was used to ballpark reasonable
// default rate limit to protect servers from thundering herds of signing
// requests on root rotation.
func BenchmarkConnectCASign(b *testing.B) {
	t := &testing.T{}

	require := require.New(b)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Generate a CSR and request signing
	spiffeID := connect.TestSpiffeIDService(b, "web")
	csr, _ := connect.TestCSR(b, spiffeID)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}
	var reply structs.IssuedCert

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply))
	}
}

func TestConnectCASign_rateLimit(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.CAConfig.Config = map[string]interface{}{
			// It actually doesn't work as expected with some higher values because
			// the token bucket is initialized with max(10%, 1) burst which for small
			// values is 1 and then the test completes so fast it doesn't actually
			// replenish any tokens so you only get the burst allowed through. This is
			// OK, running the test slower is likely to be more brittle anyway since
			// it will become more timing dependent whether the actual rate the
			// requests are made matches the expectation from the sleeps etc.
			"CSRMaxPerSecond": 1,
		}
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Generate a CSR and request signing a few times in a loop.
	spiffeID := connect.TestSpiffeIDService(t, "web")
	csr, _ := connect.TestCSR(t, spiffeID)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}
	var reply structs.IssuedCert

	errs := make([]error, 10)
	for i := 0; i < len(errs); i++ {
		errs[i] = msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply)
	}

	limitedCount := 0
	successCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
		} else if err.Error() == ErrRateLimited.Error() {
			limitedCount++
		} else {
			require.NoError(err)
		}
	}
	// I've only ever seen this as 1/9 however if the test runs slowly on an
	// over-subscribed CPU (e.g. in CI) it's possible that later requests could
	// have had their token replenished and succeed so we allow a little slack -
	// the test here isn't really the exact token bucket response more a sanity
	// check that some limiting is being applied. Note that we can't just measure
	// the time it took to send them all and infer how many should have succeeded
	// without some complex modeling of the token bucket algorithm.
	require.Truef(successCount >= 1, "at least 1 CSRs should have succeeded, got %d", successCount)
	require.Truef(limitedCount >= 7, "at least 7 CSRs should have been rate limited, got %d", limitedCount)
}

func TestConnectCASign_concurrencyLimit(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.CAConfig.Config = map[string]interface{}{
			// Must disable the rate limit since it takes precedence
			"CSRMaxPerSecond":  0,
			"CSRMaxConcurrent": 1,
		}
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Generate a CSR and request signing a few times in a loop.
	spiffeID := connect.TestSpiffeIDService(t, "web")
	csr, _ := connect.TestCSR(t, spiffeID)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}

	var wg sync.WaitGroup

	errs := make(chan error, 10)
	times := make(chan time.Duration, cap(errs))
	start := time.Now()
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codec := rpcClient(t, s1)
			defer codec.Close()
			var reply structs.IssuedCert
			errs <- msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply)
			times <- time.Since(start)
		}()
	}

	wg.Wait()
	close(errs)

	limitedCount := 0
	successCount := 0
	var minTime, maxTime time.Duration
	for err := range errs {
		elapsed := <-times
		if elapsed < minTime || minTime == 0 {
			minTime = elapsed
		}
		if elapsed > maxTime {
			maxTime = elapsed
		}
		if err == nil {
			successCount++
		} else if err.Error() == ErrRateLimited.Error() {
			limitedCount++
		} else {
			require.NoError(err)
		}
	}

	// These are very hand wavy - on my mac times look like this:
	//     2.776009ms
	//     3.705813ms
	//     4.527212ms
	//     5.267755ms
	//     6.119809ms
	//     6.958083ms
	//     7.869179ms
	//     8.675058ms
	//     9.512281ms
	//     10.238183ms
	//
	// But it's indistinguishable from noise - even if you disable the concurrency
	// limiter you get pretty much the same pattern/spread.
	//
	// On the other hand it's only timing that stops us from not hitting the 500ms
	// timeout. On highly CPU constrained CI box this could be brittle if we
	// assert that we never get rate limited.
	//
	// So this test is not super strong - but it's a sanity check at least that
	// things don't break when configured this way, and through manual
	// inspection/debug logging etc. we can verify it's actually doing the
	// concurrency limit thing. If you add a 100ms sleep into the sign endpoint
	// after the rate limit code for example it makes it much more obvious:
	//
	//   With 100ms sleep an no concurrency limit:
	//     min=109ms, max=118ms
	//   With concurrency limit of 1:
	//     min=106ms, max=538ms (with ~half hitting the 500ms timeout)
	//
	// Without instrumenting the endpoint to make the RPC take an artificially
	// long time it's hard to know what else we can do to actively detect that the
	// requests were serialized.
	t.Logf("min=%s, max=%s", minTime, maxTime)
	//t.Fail() // Uncomment to see the time spread logged
	require.Truef(successCount >= 1, "at least 1 CSRs should have succeeded, got %d", successCount)
}

func TestConnectCASignValidation(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
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
				Type: structs.ACLTokenTypeClient,
				Rules: `
				service "web" {
					policy = "write"
				}`,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &webToken))
	}

	testWebID := connect.TestSpiffeIDService(t, "web")

	tests := []struct {
		name    string
		id      connect.CertURI
		wantErr string
	}{
		{
			name: "different cluster",
			id: &connect.SpiffeIDService{
				Host:       "55555555-4444-3333-2222-111111111111.consul",
				Namespace:  testWebID.Namespace,
				Datacenter: testWebID.Datacenter,
				Service:    testWebID.Service,
			},
			wantErr: "different trust domain",
		},
		{
			name:    "same cluster should validate",
			id:      testWebID,
			wantErr: "",
		},
		{
			name: "same cluster, CSR for a different DC should NOT validate",
			id: &connect.SpiffeIDService{
				Host:       testWebID.Host,
				Namespace:  testWebID.Namespace,
				Datacenter: "dc2",
				Service:    testWebID.Service,
			},
			wantErr: "different datacenter",
		},
		{
			name: "same cluster and DC, different service should not have perms",
			id: &connect.SpiffeIDService{
				Host:       testWebID.Host,
				Namespace:  testWebID.Namespace,
				Datacenter: testWebID.Datacenter,
				Service:    "db",
			},
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
