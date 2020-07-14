package certmon

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockFallback struct {
	mock.Mock
}

func (m *mockFallback) fallback(ctx context.Context) (*structs.SignedResponse, error) {
	ret := m.Called()
	resp, _ := ret.Get(0).(*structs.SignedResponse)
	return resp, ret.Error(1)
}

type mockWatcher struct {
	ch   chan<- cache.UpdateEvent
	done <-chan struct{}
}

type mockCache struct {
	mock.Mock

	lock     sync.Mutex
	watchers map[string][]mockWatcher
}

func (m *mockCache) Notify(ctx context.Context, t string, r cache.Request, correlationID string, ch chan<- cache.UpdateEvent) error {
	m.lock.Lock()
	key := r.CacheInfo().Key
	m.watchers[key] = append(m.watchers[key], mockWatcher{ch: ch, done: ctx.Done()})
	m.lock.Unlock()
	ret := m.Called(t, r, correlationID)
	return ret.Error(0)
}

func (m *mockCache) Prepopulate(t string, result cache.FetchResult, dc string, token string, key string) error {
	ret := m.Called(t, result, dc, token, key)
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

func newMockCache(t *testing.T) *mockCache {
	mcache := mockCache{watchers: make(map[string][]mockWatcher)}
	mcache.Test(t)
	return &mcache
}

func waitForChan(timer *time.Timer, ch <-chan struct{}) bool {
	select {
	case <-timer.C:
		return false
	case <-ch:
		return true
	}
}

func waitForChans(timeout time.Duration, chans ...<-chan struct{}) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for _, ch := range chans {
		if !waitForChan(timer, ch) {
			return false
		}
	}
	return true
}

func testTLSConfigurator(t *testing.T) *tlsutil.Configurator {
	t.Helper()
	logger := testutil.Logger(t)
	cfg, err := tlsutil.NewConfigurator(tlsutil.Config{AutoEncryptTLS: true}, logger)
	require.NoError(t, err)
	return cfg
}

func newLeaf(t *testing.T, ca *structs.CARoot, idx uint64, expiration time.Duration) *structs.IssuedCert {
	t.Helper()

	pub, priv, err := connect.TestAgentLeaf(t, "node", "foo", ca, expiration)
	require.NoError(t, err)
	cert, err := connect.ParseCert(pub)
	require.NoError(t, err)

	spiffeID, err := connect.ParseCertURI(cert.URIs[0])
	require.NoError(t, err)

	agentID, ok := spiffeID.(*connect.SpiffeIDAgent)
	require.True(t, ok, "certificate doesn't have an agent leaf cert URI")

	return &structs.IssuedCert{
		SerialNumber:   cert.SerialNumber.String(),
		CertPEM:        pub,
		PrivateKeyPEM:  priv,
		ValidAfter:     cert.NotBefore,
		ValidBefore:    cert.NotAfter,
		Agent:          agentID.Agent,
		AgentURI:       agentID.URI().String(),
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
		RaftIndex: structs.RaftIndex{
			CreateIndex: idx,
			ModifyIndex: idx,
		},
	}
}

type testCertMonitor struct {
	monitor  *CertMonitor
	mcache   *mockCache
	tls      *tlsutil.Configurator
	tokens   *token.Store
	fallback *mockFallback

	extraCACerts []string
	initialCert  *structs.IssuedCert
	initialRoots *structs.IndexedCARoots

	// these are some variables that the CertMonitor was created with
	datacenter           string
	nodeName             string
	dns                  []string
	ips                  []net.IP
	verifyServerHostname bool
}

