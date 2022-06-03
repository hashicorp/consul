package consul

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"

	"github.com/hashicorp/consul/agent/connect"
	ca "github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

// TODO(kyhavlov): replace with t.Deadline()
const CATestTimeout = 7 * time.Second

func TestCAManager_Initialize_Vault_Secondary_SharedVault(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	ca.SkipIfVaultNotPresent(t)

	vault := ca.NewTestVaultServer(t)

	_, serverDC1 := testServerWithConfig(t, func(c *Config) {
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             vault.Addr,
				"Token":               vault.RootToken,
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-primary/",
			},
		}
	})

	testutil.RunStep(t, "check primary DC", func(t *testing.T) {
		testrpc.WaitForTestAgent(t, serverDC1.RPC, "dc1")

		codec := rpcClient(t, serverDC1)
		roots := structs.IndexedCARoots{}
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 1)

		leafPEM := getLeafCert(t, codec, roots.TrustDomain, "dc1")
		verifyLeafCert(t, roots.Roots[0], leafPEM)
	})

	testutil.RunStep(t, "start secondary DC", func(t *testing.T) {
		_, serverDC2 := testServerWithConfig(t, func(c *Config) {
			c.Datacenter = "dc2"
			c.PrimaryDatacenter = "dc1"
			c.CAConfig = &structs.CAConfiguration{
				Provider: "vault",
				Config: map[string]interface{}{
					"Address":             vault.Addr,
					"Token":               vault.RootToken,
					"RootPKIPath":         "pki-root/",
					"IntermediatePKIPath": "pki-secondary/",
				},
			}
		})
		joinWAN(t, serverDC2, serverDC1)
		testrpc.WaitForActiveCARoot(t, serverDC2.RPC, "dc2", nil)

		codec := rpcClient(t, serverDC2)
		roots := structs.IndexedCARoots{}
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 1)

		leafPEM := getLeafCert(t, codec, roots.TrustDomain, "dc2")
		verifyLeafCert(t, roots.Roots[0], leafPEM)
	})
}

func verifyLeafCert(t *testing.T, root *structs.CARoot, leafCertPEM string) {
	t.Helper()
	roots := structs.IndexedCARoots{
		ActiveRootID: root.ID,
		Roots:        []*structs.CARoot{root},
	}
	verifyLeafCertWithRoots(t, roots, leafCertPEM)
}

func verifyLeafCertWithRoots(t *testing.T, roots structs.IndexedCARoots, leafCertPEM string) {
	t.Helper()
	leaf, intermediates, err := connect.ParseLeafCerts(leafCertPEM)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	for _, r := range roots.Roots {
		ok := pool.AppendCertsFromPEM([]byte(r.RootCert))
		if !ok {
			t.Fatalf("Failed to add root CA PEM to cert pool")
		}
	}

	// verify with intermediates from leaf CertPEM
	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:         pool,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err, "failed to verify using intermediates from leaf cert PEM")

	// verify with intermediates from the CARoot
	intermediates = x509.NewCertPool()
	for _, r := range roots.Roots {
		for _, intermediate := range r.IntermediateCerts {
			c, err := connect.ParseCert(intermediate)
			require.NoError(t, err)
			intermediates.AddCert(c)
		}
	}

	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:         pool,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err, "failed to verify using intermediates from CARoot list")
}

type mockCAServerDelegate struct {
	t                     *testing.T
	config                *Config
	store                 *state.Store
	primaryRoot           *structs.CARoot
	secondaryIntermediate string
	callbackCh            chan string
}

func NewMockCAServerDelegate(t *testing.T, config *Config) *mockCAServerDelegate {
	delegate := &mockCAServerDelegate{
		t:           t,
		config:      config,
		store:       state.NewStateStore(nil),
		primaryRoot: connect.TestCAWithTTL(t, nil, 1*time.Second),
		callbackCh:  make(chan string, 0),
	}
	delegate.store.CASetConfig(1, testCAConfig())

	return delegate
}

func (m *mockCAServerDelegate) State() *state.Store {
	return m.store
}

func (m *mockCAServerDelegate) ProviderState(id string) (*structs.CAConsulProviderState, error) {
	_, s, err := m.store.CAProviderState(id)
	return s, err
}

