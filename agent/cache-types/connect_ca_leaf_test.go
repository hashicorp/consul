package cachetype

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCalculateSoftExpire(t *testing.T) {
	tests := []struct {
		name     string
		now      string
		issued   string
		lifetime time.Duration
		wantMin  string
		wantMax  string
	}{
		{
			name:     "72h just issued",
			now:      "2018-01-01 00:00:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Should jitter between 60% and 90% of the lifetime which is 43.2/64.8
			// hours after issued
			wantMin: "2018-01-02 19:12:00",
			wantMax: "2018-01-03 16:48:00",
		},
		{
			name: "72h in renew range",
			// This time should be inside the renewal range.
			now:      "2018-01-02 20:00:20",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Min should be the "now" time
			wantMin: "2018-01-02 20:00:20",
			wantMax: "2018-01-03 16:48:00",
		},
		{
			name: "72h in hard renew",
			// This time should be inside the renewal range.
			now:      "2018-01-03 18:00:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-03 18:00:00",
			wantMax: "2018-01-03 18:00:00",
		},
		{
			name: "72h expired",
			// This time is after expiry
			now:      "2018-01-05 00:00:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-05 00:00:00",
			wantMax: "2018-01-05 00:00:00",
		},
		{
			name:     "1h just issued",
			now:      "2018-01-01 00:00:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Should jitter between 60% and 90% of the lifetime which is 36/54 mins
			// hours after issued
			wantMin: "2018-01-01 00:36:00",
			wantMax: "2018-01-01 00:54:00",
		},
		{
			name: "1h in renew range",
			// This time should be inside the renewal range.
			now:      "2018-01-01 00:40:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Min should be the "now" time
			wantMin: "2018-01-01 00:40:00",
			wantMax: "2018-01-01 00:54:00",
		},
		{
			name: "1h in hard renew",
			// This time should be inside the renewal range.
			now:      "2018-01-01 00:55:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-01 00:55:00",
			wantMax: "2018-01-01 00:55:00",
		},
		{
			name: "1h expired",
			// This time is after expiry
			now:      "2018-01-01 01:01:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-01 01:01:01",
			wantMax: "2018-01-01 01:01:01",
		},
		{
			name: "too short lifetime",
			// This time is after expiry
			now:      "2018-01-01 01:01:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Minute,
			// Min and max should both be the "now" time
			wantMin: "2018-01-01 01:01:01",
			wantMax: "2018-01-01 01:01:01",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)
			now, err := time.Parse("2006-01-02 15:04:05", tc.now)
			require.NoError(err)
			issued, err := time.Parse("2006-01-02 15:04:05", tc.issued)
			require.NoError(err)
			wantMin, err := time.Parse("2006-01-02 15:04:05", tc.wantMin)
			require.NoError(err)
			wantMax, err := time.Parse("2006-01-02 15:04:05", tc.wantMax)
			require.NoError(err)

			min, max := calculateSoftExpiry(now, &structs.IssuedCert{
				ValidAfter:  issued,
				ValidBefore: issued.Add(tc.lifetime),
			})

			require.Equal(wantMin, min)
			require.Equal(wantMax, max)
		})
	}
}

// Test that after an initial signing, new CA roots (new ID) will
// trigger a blocking query to execute.
func TestConnectCALeaf_changingRoots(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)

	// Override the root-change jitter so we don't have to wait up to 20 seconds
	// to see root changes work.
	typ.testSetCAChangeInitialJitter = 1 * time.Millisecond

	caRoot := connect.TestCA(t, nil)
	caRoot.Active = true
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign to return signed cert
	var resp *structs.IssuedCert
	var idx uint64
	rpc.On("RPC", "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			reply := args.Get(2).(*structs.IssuedCert)
			leaf, _ := connect.TestLeaf(t, "web", caRoot)
			reply.CertPEM = leaf
			reply.ValidAfter = time.Now().Add(-1 * time.Hour)
			reply.ValidBefore = time.Now().Add(11 * time.Hour)
			reply.CreateIndex = atomic.AddUint64(&idx, 1)
			reply.ModifyIndex = reply.CreateIndex
			resp = reply
		})

	// We'll reuse the fetch options and request
	opts := cache.FetchOptions{MinIndex: 0, Timeout: 10 * time.Second}
	req := &ConnectCALeafRequest{Datacenter: "dc1", Service: "web"}

	// First fetch should return immediately
	fetchCh := TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		switch v := result.(type) {
		case error:
			require.NoError(v)
		case cache.FetchResult:
			require.Equal(resp, v.Value)
			require.Equal(uint64(1), v.Index)
			// Set the LastResult for subsequent fetches
			opts.LastResult = &v
		}
	}

	// Second fetch should block with set index
	opts.MinIndex = 1
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case result := <-fetchCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}

	// Let's send in new roots, which should trigger the sign req. We need to take
	// care to set the new root as active
	caRoot2 := connect.TestCA(t, nil)
	caRoot2.Active = true
	caRoot.Active = false
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot2.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot2,
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: atomic.AddUint64(&idx, 1)},
	}
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		switch v := result.(type) {
		case error:
			require.NoError(v)
		case cache.FetchResult:
			require.Equal(resp, v.Value)
			// 3 since the second CA "update" used up 2
			require.Equal(uint64(3), v.Index)
		}
	}

	// Third fetch should block
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case result := <-fetchCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}
}

