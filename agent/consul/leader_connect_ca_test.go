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
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	ca "github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil"
)

// TODO(kyhavlov): replace with t.Deadline()
const CATestTimeout = 7 * time.Second

type mockCAServerDelegate struct {
	t           *testing.T
	config      *Config
	store       *state.Store
	primaryRoot *structs.CARoot
	callbackCh  chan string
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

func (m *mockCAServerDelegate) IsLeader() bool {
	return true
}

func (m *mockCAServerDelegate) CheckServers(datacenter string, fn func(*metadata.Server) bool) {
	ver, _ := version.NewVersion("1.6.0")
	fn(&metadata.Server{
		Status: serf.StatusAlive,
		Build:  *ver,
	})
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

	switch req.Op {
	case structs.CAOpSetConfig:
		if req.Config.ModifyIndex != 0 {
			act, err := m.store.CACheckAndSetConfig(idx+1, req.Config.ModifyIndex, req.Config)
			if err != nil {
				return nil, err
			}

			return act, nil
		}

		return nil, m.store.CASetConfig(idx+1, req.Config)
	case structs.CAOpSetRootsAndConfig:
		act, err := m.store.CARootSetCAS(idx, req.Index, req.Roots)
		if err != nil || !act {
			return act, err
		}

		act, err = m.store.CACheckAndSetConfig(idx+1, req.Config.ModifyIndex, req.Config)
		if err != nil {
			return nil, err
		}
		return act, nil
	case structs.CAOpSetProviderState:
		_, err := m.store.CASetProviderState(idx+1, req.ProviderState)
		if err != nil {
			return nil, err
		}

		return true, nil
	case structs.CAOpDeleteProviderState:
		if err := m.store.CADeleteProviderState(idx+1, req.ProviderState.ID); err != nil {
			return nil, err
		}

		return true, nil
	case structs.CAOpIncrementProviderSerialNumber:
		return uint64(2), nil
	default:
		return nil, fmt.Errorf("Invalid CA operation '%s'", req.Op)
	}
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
		*r = m.primaryRoot.RootCert
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
func (m *mockCAProvider) GenerateRoot() error                   { return nil }
func (m *mockCAProvider) ActiveRoot() (string, error) {
	return m.rootPEM, nil
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
		require.NoError(t, manager.InitializeCA())
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
	manager := NewCAManager(delegate, nil, testutil.Logger(t), conf)
	manager.providerShim = &mockCAProvider{
		callbackCh: delegate.callbackCh,
		rootPEM:    delegate.primaryRoot.RootCert,
	}

	// Call InitializeCA and then confirm the RPCs and provider calls
	// happen in the expected order.
	require.Equal(t, caStateUninitialized, manager.state)
	errCh := make(chan error)
	go func() {
		err := manager.InitializeCA()
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

	// Make sure the InitializeCA call returned successfully.
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
	delegate := NewMockCAServerDelegate(t, conf)
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

func TestCAManager_SignLeafWithExpiredCert(t *testing.T) {
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
			delegate := NewMockCAServerDelegate(t, conf)
			manager := NewCAManager(delegate, nil, testutil.Logger(t), conf)

			err, rootPEM := generatePem(arg.notBeforeRoot, arg.notAfterRoot)
			require.NoError(t, err)
			err, intermediatePEM := generatePem(arg.notBeforeIntermediate, arg.notAfterIntermediate)
			require.NoError(t, err)
			manager.providerShim = &mockCAProvider{
				callbackCh:      delegate.callbackCh,
				rootPEM:         rootPEM,
				intermediatePem: intermediatePEM,
			}
			initTestManager(t, manager, delegate)

			// Simulate Wait half the TTL for the cert to need renewing.
			manager.timeNow = func() time.Time {
				return time.Now().Add(500 * time.Millisecond)
			}

			// Call RenewIntermediate and then confirm the RPCs and provider calls
			// happen in the expected order.

			_, err = manager.SignCertificate(&x509.CertificateRequest{}, &connect.SpiffeIDAgent{})

			if arg.isError {
				require.Error(t, err)
				require.Contains(t, err.Error(), arg.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func generatePem(notBefore time.Time, notAfter time.Time) (error, string) {
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
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err, ""
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err, ""
	}
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	return err, caPEM.String()
}

func TestCADelegateWithState_GenerateCASignRequest(t *testing.T) {
	s := Server{config: &Config{PrimaryDatacenter: "east"}, tokens: new(token.Store)}
	d := &caDelegateWithState{Server: &s}
	req := d.generateCASignRequest("A")
	require.Equal(t, "east", req.RequestDatacenter())
}