func (m *mockCAServerDelegate) IsLeader() bool {
	return true
}

func (m *mockCAServerDelegate) ServersSupportMultiDCConnectCA() error {
	return nil
}

func (m *mockCAServerDelegate) ApplyCALeafRequest() (uint64, error) {
	return 3, nil
}

// ApplyCARequest mirrors FSM.applyConnectCAOperation because that functionality
// is not exported.
func (m *mockCAServerDelegate) ApplyCARequest(req *structs.CARequest) (interface{}, error) {
	idx, _, err := m.store.CAConfig(nil)
	if err != nil {
		return nil, err
	}

	m.callbackCh <- fmt.Sprintf("raftApply/ConnectCA")

	result := fsm.ApplyConnectCAOperationFromRequest(m.store, req, idx+1)
	if err, ok := result.(error); ok && err != nil {
		return nil, err
	}
	return result, nil
}

func (m *mockCAServerDelegate) forwardDC(method, dc string, args interface{}, reply interface{}) error {
	switch method {
	case "ConnectCA.Roots":
		roots := reply.(*structs.IndexedCARoots)
		roots.TrustDomain = connect.TestClusterID
		roots.Roots = []*structs.CARoot{m.primaryRoot}
		roots.ActiveRootID = m.primaryRoot.ID
	case "ConnectCA.SignIntermediate":
		r := reply.(*string)
		*r = m.secondaryIntermediate
	default:
		return fmt.Errorf("received call to unsupported method %q", method)
	}

	m.callbackCh <- fmt.Sprintf("forwardDC/%s", method)

	return nil
}

func (m *mockCAServerDelegate) generateCASignRequest(csr string) *structs.CASignRequest {
	return &structs.CASignRequest{
		Datacenter: m.config.PrimaryDatacenter,
		CSR:        csr,
	}
}

// mockCAProvider mocks an empty provider implementation with a channel in order to coordinate
// waiting for certain methods to be called.
type mockCAProvider struct {
	callbackCh      chan string
	rootPEM         string
	intermediatePem string
}

func (m *mockCAProvider) Configure(cfg ca.ProviderConfig) error { return nil }
func (m *mockCAProvider) State() (map[string]string, error)     { return nil, nil }
func (m *mockCAProvider) GenerateRoot() (ca.RootResult, error) {
	return ca.RootResult{PEM: m.rootPEM}, nil
}
func (m *mockCAProvider) GenerateIntermediateCSR() (string, error) {
	m.callbackCh <- "provider/GenerateIntermediateCSR"
	return "", nil
}
func (m *mockCAProvider) SetIntermediate(intermediatePEM, rootPEM string) error {
	m.callbackCh <- "provider/SetIntermediate"
	return nil
}
func (m *mockCAProvider) ActiveIntermediate() (string, error) {
	if m.intermediatePem == "" {
		return m.rootPEM, nil
	}
	return m.intermediatePem, nil
}
func (m *mockCAProvider) GenerateIntermediate() (string, error)                     { return "", nil }
func (m *mockCAProvider) Sign(*x509.CertificateRequest) (string, error)             { return "", nil }
func (m *mockCAProvider) SignIntermediate(*x509.CertificateRequest) (string, error) { return "", nil }
func (m *mockCAProvider) CrossSignCA(*x509.Certificate) (string, error)             { return "", nil }
func (m *mockCAProvider) SupportsCrossSigning() (bool, error)                       { return false, nil }
func (m *mockCAProvider) Cleanup(_ bool, _ map[string]interface{}) error            { return nil }

func waitForCh(t *testing.T, ch chan string, expected string) {
	t.Helper()
	select {
	case op := <-ch:
		if op != expected {
			t.Fatalf("got unexpected op %q, wanted %q", op, expected)
		}
	case <-time.After(CATestTimeout):
		t.Fatalf("never got op %q", expected)
	}
}

func waitForEmptyCh(t *testing.T, ch chan string) {
	select {
	case op := <-ch:
		t.Fatalf("got unexpected op %q", op)
	case <-time.After(1 * time.Second):
	}
}

