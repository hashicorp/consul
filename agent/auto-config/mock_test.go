// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package autoconf

import (
	"context"
	"crypto/x509"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/proto/private/pbautoconf"
	"github.com/hashicorp/consul/sdk/testutil"
)

type mockDirectRPC struct {
	mock.Mock
}

func newMockDirectRPC(t *testing.T) *mockDirectRPC {
	m := mockDirectRPC{}
	m.Test(t)
	return &m
}

func (m *mockDirectRPC) RPC(dc string, node string, addr net.Addr, method string, args interface{}, reply interface{}) error {
	var retValues mock.Arguments
	if method == "AutoConfig.InitialConfiguration" {
		req := args.(*pbautoconf.AutoConfigRequest)
		csr := req.CSR
		req.CSR = ""
		retValues = m.Called(dc, node, addr, method, args, reply)
		req.CSR = csr
	} else if method == "AutoEncrypt.Sign" {
		req := args.(*structs.CASignRequest)
		csr := req.CSR
		req.CSR = ""
		retValues = m.Called(dc, node, addr, method, args, reply)
		req.CSR = csr
	} else {
		retValues = m.Called(dc, node, addr, method, args, reply)
	}

	return retValues.Error(0)
}

type mockTLSConfigurator struct {
	mock.Mock
}

func newMockTLSConfigurator(t *testing.T) *mockTLSConfigurator {
	m := mockTLSConfigurator{}
	m.Test(t)
	return &m
}

func (m *mockTLSConfigurator) UpdateAutoTLS(manualCAPEMs, connectCAPEMs []string, pub, priv string, verifyServerHostname bool) error {
	if priv != "" {
		priv = "redacted"
	}

	ret := m.Called(manualCAPEMs, connectCAPEMs, pub, priv, verifyServerHostname)
	return ret.Error(0)
}

func (m *mockTLSConfigurator) UpdateAutoTLSCA(pems []string) error {
	ret := m.Called(pems)
	return ret.Error(0)
}

func (m *mockTLSConfigurator) UpdateAutoTLSCert(pub, priv string) error {
	if priv != "" {
		priv = "redacted"
	}
	ret := m.Called(pub, priv)
	return ret.Error(0)
}

func (m *mockTLSConfigurator) AutoEncryptCert() *x509.Certificate {
	ret := m.Called()
	cert, _ := ret.Get(0).(*x509.Certificate)
	return cert
}

type mockServerProvider struct {
	mock.Mock
}

func newMockServerProvider(t *testing.T) *mockServerProvider {
	m := mockServerProvider{}
	m.Test(t)
	return &m
}

func (m *mockServerProvider) FindLANServer() *metadata.Server {
	ret := m.Called()
	srv, _ := ret.Get(0).(*metadata.Server)
	return srv
}

type mockWatcher struct {
	ch   chan<- cache.UpdateEvent
	done <-chan struct{}
}

type mockLeafCerts struct {
	mock.Mock

	lock     sync.Mutex
	watchers map[string][]mockWatcher
}

var _ LeafCertManager = (*mockLeafCerts)(nil)

func newMockLeafCerts(t *testing.T) *mockLeafCerts {
	m := mockLeafCerts{
		watchers: make(map[string][]mockWatcher),
	}
	m.Test(t)
	return &m
}

func (m *mockLeafCerts) Notify(ctx context.Context, req *leafcert.ConnectCALeafRequest, correlationID string, ch chan<- cache.UpdateEvent) error {
	ret := m.Called(ctx, req, correlationID, ch)

	err := ret.Error(0)
	if err == nil {
		m.lock.Lock()
		key := req.Key()
		m.watchers[key] = append(m.watchers[key], mockWatcher{ch: ch, done: ctx.Done()})
		m.lock.Unlock()
	}
	return err
}

func (m *mockLeafCerts) Prepopulate(
	ctx context.Context,
	key string,
	index uint64,
	value *structs.IssuedCert,
	authorityKeyID string,
) error {
	// we cannot know what the private key is prior to it being injected into the cache.
	// therefore redact it here and all mock expectations should take that into account
	restore := value.PrivateKeyPEM
	value.PrivateKeyPEM = "redacted"

	ret := m.Called(ctx, key, index, value, authorityKeyID)

	if restore != "" {
		value.PrivateKeyPEM = restore
	}
	return ret.Error(0)
}