func newTestCertMonitor(t *testing.T) testCertMonitor {
	t.Helper()

	tlsConfigurator := testTLSConfigurator(t)
	tokens := new(token.Store)

	id, err := uuid.GenerateUUID()
	require.NoError(t, err)
	tokens.UpdateAgentToken(id, token.TokenSourceConfig)

	ca := connect.TestCA(t, nil)
	manualCA := connect.TestCA(t, nil)
	// this cert is setup to not expire quickly. this will prevent
	// the test from accidentally running the fallback routine
	// before we want to force that to happen.
	issued := newLeaf(t, ca, 1, 10*time.Minute)

	indexedRoots := structs.IndexedCARoots{
		ActiveRootID: ca.ID,
		TrustDomain:  connect.TestClusterID,
		Roots: []*structs.CARoot{
			ca,
		},
		QueryMeta: structs.QueryMeta{
			Index: 1,
		},
	}

	initialCerts := &structs.SignedResponse{
		ConnectCARoots:       indexedRoots,
		IssuedCert:           *issued,
		ManualCARoots:        []string{manualCA.RootCert},
		VerifyServerHostname: true,
	}

	dnsSANs := []string{"test.dev"}
	ipSANs := []net.IP{net.IPv4(198, 18, 0, 1)}

	// this chan should be unbuffered so we can detect when the fallback func has been called.
	fallback := &mockFallback{}

	mcache := newMockCache(t)
	rootRes := cache.FetchResult{Value: &indexedRoots, Index: 1}
	rootsReq := structs.DCSpecificRequest{Datacenter: "foo"}
	mcache.On("Prepopulate", cachetype.ConnectCARootName, rootRes, "foo", "", rootsReq.CacheInfo().Key).Return(nil).Once()

	leafReq := cachetype.ConnectCALeafRequest{
		Token:      tokens.AgentToken(),
		Agent:      "node",
		Datacenter: "foo",
		DNSSAN:     dnsSANs,
		IPSAN:      ipSANs,
	}
	leafRes := cache.FetchResult{
		Value: issued,
		Index: 1,
		State: cachetype.ConnectCALeafSuccess(ca.SigningKeyID),
	}
	mcache.On("Prepopulate", cachetype.ConnectCALeafName, leafRes, "foo", tokens.AgentToken(), leafReq.Key()).Return(nil).Once()

	// we can assert more later but this should always be done.
	defer mcache.AssertExpectations(t)

	cfg := new(Config).
		WithCache(mcache).
		WithLogger(testutil.Logger(t)).
		WithTLSConfigurator(tlsConfigurator).
		WithTokens(tokens).
		WithFallback(fallback.fallback).
		WithDNSSANs(dnsSANs).
		WithIPSANs(ipSANs).
		WithDatacenter("foo").
		WithNodeName("node").
		WithFallbackLeeway(time.Nanosecond).
		WithFallbackRetry(time.Millisecond)

	monitor, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, monitor)

	require.NoError(t, monitor.Update(initialCerts))

	return testCertMonitor{
		monitor:              monitor,
		tls:                  tlsConfigurator,
		tokens:               tokens,
		mcache:               mcache,
		fallback:             fallback,
		extraCACerts:         []string{manualCA.RootCert},
		initialCert:          issued,
		initialRoots:         &indexedRoots,
		datacenter:           "foo",
		nodeName:             "node",
		dns:                  dnsSANs,
		ips:                  ipSANs,
		verifyServerHostname: true,
	}
}

func tlsCertificateFromIssued(t *testing.T, issued *structs.IssuedCert) *tls.Certificate {
	t.Helper()

	cert, err := tls.X509KeyPair([]byte(issued.CertPEM), []byte(issued.PrivateKeyPEM))
	require.NoError(t, err)
	return &cert
}

// convenience method to get a TLS Certificate from the intial issued certificate and priv key
func (cm *testCertMonitor) initialTLSCertificate(t *testing.T) *tls.Certificate {
	t.Helper()
	return tlsCertificateFromIssued(t, cm.initialCert)
}

// just a convenience method to get a list of all the CA pems that we set up regardless
// of manual vs connect.
func (cm *testCertMonitor) initialCACerts() []string {
	pems := cm.extraCACerts
	for _, root := range cm.initialRoots.Roots {
		pems = append(pems, root.RootCert)
	}
	return pems
}

func (cm *testCertMonitor) assertExpectations(t *testing.T) {
	cm.mcache.AssertExpectations(t)
	cm.fallback.AssertExpectations(t)
}

func TestCertMonitor_InitialCerts(t *testing.T) {
	// this also ensures that the cache was prepopulated properly
	cm := newTestCertMonitor(t)

	// verify that the certificate was injected into the TLS configurator correctly
	require.Equal(t, cm.initialTLSCertificate(t), cm.tls.Cert())
	// verify that the CA certs (both Connect and manual ones) were injected correctly
	require.ElementsMatch(t, cm.initialCACerts(), cm.tls.CAPems())
	// verify that the auto-tls verify server hostname setting was injected correctly
	require.Equal(t, cm.verifyServerHostname, cm.tls.VerifyServerHostname())
}