func testCAConfig() *structs.CAConfiguration {
	return &structs.CAConfiguration{
		ClusterID: connect.TestClusterID,
		Provider:  "mock",
		Config: map[string]interface{}{
			"LeafCertTTL":         "72h",
			"IntermediateCertTTL": "2160h",
		},
	}
}

// initTestManager initializes a CAManager with a mockCAServerDelegate, consuming
// the ops that come through the channels and returning when initialization has finished.
func initTestManager(t *testing.T, manager *CAManager, delegate *mockCAServerDelegate) {
	t.Helper()
	initCh := make(chan struct{})
	go func() {
		require.NoError(t, manager.Initialize())
		close(initCh)
	}()
	for i := 0; i < 5; i++ {
		select {
		case <-delegate.callbackCh:
		case <-time.After(CATestTimeout):
			t.Fatal("failed waiting for initialization events")
		}
	}
	select {
	case <-initCh:
	case <-time.After(CATestTimeout):
		t.Fatal("failed waiting for initialization")
	}
}

func TestCAManager_Initialize(t *testing.T) {
	conf := DefaultConfig()
	conf.ConnectEnabled = true
	conf.PrimaryDatacenter = "dc1"
	conf.Datacenter = "dc2"
	delegate := NewMockCAServerDelegate(t, conf)
	delegate.secondaryIntermediate = delegate.primaryRoot.RootCert
	manager := NewCAManager(delegate, nil, testutil.Logger(t), conf)

	manager.providerShim = &mockCAProvider{
		callbackCh: delegate.callbackCh,
		rootPEM:    delegate.primaryRoot.RootCert,
	}

	// Call Initialize and then confirm the RPCs and provider calls
	// happen in the expected order.
	require.Equal(t, caStateUninitialized, manager.state)
	errCh := make(chan error)
	go func() {
		err := manager.Initialize()
		assert.NoError(t, err)
		errCh <- err
	}()

	waitForCh(t, delegate.callbackCh, "forwardDC/ConnectCA.Roots")
	require.EqualValues(t, caStateInitializing, manager.state)
	waitForCh(t, delegate.callbackCh, "provider/GenerateIntermediateCSR")
	waitForCh(t, delegate.callbackCh, "forwardDC/ConnectCA.SignIntermediate")
	waitForCh(t, delegate.callbackCh, "provider/SetIntermediate")
	waitForCh(t, delegate.callbackCh, "raftApply/ConnectCA")
	waitForEmptyCh(t, delegate.callbackCh)

	// Make sure the Initialize call returned successfully.
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(CATestTimeout):
		t.Fatal("never got result from errCh")
	}

	require.Equal(t, caStateInitialized, manager.state)
}

func TestCAManager_UpdateConfigWhileRenewIntermediate(t *testing.T) {

	// No parallel execution because we change globals
	patchIntermediateCertRenewInterval(t)

	conf := DefaultConfig()
	conf.ConnectEnabled = true
	conf.PrimaryDatacenter = "dc1"
	conf.Datacenter = "dc2"
	delegate := NewMockCAServerDelegate(t, conf)
	delegate.secondaryIntermediate = delegate.primaryRoot.RootCert
	manager := NewCAManager(delegate, nil, testutil.Logger(t), conf)
	manager.providerShim = &mockCAProvider{
		callbackCh: delegate.callbackCh,
		rootPEM:    delegate.primaryRoot.RootCert,
	}
	initTestManager(t, manager, delegate)

	// Simulate Wait half the TTL for the cert to need renewing.
	manager.timeNow = func() time.Time {
		return time.Now().Add(500 * time.Millisecond)
	}

	// Call RenewIntermediate and then confirm the RPCs and provider calls
	// happen in the expected order.
	errCh := make(chan error)
	go func() {
		errCh <- manager.RenewIntermediate(context.TODO(), false)
	}()

	waitForCh(t, delegate.callbackCh, "provider/GenerateIntermediateCSR")

	// Call UpdateConfiguration while RenewIntermediate is still in-flight to
	// make sure we get an error about the state being occupied.
	go func() {
		require.EqualValues(t, caStateRenewIntermediate, manager.state)
		require.Error(t, errors.New("already in state"), manager.UpdateConfiguration(&structs.CARequest{}))
	}()

	waitForCh(t, delegate.callbackCh, "forwardDC/ConnectCA.SignIntermediate")
	waitForCh(t, delegate.callbackCh, "provider/SetIntermediate")
	waitForCh(t, delegate.callbackCh, "raftApply/ConnectCA")
	waitForEmptyCh(t, delegate.callbackCh)

	// Make sure the RenewIntermediate call returned successfully.
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(CATestTimeout):
		t.Fatal("never got result from errCh")
	}

	require.EqualValues(t, caStateInitialized, manager.state)
}