func (m *mockLeafCerts) sendNotification(ctx context.Context, key string, u cache.UpdateEvent) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	watchers, ok := m.watchers[key]
	if !ok || len(m.watchers) < 1 {
		return false
	}

	var newWatchers []mockWatcher

	for _, watcher := range watchers {
		select {
		case watcher.ch <- u:
			newWatchers = append(newWatchers, watcher)
		case <-watcher.done:
			// do nothing, this watcher will be removed from the list
		case <-ctx.Done():
			// return doesn't matter here really, the test is being cancelled
			return true
		}
	}

	// this removes any already cancelled watches from being sent to
	m.watchers[key] = newWatchers

	return true
}

type mockCache struct {
	mock.Mock

	lock     sync.Mutex
	watchers map[string][]mockWatcher
}

func newMockCache(t *testing.T) *mockCache {
	m := mockCache{
		watchers: make(map[string][]mockWatcher),
	}
	m.Test(t)
	return &m
}

func (m *mockCache) Notify(ctx context.Context, t string, r cache.Request, correlationID string, ch chan<- cache.UpdateEvent) error {
	ret := m.Called(ctx, t, r, correlationID, ch)

	err := ret.Error(0)
	if err == nil {
		m.lock.Lock()
		key := r.CacheInfo().Key
		m.watchers[key] = append(m.watchers[key], mockWatcher{ch: ch, done: ctx.Done()})
		m.lock.Unlock()
	}
	return err
}

func (m *mockCache) Prepopulate(t string, result cache.FetchResult, dc string, peerName string, token string, key string) error {
	var restore string
	cert, ok := result.Value.(*structs.IssuedCert)
	if ok {
		// we cannot know what the private key is prior to it being injected into the cache.
		// therefore redact it here and all mock expectations should take that into account
		restore = cert.PrivateKeyPEM
		cert.PrivateKeyPEM = "redacted"
	}

	ret := m.Called(t, result, dc, peerName, token, key)

	if ok && restore != "" {
		cert.PrivateKeyPEM = restore
	}
	return ret.Error(0)
}

func (m *mockCache) sendNotification(ctx context.Context, key string, u cache.UpdateEvent) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	watchers, ok := m.watchers[key]
	if !ok || len(m.watchers) < 1 {
		return false
	}

	var newWatchers []mockWatcher

	for _, watcher := range watchers {
		select {
		case watcher.ch <- u:
			newWatchers = append(newWatchers, watcher)
		case <-watcher.done:
			// do nothing, this watcher will be removed from the list
		case <-ctx.Done():
			// return doesn't matter here really, the test is being cancelled
			return true
		}
	}

	// this removes any already cancelled watches from being sent to
	m.watchers[key] = newWatchers

	return true
}

type mockTokenStore struct {
	mock.Mock
}

func newMockTokenStore(t *testing.T) *mockTokenStore {
	m := mockTokenStore{}
	m.Test(t)
	return &m
}

func (m *mockTokenStore) AgentToken() string {
	ret := m.Called()
	return ret.String(0)
}

func (m *mockTokenStore) UpdateAgentToken(secret string, source token.TokenSource) bool {
	return m.Called(secret, source).Bool(0)
}

func (m *mockTokenStore) Notify(kind token.TokenKind) token.Notifier {
	ret := m.Called(kind)
	n, _ := ret.Get(0).(token.Notifier)
	return n
}

func (m *mockTokenStore) StopNotify(notifier token.Notifier) {
	m.Called(notifier)
}

type mockedConfig struct {
	Config

	loader           *configLoader
	directRPC        *mockDirectRPC
	serverProvider   *mockServerProvider
	cache            *mockCache
	leafCerts        *mockLeafCerts
	tokens           *mockTokenStore
	tlsCfg           *mockTLSConfigurator
	enterpriseConfig *mockedEnterpriseConfig
}

