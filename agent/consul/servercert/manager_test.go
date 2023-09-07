// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package servercert

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/sdk/testutil"
)

type fakeStore struct {
	// conf is the current CA configuration stored in the fakeStore.
	conf chan *structs.CAConfiguration

	// tokenEntry is the current server token entry stored in the fakeStore.
	tokenEntry chan *structs.SystemMetadataEntry

	// tokenCanceler will unblock the WatchSet for the token entry.
	tokenCanceler <-chan struct{}
}

func (s *fakeStore) CAConfig(_ memdb.WatchSet) (uint64, *structs.CAConfiguration, error) {
	select {
	case conf := <-s.conf:
		return 0, conf, nil
	default:
		return 0, nil, nil
	}
}

func (s *fakeStore) setCAConfig() {
	s.conf <- &structs.CAConfiguration{
		ClusterID: connect.TestClusterID,
	}
}

func (s *fakeStore) SystemMetadataGet(ws memdb.WatchSet, _ string) (uint64, *structs.SystemMetadataEntry, error) {
	select {
	case entry := <-s.tokenEntry:
		ws.Add(s.tokenCanceler)
		return 0, entry, nil
	default:
		return 0, nil, nil
	}
}

func (s *fakeStore) setServerToken(token string, canceler <-chan struct{}) {
	s.tokenCanceler = canceler
	s.tokenEntry <- &structs.SystemMetadataEntry{
		Key:   structs.ServerManagementTokenAccessorID,
		Value: token,
	}
}

func (s *fakeStore) AbandonCh() <-chan struct{} {
	return make(<-chan struct{})
}

type testCert struct {
	pub  string
	priv string
}

type fakeTLSConfigurator struct {
	cert              testCert
	peeringServerName string

	// syncCh is used to signal that an update was handled.
	// It synchronizes readers and writers in different goroutines.
	syncCh chan struct{}
}

func (u *fakeTLSConfigurator) UpdateAutoTLSCert(pub, priv string) error {
	u.cert = testCert{
		pub:  pub,
		priv: priv,
	}
	u.syncCh <- struct{}{}
	return nil
}

func (u *fakeTLSConfigurator) UpdateAutoTLSPeeringServerName(name string) {
	u.peeringServerName = name
	u.syncCh <- struct{}{}
}

func (u *fakeTLSConfigurator) timeoutIfNotUpdated(t *testing.T) error {
	t.Helper()

	select {
	case <-u.syncCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timed out")
	}
	return nil
}

type watchInfo struct {
	ctx   context.Context
	token string
}

type fakeLeafCertManager struct {
	updateCh chan<- cache.UpdateEvent

	// watched is a map of watched correlation IDs to the ACL token of the request.
	watched map[string]watchInfo

	// syncCh is used to signal that Notify was called.
	// It synchronizes readers and writers in different goroutines.
	syncCh chan struct{}
}

func (c *fakeLeafCertManager) triggerLeafUpdate() {
	c.updateCh <- cache.UpdateEvent{
		CorrelationID: leafWatchID,
		Result: &structs.IssuedCert{
			CertPEM:       "cert-pem",
			PrivateKeyPEM: "key-pem",
			ServerURI:     "test-uri",
		},
	}
}

func (c *fakeLeafCertManager) Notify(ctx context.Context, r *leafcert.ConnectCALeafRequest, correlationID string, ch chan<- cache.UpdateEvent) error {
	c.watched[correlationID] = watchInfo{ctx: ctx, token: r.Token}
	c.updateCh = ch
	c.syncCh <- struct{}{}
	return nil
}

func (c *fakeLeafCertManager) timeoutIfNotUpdated(t *testing.T) error {
	t.Helper()

	select {
	case <-c.syncCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timed out")
	}
	return nil
}

func testWaiter() retry.Waiter {
	return retry.Waiter{
		MinFailures: 1,
		MinWait:     20 * time.Millisecond,
		MaxWait:     20 * time.Millisecond,
	}
}