func TestCAManager_SignCertificate_WithExpiredCert(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	args := []struct {
		testName              string
		notBeforeRoot         time.Time
		notAfterRoot          time.Time
		notBeforeIntermediate time.Time
		notAfterIntermediate  time.Time
		isError               bool
		errorMsg              string
	}{
		{"intermediate valid", time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 2), time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 2), false, ""},
		{"intermediate expired", time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 2), time.Now().AddDate(-2, 0, 0), time.Now().AddDate(0, 0, -1), true, "intermediate expired: certificate expired, expiration date"},
		{"root expired", time.Now().AddDate(-2, 0, 0), time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 2), true, "root expired: certificate expired, expiration date"},
		// a cert that is not yet valid is ok, assume it will be valid soon enough
		{"intermediate in the future", time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 2), time.Now().AddDate(0, 0, 1), time.Now().AddDate(0, 0, 2), false, ""},
		{"root in the future", time.Now().AddDate(0, 0, 1), time.Now().AddDate(0, 0, 2), time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 2), false, ""},
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err, "failed to generate key")

	for _, arg := range args {
		t.Run(arg.testName, func(t *testing.T) {
			// No parallel execution because we change globals
			// Set the interval and drift buffer low for renewing the cert.
			origInterval := structs.IntermediateCertRenewInterval
			origDriftBuffer := ca.CertificateTimeDriftBuffer
			defer func() {
				structs.IntermediateCertRenewInterval = origInterval
				ca.CertificateTimeDriftBuffer = origDriftBuffer
			}()
			structs.IntermediateCertRenewInterval = time.Millisecond
			ca.CertificateTimeDriftBuffer = 0

			conf := DefaultConfig()
			conf.ConnectEnabled = true
			conf.PrimaryDatacenter = "dc1"
			conf.Datacenter = "dc2"

			rootPEM := generateCertPEM(t, caPrivKey, arg.notBeforeRoot, arg.notAfterRoot)
			intermediatePEM := generateCertPEM(t, caPrivKey, arg.notBeforeIntermediate, arg.notAfterIntermediate)

			delegate := NewMockCAServerDelegate(t, conf)
			delegate.primaryRoot.RootCert = rootPEM
			delegate.secondaryIntermediate = intermediatePEM
			manager := NewCAManager(delegate, nil, testutil.Logger(t), conf)

			manager.providerShim = &mockCAProvider{
				callbackCh:      delegate.callbackCh,
				rootPEM:         rootPEM,
				intermediatePem: intermediatePEM,
			}
			initTestManager(t, manager, delegate)

			// Simulate Wait half the TTL for the cert to need renewing.
			manager.timeNow = func() time.Time {
				return time.Now().UTC().Add(500 * time.Millisecond)
			}

			// Call RenewIntermediate and then confirm the RPCs and provider calls
			// happen in the expected order.

			_, err := manager.SignCertificate(&x509.CertificateRequest{}, &connect.SpiffeIDAgent{})
			if arg.isError {
				require.Error(t, err)
				require.Contains(t, err.Error(), arg.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func generateCertPEM(t *testing.T, caPrivKey *rsa.PrivateKey, notBefore time.Time, notAfter time.Time) string {
	t.Helper()
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		URIs:                  []*url.URL{connect.SpiffeIDAgent{Host: "foo"}.URI()},
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err, "failed to create cert")

	caPEM := new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	require.NoError(t, err, "failed to encode")
	return caPEM.String()
}

func TestCADelegateWithState_GenerateCASignRequest(t *testing.T) {
	s := Server{config: &Config{PrimaryDatacenter: "east"}, tokens: new(token.Store)}
	d := &caDelegateWithState{Server: &s}
	req := d.generateCASignRequest("A")
	require.Equal(t, "east", req.RequestDatacenter())
}

func TestCAManager_Initialize_Logging(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, conf1 := testServerConfig(t)

	// Setup dummy logger to catch output
	var buf bytes.Buffer
	logger := testutil.LoggerWithOutput(t, &buf)

	deps := newDefaultDeps(t, conf1)
	deps.Logger = logger

	s1, err := NewServer(conf1, deps, grpc.NewServer())
	require.NoError(t, err)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Wait til CA root is setup
	retry.Run(t, func(r *retry.R) {
		var out structs.IndexedCARoots
		r.Check(s1.RPC("ConnectCA.Roots", structs.DCSpecificRequest{
			Datacenter: conf1.Datacenter,
		}, &out))
	})

	require.Contains(t, buf.String(), "consul CA provider configured")
}

func TestCAManager_UpdateConfiguration_Vault_Primary(t *testing.T) {
	ca.SkipIfVaultNotPresent(t)
	vault := ca.NewTestVaultServer(t)

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             vault.Addr,
				"Token":               vault.RootToken,
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate/",
			},
		}
	})
	defer func() {
		s1.Shutdown()
		s1.leaderRoutineManager.Wait()
	}()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	_, origRoot, err := s1.fsm.State().CARootActive(nil)
	require.NoError(t, err)
	require.Len(t, origRoot.IntermediateCerts, 1)

	cert, err := connect.ParseCert(s1.caManager.getLeafSigningCertFromRoot(origRoot))
	require.NoError(t, err)
	require.Equal(t, connect.HexString(cert.SubjectKeyId), origRoot.SigningKeyID)

	err = s1.caManager.UpdateConfiguration(&structs.CARequest{
		Config: &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             vault.Addr,
				"Token":               vault.RootToken,
				"RootPKIPath":         "pki-root-2/",
				"IntermediatePKIPath": "pki-intermediate-2/",
			},
		},
	})
	require.NoError(t, err)

	_, newRoot, err := s1.fsm.State().CARootActive(nil)
	require.NoError(t, err)
	require.Len(t, newRoot.IntermediateCerts, 2,
		"expected one cross-sign cert and one local leaf sign cert")
	require.NotEqual(t, origRoot.ID, newRoot.ID)

	cert, err = connect.ParseCert(s1.caManager.getLeafSigningCertFromRoot(newRoot))
	require.NoError(t, err)
	require.Equal(t, connect.HexString(cert.SubjectKeyId), newRoot.SigningKeyID)
}