func TestCertMonitor_GoRoutineManagement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cm := newTestCertMonitor(t)

	// ensure that the monitor is not running
	require.False(t, cm.monitor.IsRunning())

	// ensure that nothing bad happens and that it reports as stopped
	require.False(t, cm.monitor.Stop())

	// we will never send notifications so these just ignore everything
	cm.mcache.On("Notify", cachetype.ConnectCARootName, &structs.DCSpecificRequest{Datacenter: cm.datacenter}, rootsWatchID).Return(nil).Times(2)
	cm.mcache.On("Notify", cachetype.ConnectCALeafName,
		&cachetype.ConnectCALeafRequest{
			Token:      cm.tokens.AgentToken(),
			Datacenter: cm.datacenter,
			Agent:      cm.nodeName,
			DNSSAN:     cm.dns,
			IPSAN:      cm.ips,
		},
		leafWatchID,
	).Return(nil).Times(2)

	done, err := cm.monitor.Start(ctx)
	require.NoError(t, err)
	require.True(t, cm.monitor.IsRunning())
	_, err = cm.monitor.Start(ctx)
	testutil.RequireErrorContains(t, err, "the CertMonitor is already running")
	require.True(t, cm.monitor.Stop())

	require.True(t, waitForChans(100*time.Millisecond, done), "monitor didn't shut down")
	require.False(t, cm.monitor.IsRunning())
	done, err = cm.monitor.Start(ctx)
	require.NoError(t, err)

	// ensure that context cancellation causes us to stop as well
	cancel()
	require.True(t, waitForChans(100*time.Millisecond, done))

	cm.assertExpectations(t)
}

func startedCertMonitor(t *testing.T) (context.Context, testCertMonitor) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cm := newTestCertMonitor(t)

	rootsCtx, rootsCancel := context.WithCancel(ctx)
	defer rootsCancel()
	leafCtx, leafCancel := context.WithCancel(ctx)
	defer leafCancel()

	// initial roots watch
	cm.mcache.On("Notify", cachetype.ConnectCARootName,
		&structs.DCSpecificRequest{
			Datacenter: cm.datacenter,
		},
		rootsWatchID).
		Return(nil).
		Once().
		Run(func(_ mock.Arguments) {
			rootsCancel()
		})
	// the initial watch after starting the monitor
	cm.mcache.On("Notify", cachetype.ConnectCALeafName,
		&cachetype.ConnectCALeafRequest{
			Token:      cm.tokens.AgentToken(),
			Datacenter: cm.datacenter,
			Agent:      cm.nodeName,
			DNSSAN:     cm.dns,
			IPSAN:      cm.ips,
		},
		leafWatchID).
		Return(nil).
		Once().
		Run(func(_ mock.Arguments) {
			leafCancel()
		})

	done, err := cm.monitor.Start(ctx)
	require.NoError(t, err)
	// this prevents logs after the test finishes
	t.Cleanup(func() {
		cm.monitor.Stop()
		<-done
	})

	require.True(t,
		waitForChans(100*time.Millisecond, rootsCtx.Done(), leafCtx.Done()),
		"not all watches were started within the alotted time")

	return ctx, cm
}

// This test ensures that the cache watches are restarted with the updated
// token after receiving a token update
func TestCertMonitor_TokenUpdate(t *testing.T) {
	ctx, cm := startedCertMonitor(t)

	rootsCtx, rootsCancel := context.WithCancel(ctx)
	defer rootsCancel()
	leafCtx, leafCancel := context.WithCancel(ctx)
	defer leafCancel()

	newToken := "8e4fe8db-162d-42d8-81ca-710fb2280ad0"

	// we expect a new roots watch because when the leaf cert watch is restarted so is the root cert watch
	cm.mcache.On("Notify", cachetype.ConnectCARootName,
		&structs.DCSpecificRequest{
			Datacenter: cm.datacenter,
		},
		rootsWatchID).
		Return(nil).
		Once().
		Run(func(_ mock.Arguments) {
			rootsCancel()
		})

	secondWatch := &cachetype.ConnectCALeafRequest{
		Token:      newToken,
		Datacenter: cm.datacenter,
		Agent:      cm.nodeName,
		DNSSAN:     cm.dns,
		IPSAN:      cm.ips,
	}
	// the new watch after updating the token
	cm.mcache.On("Notify", cachetype.ConnectCALeafName, secondWatch, leafWatchID).
		Return(nil).
		Once().
		Run(func(args mock.Arguments) {
			leafCancel()
		})

	cm.tokens.UpdateAgentToken(newToken, token.TokenSourceAPI)

	require.True(t,
		waitForChans(100*time.Millisecond, rootsCtx.Done(), leafCtx.Done()),
		"not all watches were restarted within the alotted time")

	cm.assertExpectations(t)
}

