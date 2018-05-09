package cachetype

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test that after an initial signing, new CA roots (new ID) will
// trigger a blocking query to execute.
func TestConnectCALeaf_changingRoots(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: "1",
		TrustDomain:  "fake-trust-domain.consul",
		QueryMeta:    structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign to return signed cert
	var resp *structs.IssuedCert
	var idx uint64
	rpc.On("RPC", "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			reply := args.Get(2).(*structs.IssuedCert)
			reply.ValidBefore = time.Now().Add(12 * time.Hour)
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
		require.Equal(cache.FetchResult{
			Value: resp,
			Index: 1,
		}, result)
	}

	// Second fetch should block with set index
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case result := <-fetchCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}

	// Let's send in new roots, which should trigger the sign req
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: "2",
		TrustDomain:  "fake-trust-domain.consul",
		QueryMeta:    structs.QueryMeta{Index: 2},
	}
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		require.Equal(cache.FetchResult{
			Value: resp,
			Index: 2,
		}, result)
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
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: "1",
		TrustDomain:  "fake-trust-domain.consul",
		QueryMeta:    structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign to
	var resp *structs.IssuedCert
	var idx uint64
	rpc.On("RPC", "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			reply := args.Get(2).(*structs.IssuedCert)
			reply.CreateIndex = atomic.AddUint64(&idx, 1)
			reply.ModifyIndex = reply.CreateIndex

			// This sets the validity to 0 on the first call, and
			// 12 hours+ on subsequent calls. This means that our first
			// cert expires immediately.
			reply.ValidBefore = time.Now().Add((12 * time.Hour) *
				time.Duration(reply.CreateIndex-1))

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
		require.Equal(cache.FetchResult{
			Value: resp,
			Index: 1,
		}, result)
	}

	// Second fetch should return immediately despite there being
	// no updated CA roots, because we issued an expired cert.
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		require.Equal(cache.FetchResult{
			Value: resp,
			Index: 2,
		}, result)
	}

	// Third fetch should block since the cert is not expiring and
	// we also didn't update CA certs.
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
	return &ConnectCALeaf{RPC: rpc, Cache: c}, rootsCh
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