func TestCAManager_Initialize_Vault_WithIntermediateAsPrimaryCA(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	ca.SkipIfVaultNotPresent(t)

	vault := ca.NewTestVaultServer(t)
	vclient := vault.Client()
	generateExternalRootCA(t, vclient)

	meshRootPath := "pki-root"
	primaryCert := setupPrimaryCA(t, vclient, meshRootPath, "")

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             vault.Addr,
				"Token":               vault.RootToken,
				"RootPKIPath":         meshRootPath,
				"IntermediatePKIPath": "pki-intermediate/",
			},
		}
	})

	testutil.RunStep(t, "check primary DC", func(t *testing.T) {
		testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

		codec := rpcClient(t, s1)
		roots := structs.IndexedCARoots{}
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 1)
		require.Equal(t, primaryCert, roots.Roots[0].RootCert)

		leafCertPEM := getLeafCert(t, codec, roots.TrustDomain, "dc1")
		verifyLeafCert(t, roots.Roots[0], leafCertPEM)
	})

	// TODO: renew primary leaf signing cert
	// TODO: rotate root

	testutil.RunStep(t, "run secondary DC", func(t *testing.T) {
		_, sDC2 := testServerWithConfig(t, func(c *Config) {
			c.Datacenter = "dc2"
			c.PrimaryDatacenter = "dc1"
			c.CAConfig = &structs.CAConfiguration{
				Provider: "vault",
				Config: map[string]interface{}{
					"Address":             vault.Addr,
					"Token":               vault.RootToken,
					"RootPKIPath":         meshRootPath,
					"IntermediatePKIPath": "pki-secondary/",
				},
			}
		})
		defer sDC2.Shutdown()
		joinWAN(t, sDC2, s1)
		testrpc.WaitForActiveCARoot(t, sDC2.RPC, "dc2", nil)

		codec := rpcClient(t, sDC2)
		roots := structs.IndexedCARoots{}
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 1)

		leafCertPEM := getLeafCert(t, codec, roots.TrustDomain, "dc2")
		verifyLeafCert(t, roots.Roots[0], leafCertPEM)

		// TODO: renew secondary leaf signing cert
	})
}