// Test that after an initial signing, an expiringLeaf will trigger a
// blocking query to resign.
func TestConnectCALeaf_expiringLeaf(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)

	caRoot := connect.TestCA(t, nil)
	caRoot.Active = true
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign to
	var resp *structs.IssuedCert
	var idx uint64
	rpc.On("RPC", "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			reply := args.Get(2).(*structs.IssuedCert)
			reply.CreateIndex = atomic.AddUint64(&idx, 1)
			reply.ModifyIndex = reply.CreateIndex

			leaf, _ := connect.TestLeaf(t, "web", caRoot)
			reply.CertPEM = leaf

			if reply.CreateIndex == 1 {
				// First call returns expired cert to prime cache with an expired one.
				reply.ValidAfter = time.Now().Add(-13 * time.Hour)
				reply.ValidBefore = time.Now().Add(-1 * time.Hour)
			} else {
				reply.ValidAfter = time.Now().Add(-1 * time.Hour)
				reply.ValidBefore = time.Now().Add(11 * time.Hour)
			}

			resp = reply
		})

	// We'll reuse the fetch options and request
	opts := cache.FetchOptions{MinIndex: 0, Timeout: 10 * time.Second}
	req := &ConnectCALeafRequest{Datacenter: "dc1", Service: "web"}

	// First fetch should return immediately
	fetchCh := TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		switch v := result.(type) {
		case error:
			require.NoError(v)
		case cache.FetchResult:
			require.Equal(resp, v.Value)
			require.Equal(uint64(1), v.Index)
			// Set the LastResult for subsequent fetches
			opts.LastResult = &v
		}
	}

	// Second fetch should return immediately despite there being
	// no updated CA roots, because we issued an expired cert.
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		switch v := result.(type) {
		case error:
			require.NoError(v)
		case cache.FetchResult:
			require.Equal(resp, v.Value)
			require.Equal(uint64(2), v.Index)
			// Set the LastResult for subsequent fetches
			opts.LastResult = &v
		}
	}

	// Third fetch should block since the cert is not expiring and
	// we also didn't update CA certs.
	opts.MinIndex = 2
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case result := <-fetchCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}
}

// testCALeafType returns a *ConnectCALeaf that is pre-configured to
// use the given RPC implementation for "ConnectCA.Sign" operations.
func testCALeafType(t *testing.T, rpc RPC) (*ConnectCALeaf, chan structs.IndexedCARoots) {
	// This creates an RPC implementation that will block until the
	// value is sent on the channel. This lets us control when the
	// next values show up.
	rootsCh := make(chan structs.IndexedCARoots, 10)
	rootsRPC := &testGatedRootsRPC{ValueCh: rootsCh}

	// Create a cache
	c := cache.TestCache(t)
	c.RegisterType(ConnectCARootName, &ConnectCARoot{RPC: rootsRPC}, &cache.RegisterOptions{
		// Disable refresh so that the gated channel controls the
		// request directly. Otherwise, we get background refreshes and
		// it screws up the ordering of the channel reads of the
		// testGatedRootsRPC implementation.
		Refresh: false,
	})

	// Create the leaf type
	return &ConnectCALeaf{
		RPC:        rpc,
		Cache:      c,
		Datacenter: "dc1",
	}, rootsCh
}

// testGatedRootsRPC will send each subsequent value on the channel as the
// RPC response, blocking if it is waiting for a value on the channel. This
// can be used to control when background fetches are returned and what they
// return.
//
// This should be used with Refresh = false for the registration options so
// automatic refreshes don't mess up the channel read ordering.
type testGatedRootsRPC struct {
	ValueCh chan structs.IndexedCARoots
}

func (r *testGatedRootsRPC) RPC(method string, args interface{}, reply interface{}) error {
	if method != "ConnectCA.Roots" {
		return fmt.Errorf("invalid RPC method: %s", method)
	}

	replyReal := reply.(*structs.IndexedCARoots)
	*replyReal = <-r.ValueCh
	return nil
}