func TestCertManager_ACLsDisabled(t *testing.T) {
	tlsConfigurator := fakeTLSConfigurator{syncCh: make(chan struct{}, 1)}
	leafCerts := fakeLeafCertManager{watched: make(map[string]watchInfo), syncCh: make(chan struct{}, 1)}
	store := fakeStore{
		conf:       make(chan *structs.CAConfiguration, 1),
		tokenEntry: make(chan *structs.SystemMetadataEntry, 1),
	}

	mgr := NewCertManager(Deps{
		Logger: testutil.Logger(t),
		Config: Config{
			Datacenter:  "my-dc",
			ACLsEnabled: false,
		},
		TLSConfigurator: &tlsConfigurator,
		LeafCertManager: &leafCerts,
		GetStore:        func() Store { return &store },
	})

	// Override the default waiter to reduce time between retries.
	mgr.waiter = testWaiter()

	require.NoError(t, mgr.Start(context.Background()))

	testutil.RunStep(t, "initial empty state", func(t *testing.T) {
		require.Empty(t, tlsConfigurator.cert)
		require.Empty(t, tlsConfigurator.peeringServerName)

		require.Contains(t, leafCerts.watched, leafWatchID)
	})

	testutil.RunStep(t, "leaf cert update", func(t *testing.T) {
		leafCerts.triggerLeafUpdate()

		// Wait for the update to arrive.
		require.NoError(t, tlsConfigurator.timeoutIfNotUpdated(t))

		expect := testCert{
			pub:  "cert-pem",
			priv: "key-pem",
		}
		require.Equal(t, expect, tlsConfigurator.cert)
	})

	testutil.RunStep(t, "ca config update", func(t *testing.T) {
		store.setCAConfig()

		// Wait for the update to arrive.
		require.NoError(t, tlsConfigurator.timeoutIfNotUpdated(t))

		expect := connect.PeeringServerSAN(mgr.config.Datacenter, connect.TestTrustDomain)
		require.Equal(t, expect, tlsConfigurator.peeringServerName)
	})
}

func TestCertManager_ACLsEnabled(t *testing.T) {
	tlsConfigurator := fakeTLSConfigurator{syncCh: make(chan struct{}, 1)}
	leafCerts := fakeLeafCertManager{watched: make(map[string]watchInfo), syncCh: make(chan struct{}, 1)}
	store := fakeStore{
		conf:       make(chan *structs.CAConfiguration, 1),
		tokenEntry: make(chan *structs.SystemMetadataEntry, 1),
	}

	mgr := NewCertManager(Deps{
		Logger: testutil.Logger(t),
		Config: Config{
			Datacenter:  "my-dc",
			ACLsEnabled: true,
		},
		TLSConfigurator: &tlsConfigurator,
		LeafCertManager: &leafCerts,
		GetStore:        func() Store { return &store },
	})

	// Override the default waiter to reduce time between retries.
	mgr.waiter = testWaiter()

	require.NoError(t, mgr.Start(context.Background()))

	testutil.RunStep(t, "initial empty state", func(t *testing.T) {
		require.Empty(t, tlsConfigurator.cert)
		require.Empty(t, tlsConfigurator.peeringServerName)

		require.Empty(t, leafCerts.watched)
	})

	var leafCtx context.Context
	tokenCanceler := make(chan struct{})

	testutil.RunStep(t, "server token update", func(t *testing.T) {
		store.setServerToken("first-secret", tokenCanceler)

		require.NoError(t, leafCerts.timeoutIfNotUpdated(t))

		require.Contains(t, leafCerts.watched, leafWatchID)
		require.Equal(t, "first-secret", leafCerts.watched[leafWatchID].token)

		leafCtx = leafCerts.watched[leafWatchID].ctx
	})

	testutil.RunStep(t, "leaf cert update", func(t *testing.T) {
		leafCerts.triggerLeafUpdate()

		// Wait for the update to arrive.
		require.NoError(t, tlsConfigurator.timeoutIfNotUpdated(t))

		expect := testCert{
			pub:  "cert-pem",
			priv: "key-pem",
		}
		require.Equal(t, expect, tlsConfigurator.cert)
	})

	testutil.RunStep(t, "another server token update", func(t *testing.T) {
		store.setServerToken("second-secret", nil)

		// Fire the existing WatchSet to simulate a state store update.
		tokenCanceler <- struct{}{}

		// The leaf watch in the leafCerts should have been reset.
		require.NoError(t, leafCerts.timeoutIfNotUpdated(t))

		// The original leaf watch context should have been canceled.
		require.Error(t, leafCtx.Err())

		// A new leaf watch is expected with the new token.
		require.Contains(t, leafCerts.watched, leafWatchID)
		require.Equal(t, "second-secret", leafCerts.watched[leafWatchID].token)
	})

	testutil.RunStep(t, "ca config update", func(t *testing.T) {
		store.setCAConfig()

		// Wait for the update to arrive.
		require.NoError(t, tlsConfigurator.timeoutIfNotUpdated(t))

		expect := connect.PeeringServerSAN(mgr.config.Datacenter, connect.TestTrustDomain)
		require.Equal(t, expect, tlsConfigurator.peeringServerName)
	})
}