func TestCAManager_Verify_Vault_NoChangeToSecondaryConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	ca.SkipIfVaultNotPresent(t)

	vault := ca.NewTestVaultServer(t)

	_, sDC1 := testServerWithConfig(t, func(c *Config) {
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             vault.Addr,
				"Token":               vault.RootToken,
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate/",
			},
		}
	})
	defer sDC1.Shutdown()
	testrpc.WaitForActiveCARoot(t, sDC1.RPC, "dc1", nil)

	_, sDC2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             vault.Addr,
				"Token":               vault.RootToken,
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate-2/",
			},
		}
	})
	defer sDC2.Shutdown()
	joinWAN(t, sDC2, sDC1)
	testrpc.WaitForActiveCARoot(t, sDC2.RPC, "dc2", nil)

	codec := rpcClient(t, sDC2)
	var configBefore structs.CAConfiguration
	err := msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", &structs.DCSpecificRequest{}, &configBefore)
	require.NoError(t, err)

	renewLeafSigningCert(t, sDC1.caManager, sDC1.caManager.primaryRenewIntermediate)

	// Give the secondary some time to notice the update
	time.Sleep(100 * time.Millisecond)

	var configAfter structs.CAConfiguration
	err = msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", &structs.DCSpecificRequest{}, &configAfter)
	require.NoError(t, err)

	require.EqualValues(t, configBefore.ModifyIndex, configAfter.ModifyIndex)
}

func getLeafCert(t *testing.T, codec rpc.ClientCodec, trustDomain string, dc string) string {
	pk, _, err := connect.GeneratePrivateKey()
	require.NoError(t, err)
	spiffeID := &connect.SpiffeIDService{
		Host:       trustDomain,
		Service:    "srv1",
		Datacenter: dc,
	}
	csr, err := connect.CreateCSR(spiffeID, pk, nil, nil)
	require.NoError(t, err)

	req := structs.CASignRequest{CSR: csr}
	cert := structs.IssuedCert{}
	err = msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", &req, &cert)
	require.NoError(t, err)
	return cert.CertPEM
}