func TestCertMonitor_RootsUpdate(t *testing.T) {
	ctx, cm := startedCertMonitor(t)

	secondCA := connect.TestCA(t, cm.initialRoots.Roots[0])
	secondRoots := structs.IndexedCARoots{
		ActiveRootID: secondCA.ID,
		TrustDomain:  connect.TestClusterID,
		Roots: []*structs.CARoot{
			secondCA,
			cm.initialRoots.Roots[0],
		},
		QueryMeta: structs.QueryMeta{
			Index: 99,
		},
	}

	// assert value of the CA certs prior to updating
	require.ElementsMatch(t, cm.initialCACerts(), cm.tls.CAPems())

	req := structs.DCSpecificRequest{Datacenter: cm.datacenter}
	require.True(t, cm.mcache.sendNotification(ctx, req.CacheInfo().Key, cache.UpdateEvent{
		CorrelationID: rootsWatchID,
		Result:        &secondRoots,
		Meta: cache.ResultMeta{
			Index: secondRoots.Index,
		},
	}))

	expectedCAs := append(cm.extraCACerts, secondCA.RootCert, cm.initialRoots.Roots[0].RootCert)

	// this will wait up to 200ms (8 x 25 ms waits between the 9 requests)
	retry.RunWith(&retry.Counter{Count: 9, Wait: 25 * time.Millisecond}, t, func(r *retry.R) {
		require.ElementsMatch(r, expectedCAs, cm.tls.CAPems())
	})

	cm.assertExpectations(t)
}

func TestCertMonitor_CertUpdate(t *testing.T) {
	ctx, cm := startedCertMonitor(t)

	secondCert := newLeaf(t, cm.initialRoots.Roots[0], 100, 10*time.Minute)

	// assert value of cert prior to updating the leaf
	require.Equal(t, cm.initialTLSCertificate(t), cm.tls.Cert())

	key := cm.monitor.leafReq.CacheInfo().Key

	// send the new certificate - this notifies only the watchers utilizing
	// the new ACL token
	require.True(t, cm.mcache.sendNotification(ctx, key, cache.UpdateEvent{
		CorrelationID: leafWatchID,
		Result:        secondCert,
		Meta: cache.ResultMeta{
			Index: secondCert.ModifyIndex,
		},
	}))

	tlsCert := tlsCertificateFromIssued(t, secondCert)

	// this will wait up to 200ms (8 x 25 ms waits between the 9 requests)
	retry.RunWith(&retry.Counter{Count: 9, Wait: 25 * time.Millisecond}, t, func(r *retry.R) {
		require.Equal(r, tlsCert, cm.tls.Cert())
	})

	cm.assertExpectations(t)
}

