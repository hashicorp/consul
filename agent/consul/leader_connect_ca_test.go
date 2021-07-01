package consul

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	ca "github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
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

func (m *mockCAServerDelegate) ApplyCARequest(req *structs.CARequest) (interface{}, error) {
	return ca.ApplyCARequestToStore(m.store, req)
}

func (m *mockCAServerDelegate) createCAProvider(conf *structs.CAConfiguration) (ca.Provider, error) {
	return &mockCAProvider{
		callbackCh: m.callbackCh,
		rootPEM:    m.primaryRoot.RootCert,
	}, nil
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

func (m *mockCAServerDelegate) raftApply(t structs.MessageType, msg interface{}) (interface{}, error) {
	if t == structs.ConnectCARequestType {
		req := msg.(*structs.CARequest)
		act, err := m.store.CARootSetCAS(1, req.Index, req.Roots)
		require.NoError(m.t, err)
		require.True(m.t, act)

		act, err = m.store.CACheckAndSetConfig(1, req.Config.ModifyIndex, req.Config)
		require.NoError(m.t, err)
		require.True(m.t, act)
	} else {
		return nil, fmt.Errorf("got invalid MessageType %v", t)
	}
	m.callbackCh <- fmt.Sprintf("raftApply/%s", t)
	return nil, nil
}

// mockCAProvider mocks an empty provider implementation with a channel in order to coordinate
// waiting for certain methods to be called.
type mockCAProvider struct {
	callbackCh chan string
	rootPEM    string
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
	return m.rootPEM, nil
}
func (m *mockCAProvider) GenerateIntermediate() (string, error)                     { return "", nil }
func (m *mockCAProvider) Sign(*x509.CertificateRequest) (string, error)             { return "", nil }
func (m *mockCAProvider) SignIntermediate(*x509.CertificateRequest) (string, error) { return "", nil }
func (m *mockCAProvider) CrossSignCA(*x509.Certificate) (string, error)             { return "", nil }
func (m *mockCAProvider) SupportsCrossSigning() (bool, error)                       { return false, nil }
func (m *mockCAProvider) Cleanup(_ bool, _ map[string]interface{}) error            { return nil }

func waitForCh(t *testing.T, ch chan string, expected string) {
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

	// Call InitializeCA and then confirm the RPCs and provider calls
	// happen in the expected order.
	require.EqualValues(t, caStateUninitialized, manager.state)
	errCh := make(chan error)
	go func() {
		errCh <- manager.InitializeCA()
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

	require.EqualValues(t, caStateInitialized, manager.state)
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
	initTestManager(t, manager, delegate)

	// Wait half the TTL for the cert to need renewing.
	time.Sleep(500 * time.Millisecond)

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