func TestCAManager_Initialize_Vault_WithExternalTrustedCA(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	ca.SkipIfVaultNotPresent(t)

	vault := ca.NewTestVaultServer(t)
	vclient := vault.Client()
	rootPEM := generateExternalRootCA(t, vclient)

	primaryCAPath := "pki-primary"
	primaryCert := setupPrimaryCA(t, vclient, primaryCAPath, rootPEM)

	_, serverDC1 := testServerWithConfig(t, func(c *Config) {
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             vault.Addr,
				"Token":               vault.RootToken,
				"RootPKIPath":         primaryCAPath,
				"IntermediatePKIPath": "pki-intermediate/",
			},
		}
	})
	testrpc.WaitForTestAgent(t, serverDC1.RPC, "dc1")

	var origLeaf string
	roots := structs.IndexedCARoots{}
	testutil.RunStep(t, "verify primary DC", func(t *testing.T) {
		codec := rpcClient(t, serverDC1)
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 1)
		require.Equal(t, primaryCert, roots.Roots[0].RootCert)
		require.Contains(t, roots.Roots[0].RootCert, rootPEM)

		leafCert := getLeafCert(t, codec, roots.TrustDomain, "dc1")
		verifyLeafCert(t, roots.Active(), leafCert)
		origLeaf = leafCert
	})

	_, serverDC2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             vault.Addr,
				"Token":               vault.RootToken,
				"RootPKIPath":         "should-be-ignored",
				"IntermediatePKIPath": "pki-secondary/",
			},
		}
	})

	var origLeafSecondary string
	testutil.RunStep(t, "start secondary DC", func(t *testing.T) {
		joinWAN(t, serverDC2, serverDC1)
		testrpc.WaitForActiveCARoot(t, serverDC2.RPC, "dc2", nil)

		codec := rpcClient(t, serverDC2)
		roots = structs.IndexedCARoots{}
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 1)

		leafPEM := getLeafCert(t, codec, roots.TrustDomain, "dc2")
		verifyLeafCert(t, roots.Roots[0], leafPEM)
		origLeafSecondary = leafPEM
	})

	testutil.RunStep(t, "renew leaf signing CA in primary", func(t *testing.T) {
		previous := serverDC1.caManager.getLeafSigningCertFromRoot(roots.Active())

		renewLeafSigningCert(t, serverDC1.caManager, serverDC1.caManager.primaryRenewIntermediate)

		codec := rpcClient(t, serverDC1)
		roots = structs.IndexedCARoots{}
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 1)
		require.Len(t, roots.Roots[0].IntermediateCerts, 2)

		newCert := serverDC1.caManager.getLeafSigningCertFromRoot(roots.Active())
		require.NotEqual(t, previous, newCert)

		leafPEM := getLeafCert(t, codec, roots.TrustDomain, "dc1")
		verifyLeafCert(t, roots.Roots[0], leafPEM)

		// original certs from old signing cert should still verify
		verifyLeafCert(t, roots.Roots[0], origLeaf)
	})

	testutil.RunStep(t, "renew leaf signing CA in secondary", func(t *testing.T) {
		previous := serverDC2.caManager.getLeafSigningCertFromRoot(roots.Active())

		renewLeafSigningCert(t, serverDC2.caManager, serverDC2.caManager.secondaryRequestNewSigningCert)

		codec := rpcClient(t, serverDC2)
		roots = structs.IndexedCARoots{}
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 1)
		// one intermediate from primary, two from secondary
		require.Len(t, roots.Roots[0].IntermediateCerts, 3)

		newCert := serverDC1.caManager.getLeafSigningCertFromRoot(roots.Active())
		require.NotEqual(t, previous, newCert)

		leafPEM := getLeafCert(t, codec, roots.TrustDomain, "dc2")
		verifyLeafCert(t, roots.Roots[0], leafPEM)

		// original certs from old signing cert should still verify
		verifyLeafCert(t, roots.Roots[0], origLeaf)
	})

	testutil.RunStep(t, "rotate root by changing the provider", func(t *testing.T) {
		codec := rpcClient(t, serverDC1)
		req := &structs.CARequest{
			Op: structs.CAOpSetConfig,
			Config: &structs.CAConfiguration{
				Provider: "consul",
			},
		}
		var resp error
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", req, &resp)
		require.NoError(t, err)
		require.Nil(t, resp)

		roots = structs.IndexedCARoots{}
		err = msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 2)
		active := roots.Active()
		require.Len(t, active.IntermediateCerts, 1)

		leafPEM := getLeafCert(t, codec, roots.TrustDomain, "dc1")
		verifyLeafCert(t, roots.Active(), leafPEM)

		// original certs from old root cert should still verify
		verifyLeafCertWithRoots(t, roots, origLeaf)

		// original certs from secondary should still verify
		rootsSecondary := structs.IndexedCARoots{}
		r := &structs.DCSpecificRequest{Datacenter: "dc2"}
		err = msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", r, &rootsSecondary)
		require.NoError(t, err)
		verifyLeafCertWithRoots(t, rootsSecondary, origLeafSecondary)
	})

	testutil.RunStep(t, "rotate to a different external root", func(t *testing.T) {
		setupPrimaryCA(t, vclient, "pki-primary-2/", rootPEM)

		codec := rpcClient(t, serverDC1)
		req := &structs.CARequest{
			Op: structs.CAOpSetConfig,
			Config: &structs.CAConfiguration{
				Provider: "vault",
				Config: map[string]interface{}{
					"Address":             vault.Addr,
					"Token":               vault.RootToken,
					"RootPKIPath":         "pki-primary-2/",
					"IntermediatePKIPath": "pki-intermediate-2/",
				},
			},
		}
		var resp error
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", req, &resp)
		require.NoError(t, err)
		require.Nil(t, resp)

		roots = structs.IndexedCARoots{}
		err = msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
		require.NoError(t, err)
		require.Len(t, roots.Roots, 3)
		active := roots.Active()
		require.Len(t, active.IntermediateCerts, 2)

		leafPEM := getLeafCert(t, codec, roots.TrustDomain, "dc1")
		verifyLeafCert(t, roots.Active(), leafPEM)

		// original certs from old root cert should still verify
		verifyLeafCertWithRoots(t, roots, origLeaf)

		// original certs from secondary should still verify
		rootsSecondary := structs.IndexedCARoots{}
		r := &structs.DCSpecificRequest{Datacenter: "dc2"}
		err = msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", r, &rootsSecondary)
		require.NoError(t, err)
		verifyLeafCertWithRoots(t, rootsSecondary, origLeafSecondary)
	})
}