func TestCertMonitor_Fallback(t *testing.T) {
	ctx, cm := startedCertMonitor(t)

	// at this point everything is operating normally and the monitor is just
	// waiting for events. We are going to send a new cert that is basically
	// already expired and then allow the fallback routine to kick in.
	secondCert := newLeaf(t, cm.initialRoots.Roots[0], 100, time.Nanosecond)
	secondCA := connect.TestCA(t, cm.initialRoots.Roots[0])
	secondRoots := structs.IndexedCARoots{
		ActiveRootID: secondCA.ID,
		TrustDomain:  connect.TestClusterID,
		Roots: []*structs.CARoot{
			secondCA,
			cm.initialRoots.Roots[0],
		},
		QueryMeta: structs.QueryMeta{
			Index: 101,
		},
	}
	thirdCert := newLeaf(t, secondCA, 102, 10*time.Minute)

	// inject a fallback routine error to check that we rerun it quickly
	cm.fallback.On("fallback").Return(nil, fmt.Errorf("induced error")).Once()

	// expect the fallback routine to be executed and setup the return
	cm.fallback.On("fallback").Return(&structs.SignedResponse{
		ConnectCARoots:       secondRoots,
		IssuedCert:           *thirdCert,
		ManualCARoots:        cm.extraCACerts,
		VerifyServerHostname: true,
	}, nil).Once()

	// Add another roots cache prepopulation expectation which should happen
	// in response to executing the fallback mechanism
	rootRes := cache.FetchResult{Value: &secondRoots, Index: 101}
	rootsReq := structs.DCSpecificRequest{Datacenter: cm.datacenter}
	cm.mcache.On("Prepopulate", cachetype.ConnectCARootName, rootRes, cm.datacenter, "", rootsReq.CacheInfo().Key).Return(nil).Once()

	// add another leaf cert cache prepopulation expectation which should happen
	// in response to executing the fallback mechanism
	leafReq := cachetype.ConnectCALeafRequest{
		Token:      cm.tokens.AgentToken(),
		Agent:      cm.nodeName,
		Datacenter: cm.datacenter,
		DNSSAN:     cm.dns,
		IPSAN:      cm.ips,
	}
	leafRes := cache.FetchResult{
		Value: thirdCert,
		Index: 101,
		State: cachetype.ConnectCALeafSuccess(secondCA.SigningKeyID),
	}
	cm.mcache.On("Prepopulate", cachetype.ConnectCALeafName, leafRes, leafReq.Datacenter, leafReq.Token, leafReq.Key()).Return(nil).Once()

	// nothing in the monitor should be looking at this as its only done
	// in response to sending token updates, no need to synchronize
	key := cm.monitor.leafReq.CacheInfo().Key
	// send the new certificate - this notifies only the watchers utilizing
	// the new ACL token
	require.True(t, cm.mcache.sendNotification(ctx, key, cache.UpdateEvent{
		CorrelationID: leafWatchID,
		Result:        secondCert,
		Meta: cache.ResultMeta{
			Index: secondCert.ModifyIndex,
		},
	}))

	// if all went well we would have updated the first certificate which was pretty much expired
	// causing the fallback handler to be invoked almost immediately. The fallback routine will
	// return the response containing the third cert and second CA roots so now we should wait
	// a little while and ensure they were applied to the TLS Configurator
	tlsCert := tlsCertificateFromIssued(t, thirdCert)
	expectedCAs := append(cm.extraCACerts, secondCA.RootCert, cm.initialRoots.Roots[0].RootCert)

	// this will wait up to 200ms (8 x 25 ms waits between the 9 requests)
	retry.RunWith(&retry.Counter{Count: 9, Wait: 25 * time.Millisecond}, t, func(r *retry.R) {
		require.Equal(r, tlsCert, cm.tls.Cert())
		require.ElementsMatch(r, expectedCAs, cm.tls.CAPems())
	})

	cm.assertExpectations(t)
}

func TestCertMonitor_New_Errors(t *testing.T) {
	type testCase struct {
		cfg Config
		err string
	}

	fallback := func(_ context.Context) (*structs.SignedResponse, error) {
		return nil, fmt.Errorf("Unimplemented")
	}

	tokens := new(token.Store)

	cases := map[string]testCase{
		"no-cache": {
			cfg: Config{
				TLSConfigurator: testTLSConfigurator(t),
				Fallback:        fallback,
				Tokens:          tokens,
				Datacenter:      "foo",
				NodeName:        "bar",
			},
			err: "CertMonitor creation requires a Cache",
		},
		"no-tls-configurator": {
			cfg: Config{
				Cache:      cache.New(nil),
				Fallback:   fallback,
				Tokens:     tokens,
				Datacenter: "foo",
				NodeName:   "bar",
			},
			err: "CertMonitor creation requires a TLS Configurator",
		},
		"no-fallback": {
			cfg: Config{
				Cache:           cache.New(nil),
				TLSConfigurator: testTLSConfigurator(t),
				Tokens:          tokens,
				Datacenter:      "foo",
				NodeName:        "bar",
			},
			err: "CertMonitor creation requires specifying a FallbackFunc",
		},
		"no-tokens": {
			cfg: Config{
				Cache:           cache.New(nil),
				TLSConfigurator: testTLSConfigurator(t),
				Fallback:        fallback,
				Datacenter:      "foo",
				NodeName:        "bar",
			},
			err: "CertMonitor creation requires specifying a token store",
		},
		"no-datacenter": {
			cfg: Config{
				Cache:           cache.New(nil),
				TLSConfigurator: testTLSConfigurator(t),
				Fallback:        fallback,
				Tokens:          tokens,
				NodeName:        "bar",
			},
			err: "CertMonitor creation requires specifying the datacenter",
		},
		"no-node-name": {
			cfg: Config{
				Cache:           cache.New(nil),
				TLSConfigurator: testTLSConfigurator(t),
				Fallback:        fallback,
				Tokens:          tokens,
				Datacenter:      "foo",
			},
			err: "CertMonitor creation requires specifying the agent's node name",
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			monitor, err := New(&tcase.cfg)
			testutil.RequireErrorContains(t, err, tcase.err)
			require.Nil(t, monitor)
		})
	}
}