func newMockedConfig(t *testing.T) *mockedConfig {
	loader := setupRuntimeConfig(t)
	directRPC := newMockDirectRPC(t)
	serverProvider := newMockServerProvider(t)
	mcache := newMockCache(t)
	mleafs := newMockLeafCerts(t)
	tokens := newMockTokenStore(t)
	tlsCfg := newMockTLSConfigurator(t)

	entConfig := newMockedEnterpriseConfig(t)

	// I am not sure it is well defined behavior but in testing it
	// out it does appear like Cleanup functions can fail tests
	// Adding in the mock expectations assertions here saves us
	// a bunch of code in the other test functions.
	t.Cleanup(func() {
		if !t.Failed() {
			directRPC.AssertExpectations(t)
			serverProvider.AssertExpectations(t)
			mleafs.AssertExpectations(t)
			mcache.AssertExpectations(t)
			tokens.AssertExpectations(t)
			tlsCfg.AssertExpectations(t)
		}
	})

	return &mockedConfig{
		Config: Config{
			Loader:           loader.Load,
			DirectRPC:        directRPC,
			ServerProvider:   serverProvider,
			Cache:            mcache,
			LeafCertManager:  mleafs,
			Tokens:           tokens,
			TLSConfigurator:  tlsCfg,
			Logger:           testutil.Logger(t),
			EnterpriseConfig: entConfig.EnterpriseConfig,
		},
		loader:         loader,
		directRPC:      directRPC,
		serverProvider: serverProvider,
		cache:          mcache,
		leafCerts:      mleafs,
		tokens:         tokens,
		tlsCfg:         tlsCfg,

		enterpriseConfig: entConfig,
	}
}

func (m *mockedConfig) expectInitialTLS(t *testing.T, agentName, datacenter, token string, ca *structs.CARoot, indexedRoots *structs.IndexedCARoots, cert *structs.IssuedCert, extraCerts []string) {
	var pems []string
	for _, root := range indexedRoots.Roots {
		pems = append(pems, root.RootCert)
	}
	for _, root := range indexedRoots.Roots {
		if len(root.IntermediateCerts) == 0 {
			root.IntermediateCerts = nil
		}
	}

	// we should update the TLS configurator with the proper certs
	m.tlsCfg.On("UpdateAutoTLS",
		extraCerts,
		pems,
		cert.CertPEM,
		// auto-config handles the CSR and Key so our tests don't have
		// a way to know that the key is correct or not. We do replace
		// a non empty PEM with "redacted" so we can ensure that some
		// certificate is being sent
		"redacted",
		true,
	).Return(nil).Once()

	rootRes := cache.FetchResult{Value: indexedRoots, Index: indexedRoots.QueryMeta.Index}
	rootsReq := structs.DCSpecificRequest{Datacenter: datacenter}

	// we should prepopulate the cache with the CA roots
	m.cache.On("Prepopulate",
		cachetype.ConnectCARootName,
		rootRes,
		datacenter,
		"",
		"",
		rootsReq.CacheInfo().Key,
	).Return(nil).Once()

	leafReq := leafcert.ConnectCALeafRequest{
		Token:      token,
		Agent:      agentName,
		Datacenter: datacenter,
	}

	// copy the cert and redact the private key for the mock expectation
	// the actual private key will not correspond to the cert but thats
	// because AutoConfig is generated a key/csr internally and sending that
	// on up with the request.
	copy := *cert
	copy.PrivateKeyPEM = "redacted"

	// we should prepopulate the cache with the agents cert
	m.leafCerts.On("Prepopulate",
		mock.Anything,
		leafReq.Key(),
		copy.RaftIndex.ModifyIndex,
		&copy,
		ca.SigningKeyID,
	).Return(nil).Once()

	// when prepopulating the cert in the cache we grab the token so
	// we should expect that here
	m.tokens.On("AgentToken").Return(token).Once()
}

func (m *mockedConfig) setupInitialTLS(t *testing.T, agentName, datacenter, token string) (*structs.IndexedCARoots, *structs.IssuedCert, []string) {
	ca, indexedRoots, cert := testCerts(t, agentName, datacenter)

	ca2 := connect.TestCA(t, nil)
	extraCerts := []string{ca2.RootCert}

	m.expectInitialTLS(t, agentName, datacenter, token, ca, indexedRoots, cert, extraCerts)
	return indexedRoots, cert, extraCerts
}