// renewLeafSigningCert mimics RenewIntermediate. This is unfortunate, but
// necessary for now as there is no easy way to invoke that logic unconditionally.
// Currently, it requires patching values and polling for the operation to
// complete, which adds a lot of distractions to a test case.
// With this function we can instead unconditionally rotate the leaf signing cert
// synchronously.
func renewLeafSigningCert(t *testing.T, manager *CAManager, fn func(ca.Provider, *structs.CARoot) error) {
	t.Helper()
	provider, _ := manager.getCAProvider()

	store := manager.delegate.State()
	_, root, err := store.CARootActive(nil)
	require.NoError(t, err)

	activeRoot := root.Clone()
	err = fn(provider, activeRoot)
	require.NoError(t, err)
	err = manager.persistNewRootAndConfig(provider, activeRoot, nil)
	require.NoError(t, err)
	manager.setCAProvider(provider, activeRoot)
}

func generateExternalRootCA(t *testing.T, client *vaultapi.Client) string {
	t.Helper()
	err := client.Sys().Mount("corp", &vaultapi.MountInput{
		Type:        "pki",
		Description: "External root, probably corporate CA",
		Config: vaultapi.MountConfigInput{
			MaxLeaseTTL:     "2400h",
			DefaultLeaseTTL: "1h",
		},
	})
	require.NoError(t, err, "failed to mount")

	resp, err := client.Logical().Write("corp/root/generate/internal", map[string]interface{}{
		"common_name": "corporate CA",
		"ttl":         "2400h",
	})
	require.NoError(t, err, "failed to generate root")
	return lib.EnsureTrailingNewline(resp.Data["certificate"].(string))
}

func setupPrimaryCA(t *testing.T, client *vaultapi.Client, path string, rootPEM string) string {
	t.Helper()
	err := client.Sys().Mount(path, &vaultapi.MountInput{
		Type:        "pki",
		Description: "primary CA for Consul CA",
		Config: vaultapi.MountConfigInput{
			MaxLeaseTTL:     "2200h",
			DefaultLeaseTTL: "1h",
		},
	})
	require.NoError(t, err, "failed to mount")

	out, err := client.Logical().Write(path+"/intermediate/generate/internal", map[string]interface{}{
		"common_name": "primary CA",
		"ttl":         "2200h",
		"key_type":    "ec",
		"key_bits":    256,
	})
	require.NoError(t, err, "failed to generate root")

	intermediate, err := client.Logical().Write("corp/root/sign-intermediate", map[string]interface{}{
		"csr":            out.Data["csr"],
		"use_csr_values": true,
		"format":         "pem_bundle",
		"ttl":            "2200h",
	})
	require.NoError(t, err, "failed to sign intermediate")

	var buf strings.Builder
	buf.WriteString(lib.EnsureTrailingNewline(intermediate.Data["certificate"].(string)))
	buf.WriteString(lib.EnsureTrailingNewline(rootPEM))

	_, err = client.Logical().Write(path+"/intermediate/set-signed", map[string]interface{}{
		"certificate": buf.String(),
	})
	require.NoError(t, err, "failed to set signed intermediate")
	return lib.EnsureTrailingNewline(buf.String())
}
