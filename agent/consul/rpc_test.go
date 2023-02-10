package consul

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/rate"
	rpcRate "github.com/hashicorp/consul/agent/consul/rate"
	"github.com/hashicorp/consul/agent/consul/state"
	agent_grpc "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbsubscribe"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
)

func TestRPC_NoLeader_Fail(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.RPCHoldTimeout = 1 * time.Millisecond
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	var out struct{}

	// Make sure we eventually fail with a no leader error, which we should
	// see given the short timeout.
	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err == nil || err.Error() != structs.ErrNoLeader.Error() {
		t.Fatalf("bad: %v", err)
	}

	// Now make sure it goes through.
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
}

func TestRPC_NoLeader_Fail_on_stale_read(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.RPCHoldTimeout = 1 * time.Millisecond
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	var out struct{}

	// Make sure we eventually fail with a no leader error, which we should
	// see given the short timeout.
	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err == nil || err.Error() != structs.ErrNoLeader.Error() {
		t.Fatalf("bad: %v", err)
	}

	// Until leader has never been known, stale should fail
	getKeysReq := structs.KeyListRequest{
		Datacenter:   "dc1",
		Prefix:       "",
		Seperator:    "/",
		QueryOptions: structs.QueryOptions{AllowStale: true},
	}
	var keyList structs.IndexedKeyList
	if err := msgpackrpc.CallWithCodec(codec, "KVS.ListKeys", &getKeysReq, &keyList); err.Error() != structs.ErrNoLeader.Error() {
		t.Fatalf("expected %v but got err: %v", structs.ErrNoLeader, err)
	}

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	if err := msgpackrpc.CallWithCodec(codec, "KVS.ListKeys", &getKeysReq, &keyList); err != nil {
		t.Fatalf("Did not expect any error but got err: %v", err)
	}
}

func TestRPC_NoLeader_Retry(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.RPCHoldTimeout = 10 * time.Second
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	var out struct{}

	// This isn't sure-fire but tries to check that we don't have a
	// leader going into the RPC, so we exercise the retry logic.
	if ok, _, _ := s1.getLeader(); ok {
		t.Fatalf("should not have a leader yet")
	}

	// The timeout is long enough to ride out any reasonable leader
	// election.
	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
}

func TestRPC_getLeader_ErrLeaderNotTracked(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	cluster := newTestCluster(t, &testClusterConfig{
		Datacenter: "dc1",
		Servers:    3,
		ServerWait: func(t *testing.T, srv *Server) {
			// The test cluster waits for a leader to be established
			// but not for all the RPC tracking of all servers to be updated
			// so we also want to wait for that here
			retry.Run(t, func(r *retry.R) {
				if !srv.IsLeader() {
					_, _, err := srv.getLeader()
					require.NoError(r, err)
				}
			})

		},
	})

	// At this point we know we have a cluster with a leader and all followers are tracking that
	// leader in the serverLookup struct. We need to find a follower to hack its server lookup
	// to force the error we desire

	var follower *Server
	for _, srv := range cluster.Servers {
		if !srv.IsLeader() {
			follower = srv
			break
		}
	}

	_, leaderMeta, err := follower.getLeader()
	require.NoError(t, err)

	// now do some behind the scenes trickery on the followers server lookup
	// to remove the leader from it so that we can force a ErrLeaderNotTracked error
	follower.serverLookup.RemoveServer(leaderMeta)

	isLeader, meta, err := follower.getLeader()
	require.Error(t, err)
	require.True(t, errors.Is(err, structs.ErrLeaderNotTracked))
	require.Nil(t, meta)
	require.False(t, isLeader)
}

type MockSink struct {
	*bytes.Buffer
	cancel bool
}

func (m *MockSink) ID() string {
	return "Mock"
}

func (m *MockSink) Cancel() error {
	m.cancel = true
	return nil
}

func (m *MockSink) Close() error {
	return nil
}

func TestServer_blockingQuery(t *testing.T) {
	t.Parallel()
	_, s := testServerWithConfig(t)

	// Perform a non-blocking query. Note that it's significant that the meta has
	// a zero index in response - the implied opts.MinQueryIndex is also zero but
	// this should not block still.
	t.Run("non-blocking query", func(t *testing.T) {
		var opts structs.QueryOptions
		var meta structs.QueryMeta
		var calls int
		fn := func(_ memdb.WatchSet, _ *state.Store) error {
			calls++
			return nil
		}
		err := s.blockingQuery(&opts, &meta, fn)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})

	// Perform a blocking query that gets woken up and loops around once.
	t.Run("blocking query - single loop", func(t *testing.T) {
		opts := structs.QueryOptions{
			MinQueryIndex: 3,
		}
		var meta structs.QueryMeta
		var calls int
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			if calls == 0 {
				meta.Index = 3

				fakeCh := make(chan struct{})
				close(fakeCh)
				ws.Add(fakeCh)
			} else {
				meta.Index = 4
			}
			calls++
			return nil
		}
		err := s.blockingQuery(&opts, &meta, fn)
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	// Perform a blocking query that returns a zero index from blocking func (e.g.
	// no state yet). This should still return an empty response immediately, but
	// with index of 1 and then block on the next attempt. In one sense zero index
	// is not really a valid response from a state method that is not an error but
	// in practice a lot of state store operations do return it unless they
	// explicitly special checks to turn 0 into 1. Often this is not caught or
	// covered by tests but eventually when hit in the wild causes blocking
	// clients to busy loop and burn CPU. This test ensure that blockingQuery
	// systematically does the right thing to prevent future bugs like that.
	t.Run("blocking query with 0 modifyIndex from state func", func(t *testing.T) {
		opts := structs.QueryOptions{
			MinQueryIndex: 0,
		}
		var meta structs.QueryMeta
		var calls int
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			if opts.MinQueryIndex > 0 {
				// If client requested blocking, block forever. This is simulating
				// waiting for the watched resource to be initialized/written to giving
				// it a non-zero index. Note the timeout on the query options is relied
				// on to stop the test taking forever.
				fakeCh := make(chan struct{})
				ws.Add(fakeCh)
			}
			meta.Index = 0
			calls++
			return nil
		}
		require.NoError(t, s.blockingQuery(&opts, &meta, fn))
		assert.Equal(t, 1, calls)
		assert.Equal(t, uint64(1), meta.Index,
			"expect fake index of 1 to force client to block on next update")

		// Simulate client making next request
		opts.MinQueryIndex = 1
		opts.MaxQueryTime = 20 * time.Millisecond // Don't wait too long

		// This time we should block even though the func returns index 0 still
		t0 := time.Now()
		require.NoError(t, s.blockingQuery(&opts, &meta, fn))
		t1 := time.Now()
		assert.Equal(t, 2, calls)
		assert.Equal(t, uint64(1), meta.Index,
			"expect fake index of 1 to force client to block on next update")
		assert.True(t, t1.Sub(t0) > 20*time.Millisecond,
			"should have actually blocked waiting for timeout")

	})

	// Perform a query that blocks and gets interrupted when the state store
	// is abandoned.
	t.Run("blocking query interrupted by abandonCh", func(t *testing.T) {
		opts := structs.QueryOptions{
			MinQueryIndex: 3,
		}
		var meta structs.QueryMeta
		var calls int
		fn := func(_ memdb.WatchSet, _ *state.Store) error {
			if calls == 0 {
				meta.Index = 3

				snap, err := s.fsm.Snapshot()
				if err != nil {
					t.Fatalf("err: %v", err)
				}
				defer snap.Release()

				buf := bytes.NewBuffer(nil)
				sink := &MockSink{buf, false}
				if err := snap.Persist(sink); err != nil {
					t.Fatalf("err: %v", err)
				}

				if err := s.fsm.Restore(sink); err != nil {
					t.Fatalf("err: %v", err)
				}
			}
			calls++
			return nil
		}
		err := s.blockingQuery(&opts, &meta, fn)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})

	t.Run("ResultsFilteredByACLs is reset for unauthenticated calls", func(t *testing.T) {
		opts := structs.QueryOptions{
			Token: "",
		}
		var meta structs.QueryMeta
		fn := func(_ memdb.WatchSet, _ *state.Store) error {
			meta.ResultsFilteredByACLs = true
			return nil
		}

		err := s.blockingQuery(&opts, &meta, fn)
		require.NoError(t, err)
		require.False(t, meta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be reset for unauthenticated calls")
	})

	t.Run("ResultsFilteredByACLs is honored for authenticated calls", func(t *testing.T) {
		token, err := lib.GenerateUUID(nil)
		require.NoError(t, err)

		opts := structs.QueryOptions{
			Token: token,
		}
		var meta structs.QueryMeta
		fn := func(_ memdb.WatchSet, _ *state.Store) error {
			meta.ResultsFilteredByACLs = true
			return nil
		}

		err = s.blockingQuery(&opts, &meta, fn)
		require.NoError(t, err)
		require.True(t, meta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be honored for authenticated calls")
	})

	t.Run("non-blocking query for item that does not exist", func(t *testing.T) {
		opts := structs.QueryOptions{}
		meta := structs.QueryMeta{}
		calls := 0
		fn := func(_ memdb.WatchSet, _ *state.Store) error {
			calls++
			return errNotFound
		}

		err := s.blockingQuery(&opts, &meta, fn)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})

	t.Run("blocking query for item that does not exist", func(t *testing.T) {
		opts := structs.QueryOptions{MinQueryIndex: 3, MaxQueryTime: 100 * time.Millisecond}
		meta := structs.QueryMeta{}
		calls := 0
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			calls++
			if calls == 1 {
				meta.Index = 3

				ch := make(chan struct{})
				close(ch)
				ws.Add(ch)
				return errNotFound
			}
			meta.Index = 5
			return errNotFound
		}

		err := s.blockingQuery(&opts, &meta, fn)
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("blocking query for item that existed and is removed", func(t *testing.T) {
		opts := structs.QueryOptions{MinQueryIndex: 3, MaxQueryTime: 100 * time.Millisecond}
		meta := structs.QueryMeta{}
		calls := 0
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			calls++
			if calls == 1 {
				meta.Index = 3

				ch := make(chan struct{})
				close(ch)
				ws.Add(ch)
				return nil
			}
			meta.Index = 5
			return errNotFound
		}

		start := time.Now()
		err := s.blockingQuery(&opts, &meta, fn)
		require.True(t, time.Since(start) < opts.MaxQueryTime, "query timed out")
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("blocking query for non-existent item that is created", func(t *testing.T) {
		opts := structs.QueryOptions{MinQueryIndex: 3, MaxQueryTime: 100 * time.Millisecond}
		meta := structs.QueryMeta{}
		calls := 0
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			calls++
			if calls == 1 {
				meta.Index = 3

				ch := make(chan struct{})
				close(ch)
				ws.Add(ch)
				return errNotFound
			}
			meta.Index = 5
			return nil
		}

		start := time.Now()
		err := s.blockingQuery(&opts, &meta, fn)
		require.True(t, time.Since(start) < opts.MaxQueryTime, "query timed out")
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})
}

func TestRPC_ReadyForConsistentReads(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir, s := testServerWithConfig(t, func(c *Config) {
		c.RPCHoldTimeout = 2 * time.Millisecond
	})
	defer os.RemoveAll(dir)
	defer s.Shutdown()

	testrpc.WaitForLeader(t, s.RPC, "dc1")

	if !s.isReadyForConsistentReads() {
		t.Fatal("Server should be ready for consistent reads")
	}

	s.resetConsistentReadReady()
	err := s.consistentRead()
	if err.Error() != "Not ready to serve consistent reads" {
		t.Fatal("Server should NOT be ready for consistent reads")
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		s.setConsistentReadReady()
	}()

	retry.Run(t, func(r *retry.R) {
		if err := s.consistentRead(); err != nil {
			r.Fatalf("Expected server to be ready for consistent reads, got error %v", err)
		}
	})
}

func TestRPC_MagicByteTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.RPCHandshakeTimeout = 10 * time.Millisecond
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	// Connect to the server with bare TCP to simulate a malicious client trying
	// to hold open resources.
	addr := s1.config.RPCAdvertise
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	require.NoError(t, err)
	defer conn.Close()

	// Wait for more than the timeout. This is timing dependent so could fail if
	// the CPU is super overloaded so the handler goroutine so I'm using a retry
	// loop below to be sure but this feels like a pretty generous margin for
	// error (10x the timeout and 100ms of scheduling time).
	time.Sleep(100 * time.Millisecond)

	// Set a read deadline on the Conn in case the timeout is not working we don't
	// want the read below to block forever. Needs to be much longer than what we
	// expect and the error should be different too.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	retry.Run(t, func(r *retry.R) {
		// Sanity check the conn was closed by attempting to read from it (a write
		// might not detect the close).
		buf := make([]byte, 10)
		_, err = conn.Read(buf)
		require.Error(r, err)
		require.Contains(r, err.Error(), "EOF")
	})
}

func TestRPC_TLSHandshakeTimeout(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.RPCHandshakeTimeout = 10 * time.Millisecond
		c.TLSConfig.InternalRPC.CAFile = "../../test/hostname/CertAuth.crt"
		c.TLSConfig.InternalRPC.CertFile = "../../test/hostname/Alice.crt"
		c.TLSConfig.InternalRPC.KeyFile = "../../test/hostname/Alice.key"
		c.TLSConfig.InternalRPC.VerifyServerHostname = true
		c.TLSConfig.InternalRPC.VerifyOutgoing = true
		c.TLSConfig.InternalRPC.VerifyIncoming = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	// Connect to the server with TLS magic byte delivered on time
	addr := s1.config.RPCAdvertise
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	require.NoError(t, err)
	defer conn.Close()

	// Write TLS byte to avoid being closed by either the (outer) first byte
	// timeout or the fact that server requires TLS
	_, err = conn.Write([]byte{byte(pool.RPCTLS)})
	require.NoError(t, err)

	// Wait for more than the timeout before we start a TLS handshake. This is
	// timing dependent so could fail if the CPU is super overloaded so the
	// handler goroutine so I'm using a retry loop below to be sure but this feels
	// like a pretty generous margin for error (10x the timeout and 100ms of
	// scheduling time).
	time.Sleep(100 * time.Millisecond)

	// Set a read deadline on the Conn in case the timeout is not working we don't
	// want the read below to block forever. Needs to be much longer than what we
	// expect and the error should be different too.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	retry.Run(t, func(r *retry.R) {
		// Sanity check the conn was closed by attempting to read from it (a write
		// might not detect the close).
		buf := make([]byte, 10)
		_, err = conn.Read(buf)
		require.Error(r, err)
		require.Contains(r, err.Error(), "EOF")
	})
}

func TestRPC_PreventsTLSNesting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	cases := []struct {
		name      string
		outerByte pool.RPCType
		innerByte pool.RPCType
		wantClose bool
	}{
		{
			// Base case, sanity check normal RPC in TLS works
			name:      "RPC in TLS",
			outerByte: pool.RPCTLS,
			innerByte: pool.RPCConsul,
			wantClose: false,
		},
		{
			// Nested TLS-in-TLS
			name:      "TLS in TLS",
			outerByte: pool.RPCTLS,
			innerByte: pool.RPCTLS,
			wantClose: true,
		},
		{
			// Nested TLS-in-TLS
			name:      "TLS in Insecure TLS",
			outerByte: pool.RPCTLSInsecure,
			innerByte: pool.RPCTLS,
			wantClose: true,
		},
		{
			// Nested TLS-in-TLS
			name:      "Insecure TLS in TLS",
			outerByte: pool.RPCTLS,
			innerByte: pool.RPCTLSInsecure,
			wantClose: true,
		},
		{
			// Nested TLS-in-TLS
			name:      "Insecure TLS in Insecure TLS",
			outerByte: pool.RPCTLSInsecure,
			innerByte: pool.RPCTLSInsecure,
			wantClose: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir1, s1 := testServerWithConfig(t, func(c *Config) {
				c.TLSConfig.InternalRPC.CAFile = "../../test/hostname/CertAuth.crt"
				c.TLSConfig.InternalRPC.CertFile = "../../test/hostname/Alice.crt"
				c.TLSConfig.InternalRPC.KeyFile = "../../test/hostname/Alice.key"
				c.TLSConfig.InternalRPC.VerifyServerHostname = true
				c.TLSConfig.InternalRPC.VerifyOutgoing = true
				c.TLSConfig.InternalRPC.VerifyIncoming = false // saves us getting client cert setup
				c.TLSConfig.Domain = "consul"
			})
			defer os.RemoveAll(dir1)
			defer s1.Shutdown()

			// Connect to the server with TLS magic byte delivered on time
			addr := s1.config.RPCAdvertise
			conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
			require.NoError(t, err)
			defer conn.Close()

			// Write Outer magic byte
			_, err = conn.Write([]byte{byte(tc.outerByte)})
			require.NoError(t, err)

			// Start tls client
			tlsWrap := s1.tlsConfigurator.OutgoingRPCWrapper()
			tlsConn, err := tlsWrap("dc1", conn)
			require.NoError(t, err)

			// Write Inner magic byte
			_, err = tlsConn.Write([]byte{byte(tc.innerByte)})
			require.NoError(t, err)

			if tc.wantClose {
				// Allow up to a second for a read failure to indicate conn was closed by
				// server.
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))

				retry.Run(t, func(r *retry.R) {
					// Sanity check the conn was closed by attempting to read from it (a
					// write might not detect the close).
					buf := make([]byte, 10)
					_, err = tlsConn.Read(buf)
					require.Error(r, err)
					require.Contains(r, err.Error(), "EOF")
				})
			} else {
				// Set a shorter read deadline that should typically be enough to detect
				// immediate close but will also not make test hang forever. This
				// positive case is mostly just a sanity check that the test code here
				// is actually not failing just due to some other error in the way we
				// setup TLS. It also sanity checks that we still allow valid TLS conns
				// but if it produces possible false-positives in CI sometimes that's
				// not such a huge deal - CI won't be brittle and it will have done it's
				// job as a sanity check most of the time.
				conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
				buf := make([]byte, 10)
				_, err = tlsConn.Read(buf)
				require.Error(t, err)
				require.Contains(t, err.Error(), "i/o timeout")
			}
		})
	}
}

func connectClient(t *testing.T, s1 *Server, mb pool.RPCType, useTLS, wantOpen bool, message string) net.Conn {
	t.Helper()

	addr := s1.config.RPCAdvertise
	tlsWrap := s1.tlsConfigurator.OutgoingRPCWrapper()

	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	require.NoError(t, err)

	// Write magic byte so we aren't timed out
	outerByte := mb
	if useTLS {
		outerByte = pool.RPCTLS
	}
	_, err = conn.Write([]byte{byte(outerByte)})
	require.NoError(t, err)

	if useTLS {
		tlsConn, err := tlsWrap(s1.config.Datacenter, conn)
		// Subtly, tlsWrap will NOT actually do a handshake in this case - it only
		// does so for some configs, so even if the server closed the conn before
		// handshake this won't fail and it's only when we attempt to read or write
		// that we'll see the broken pipe.
		require.NoError(t, err, "%s: wanted open conn, failed TLS handshake: %s",
			message, err)
		conn = tlsConn

		// Write Inner magic byte
		_, err = conn.Write([]byte{byte(mb)})
		if !wantOpen {
			// TLS Handshake will be done on this attempt to write and should fail
			require.Error(t, err, "%s: wanted closed conn, TLS Handshake succeeded", message)
		} else {
			require.NoError(t, err, "%s: wanted open conn, failed writing inner magic byte: %s",
				message, err)
		}
	}

	// Check if the conn is in the state we want.
	retry.Run(t, func(r *retry.R) {
		// Don't wait around as server won't be sending data but the read will fail
		// immediately if the conn is closed.
		conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
		buf := make([]byte, 10)
		_, err := conn.Read(buf)
		require.Error(r, err)
		if wantOpen {
			require.Contains(r, err.Error(), "i/o timeout",
				"%s: wanted an open conn (read timeout)", message)
		} else {
			if useTLS {
				require.Error(r, err)
				// TLS may fail during either read or write of the handshake so there
				// are a few different errors that come up.
				if !strings.Contains(err.Error(), "read: connection reset by peer") &&
					!strings.Contains(err.Error(), "write: connection reset by peer") &&
					!strings.Contains(err.Error(), "write: broken pipe") {
					r.Fatalf("%s: wanted closed conn got err: %s", message, err)
				}
			} else {
				require.Contains(r, err.Error(), "EOF", "%s: wanted a closed conn",
					message)
			}
		}
	})

	return conn
}

func TestRPC_RPCMaxConnsPerClient(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	cases := []struct {
		name       string
		magicByte  pool.RPCType
		tlsEnabled bool
	}{
		{"RPC v2", pool.RPCMultiplexV2, false},
		{"RPC v2 TLS", pool.RPCMultiplexV2, true},
		{"RPC", pool.RPCConsul, false},
		{"RPC TLS", pool.RPCConsul, true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir1, s1 := testServerWithConfig(t, func(c *Config) {
				// we have to set this to 3 because autopilot is going to keep a connection open
				c.RPCMaxConnsPerClient = 3
				if tc.tlsEnabled {
					c.TLSConfig.InternalRPC.CAFile = "../../test/hostname/CertAuth.crt"
					c.TLSConfig.InternalRPC.CertFile = "../../test/hostname/Alice.crt"
					c.TLSConfig.InternalRPC.KeyFile = "../../test/hostname/Alice.key"
					c.TLSConfig.InternalRPC.VerifyServerHostname = true
					c.TLSConfig.InternalRPC.VerifyOutgoing = true
					c.TLSConfig.InternalRPC.VerifyIncoming = false // saves us getting client cert setup
					c.TLSConfig.Domain = "consul"
				}
			})
			defer os.RemoveAll(dir1)
			defer s1.Shutdown()

			waitForLeaderEstablishment(t, s1)

			// Connect to the server with bare TCP
			conn1 := connectClient(t, s1, tc.magicByte, tc.tlsEnabled, true, "conn1")
			defer conn1.Close()

			// Two conns should succeed
			conn2 := connectClient(t, s1, tc.magicByte, tc.tlsEnabled, true, "conn2")
			defer conn2.Close()

			// Third should be closed byt the limiter
			conn3 := connectClient(t, s1, tc.magicByte, tc.tlsEnabled, false, "conn3")
			defer conn3.Close()

			// If we close one of the earlier ones, we should be able to open another
			addr := conn1.RemoteAddr()
			conn1.Close()
			retry.Run(t, func(r *retry.R) {
				if n := s1.rpcConnLimiter.NumOpen(addr); n >= 3 {
					r.Fatal("waiting for open conns to drop")
				}
			})
			conn4 := connectClient(t, s1, tc.magicByte, tc.tlsEnabled, true, "conn4")
			defer conn4.Close()

			// Reload config with higher limit
			rc := ReloadableConfig{
				RPCRateLimit:         s1.config.RPCRateLimit,
				RPCMaxBurst:          s1.config.RPCMaxBurst,
				RPCMaxConnsPerClient: 10,
			}
			require.NoError(t, s1.ReloadConfig(rc))

			// Now another conn should be allowed
			conn5 := connectClient(t, s1, tc.magicByte, tc.tlsEnabled, true, "conn5")
			defer conn5.Close()
		})
	}
}

func TestRPC_readUint32(t *testing.T) {
	cases := []struct {
		name    string
		writeFn func(net.Conn)
		readFn  func(*testing.T, net.Conn)
	}{
		{
			name: "timeouts irrelevant",
			writeFn: func(conn net.Conn) {
				_ = binary.Write(conn, binary.BigEndian, uint32(42))
				_ = binary.Write(conn, binary.BigEndian, uint32(math.MaxUint32))
				_ = binary.Write(conn, binary.BigEndian, uint32(1))
			},
			readFn: func(t *testing.T, conn net.Conn) {
				t.Helper()
				v, err := readUint32(conn, 5*time.Second)
				require.NoError(t, err)
				require.Equal(t, uint32(42), v)

				v, err = readUint32(conn, 5*time.Second)
				require.NoError(t, err)
				require.Equal(t, uint32(math.MaxUint32), v)

				v, err = readUint32(conn, 5*time.Second)
				require.NoError(t, err)
				require.Equal(t, uint32(1), v)
			},
		},
		{
			name: "triggers timeout on last read",
			writeFn: func(conn net.Conn) {
				_ = binary.Write(conn, binary.BigEndian, uint32(42))
				_ = binary.Write(conn, binary.BigEndian, uint32(math.MaxUint32))
				_ = binary.Write(conn, binary.BigEndian, uint16(1)) // half as many bytes as expected
			},
			readFn: func(t *testing.T, conn net.Conn) {
				t.Helper()
				v, err := readUint32(conn, 5*time.Second)
				require.NoError(t, err)
				require.Equal(t, uint32(42), v)

				v, err = readUint32(conn, 5*time.Second)
				require.NoError(t, err)
				require.Equal(t, uint32(math.MaxUint32), v)

				_, err = readUint32(conn, 50*time.Millisecond)
				require.Error(t, err)
				nerr, ok := err.(net.Error)
				require.True(t, ok)
				require.True(t, nerr.Timeout())
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var doneWg sync.WaitGroup
			defer doneWg.Wait()

			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			// Client pushes some data.
			doneWg.Add(1)
			go func() {
				doneWg.Done()
				tc.writeFn(client)
			}()

			// The server tests the function for us.
			tc.readFn(t, server)
		})
	}
}

func TestRPC_LocalTokenStrippedOnForward(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.ACLInitialManagementToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 100
		c.ACLReplicationApplyLimit = 1000000
	})
	s2.tokens.UpdateReplicationToken("root", tokenStore.TokenSourceConfig)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")
	waitForNewACLReplication(t, s2, structs.ACLReplicateTokens, 1, 1, 0)

	// create simple kv policy
	kvPolicy, err := upsertTestPolicyWithRules(codec, "root", "dc1", `
	key_prefix "" { policy = "write" }
	`)
	require.NoError(t, err)

	// Wait for it to replicate
	retry.Run(t, func(r *retry.R) {
		_, p, err := s2.fsm.State().ACLPolicyGetByID(nil, kvPolicy.ID, &acl.EnterpriseMeta{})
		require.Nil(r, err)
		require.NotNil(r, p)
	})

	// create local token that only works in DC2
	localToken2, err := upsertTestToken(codec, "root", "dc2", func(token *structs.ACLToken) {
		token.Local = true
		token.Policies = []structs.ACLTokenPolicyLink{
			{ID: kvPolicy.ID},
		}
	})
	require.NoError(t, err)

	// Try to use it locally (it should work)
	arg := structs.KVSRequest{
		Datacenter: "dc2",
		Op:         api.KVSet,
		DirEnt: structs.DirEntry{
			Key:   "foo",
			Value: []byte("bar"),
		},
		WriteRequest: structs.WriteRequest{Token: localToken2.SecretID},
	}
	var out bool
	err = msgpackrpc.CallWithCodec(codec2, "KVS.Apply", &arg, &out)
	require.NoError(t, err)
	require.Equal(t, localToken2.SecretID, arg.WriteRequest.Token, "token should not be stripped")

	// Try to use it remotely
	arg = structs.KVSRequest{
		Datacenter: "dc1",
		Op:         api.KVSet,
		DirEnt: structs.DirEntry{
			Key:   "foo",
			Value: []byte("bar"),
		},
		WriteRequest: structs.WriteRequest{Token: localToken2.SecretID},
	}
	err = msgpackrpc.CallWithCodec(codec2, "KVS.Apply", &arg, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Update the anon token to also be able to write to kv
	{
		tokenUpsertReq := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID: acl.AnonymousTokenID,
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: kvPolicy.ID,
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		token := structs.ACLToken{}
		err = msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &tokenUpsertReq, &token)
		require.NoError(t, err)
		require.NotEmpty(t, token.SecretID)
	}

	// Try to use it remotely again, but this time it should fallback to anon
	arg = structs.KVSRequest{
		Datacenter: "dc1",
		Op:         api.KVSet,
		DirEnt: structs.DirEntry{
			Key:   "foo",
			Value: []byte("bar"),
		},
		WriteRequest: structs.WriteRequest{Token: localToken2.SecretID},
	}
	err = msgpackrpc.CallWithCodec(codec2, "KVS.Apply", &arg, &out)
	require.NoError(t, err)
	require.Equal(t, localToken2.SecretID, arg.WriteRequest.Token, "token should not be stripped")
}

func TestRPC_LocalTokenStrippedOnForward_GRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.ACLInitialManagementToken = "root"
		c.RPCConfig.EnableStreaming = true
	})
	s1.tokens.UpdateAgentToken("root", tokenStore.TokenSourceConfig)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 100
		c.ACLReplicationApplyLimit = 1000000
		c.RPCConfig.EnableStreaming = true
	})
	s2.tokens.UpdateReplicationToken("root", tokenStore.TokenSourceConfig)
	s2.tokens.UpdateAgentToken("root", tokenStore.TokenSourceConfig)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")
	waitForNewACLReplication(t, s2, structs.ACLReplicateTokens, 1, 1, 0)

	// create simple service policy
	policy, err := upsertTestPolicyWithRules(codec, "root", "dc1", `
	node_prefix "" { policy = "read" }
	service_prefix "" { policy = "read" }
	`)
	require.NoError(t, err)

	// Wait for it to replicate
	retry.Run(t, func(r *retry.R) {
		_, p, err := s2.fsm.State().ACLPolicyGetByID(nil, policy.ID, &acl.EnterpriseMeta{})
		require.Nil(r, err)
		require.NotNil(r, p)
	})

	// create local token that only works in DC2
	localToken2, err := upsertTestToken(codec, "root", "dc2", func(token *structs.ACLToken) {
		token.Local = true
		token.Policies = []structs.ACLTokenPolicyLink{
			{ID: policy.ID},
		}
	})
	require.NoError(t, err)

	testutil.RunStep(t, "Register a dummy node with a service", func(t *testing.T) {
		req := &structs.RegisterRequest{
			Node:       "node1",
			Address:    "3.4.5.6",
			Datacenter: "dc1",
			Service: &structs.NodeService{
				ID:      "redis1",
				Service: "redis",
				Address: "3.4.5.6",
				Port:    8080,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out struct{}
		require.NoError(t, s1.RPC(context.Background(), "Catalog.Register", &req, &out))
	})

	var conn *grpc.ClientConn
	{
		client, resolverBuilder, balancerBuilder := newClientWithGRPCPlumbing(t, func(c *Config) {
			c.Datacenter = "dc2"
			c.PrimaryDatacenter = "dc1"
			c.RPCConfig.EnableStreaming = true
		})
		joinLAN(t, client, s2)
		testrpc.WaitForTestAgent(t, client.RPC, "dc2", testrpc.WithToken("root"))

		pool := agent_grpc.NewClientConnPool(agent_grpc.ClientConnPoolConfig{
			Servers:               resolverBuilder,
			DialingFromServer:     false,
			DialingFromDatacenter: "dc2",
		})

		conn, err = pool.ClientConn("dc2")
		require.NoError(t, err)
	}

	// Try to use it locally (it should work)
	testutil.RunStep(t, "token used locally should work", func(t *testing.T) {
		arg := &pbsubscribe.SubscribeRequest{
			Topic:      pbsubscribe.Topic_ServiceHealth,
			Key:        "redis",
			Token:      localToken2.SecretID,
			Datacenter: "dc2",
		}
		event, err := getFirstSubscribeEventOrError(conn, arg)
		require.NoError(t, err)
		require.NotNil(t, event)

		// make sure that token restore defer works
		require.Equal(t, localToken2.SecretID, arg.Token, "token should not be stripped")
	})

	testutil.RunStep(t, "token used remotely should not work", func(t *testing.T) {
		arg := &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key: "redis",
				},
			},
			Token:      localToken2.SecretID,
			Datacenter: "dc1",
		}

		event, err := getFirstSubscribeEventOrError(conn, arg)

		// NOTE: the subscription endpoint is a filtering style instead of a
		// hard-fail style so when the token isn't present 100% of the data is
		// filtered out leading to a stream with an empty snapshot.
		require.NoError(t, err)
		require.IsType(t, &pbsubscribe.Event_EndOfSnapshot{}, event.Payload)
		require.True(t, event.Payload.(*pbsubscribe.Event_EndOfSnapshot).EndOfSnapshot)
	})

	testutil.RunStep(t, "update anonymous token to read services", func(t *testing.T) {
		tokenUpsertReq := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID: acl.AnonymousTokenID,
				Policies: []structs.ACLTokenPolicyLink{
					{ID: policy.ID},
				},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		token := structs.ACLToken{}
		err = msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &tokenUpsertReq, &token)
		require.NoError(t, err)
		require.NotEmpty(t, token.SecretID)
	})

	testutil.RunStep(t, "token used remotely should fallback on anonymous token now", func(t *testing.T) {
		arg := &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key: "redis",
				},
			},
			Token:      localToken2.SecretID,
			Datacenter: "dc1",
		}

		event, err := getFirstSubscribeEventOrError(conn, arg)
		require.NoError(t, err)
		require.NotNil(t, event)

		// So now that we can read data, we should get a snapshot with just instances of the "consul" service.
		require.NoError(t, err)

		require.IsType(t, &pbsubscribe.Event_ServiceHealth{}, event.Payload)
		esh := event.Payload.(*pbsubscribe.Event_ServiceHealth)

		require.Equal(t, pbsubscribe.CatalogOp_Register, esh.ServiceHealth.Op)
		csn := esh.ServiceHealth.CheckServiceNode

		require.NotNil(t, csn)
		require.NotNil(t, csn.Node)
		require.Equal(t, "node1", csn.Node.Node)
		require.Equal(t, "3.4.5.6", csn.Node.Address)
		require.NotNil(t, csn.Service)
		require.Equal(t, "redis1", csn.Service.ID)
		require.Equal(t, "redis", csn.Service.Service)

		// make sure that token restore defer works
		require.Equal(t, localToken2.SecretID, arg.Token, "token should not be stripped")
	})
}

func TestCanRetry(t *testing.T) {
	type testCase struct {
		name     string
		req      structs.RPCInfo
		err      error
		expected bool
		timeout  time.Time
	}
	config := DefaultConfig()
	now := time.Now()
	config.RPCHoldTimeout = 7 * time.Second
	retryableMessages := []error{
		ErrChunkingResubmit,
		rpcRate.ErrRetryElsewhere,
	}
	run := func(t *testing.T, tc testCase) {
		timeOutValue := tc.timeout
		if timeOutValue.IsZero() {
			timeOutValue = now
		}
		require.Equal(t, tc.expected, canRetry(tc.req, tc.err, timeOutValue, config, retryableMessages))
	}

	var testCases = []testCase{
		{
			name:     "unexpected error",
			err:      fmt.Errorf("some arbitrary error"),
			expected: false,
		},
		{
			name:     "checking error",
			err:      fmt.Errorf("some wrapping :%w", ErrChunkingResubmit),
			expected: true,
		},
		{
			name:     "no leader error",
			err:      fmt.Errorf("some wrapping: %w", structs.ErrNoLeader),
			expected: true,
		},
		{
			name:     "ErrRetryElsewhere",
			err:      fmt.Errorf("some wrapping: %w", rate.ErrRetryElsewhere),
			expected: true,
		},
		{
			name:     "ErrRetryLater",
			err:      fmt.Errorf("some wrapping: %w", rate.ErrRetryLater),
			expected: false,
		},
		{
			name:     "EOF on read request",
			req:      isReadRequest{},
			err:      io.EOF,
			expected: true,
		},
		{
			name:     "EOF error",
			req:      &structs.DCSpecificRequest{},
			err:      io.EOF,
			expected: true,
		},
		{
			name:     "HasTimedOut implementation with no error",
			req:      &structs.DCSpecificRequest{},
			err:      nil,
			expected: false,
		},
		{
			name:     "HasTimedOut implementation timedOut with no error",
			req:      &structs.DCSpecificRequest{},
			err:      nil,
			expected: false,
			timeout:  now.Add(-(config.RPCHoldTimeout + time.Second)),
		},
		{
			name:     "HasTimedOut implementation timedOut (with EOF error)",
			req:      &structs.DCSpecificRequest{},
			err:      io.EOF,
			expected: false,
			timeout:  now.Add(-(config.RPCHoldTimeout + time.Second)),
		},
		{
			name:     "HasTimedOut implementation timedOut blocking call",
			req:      &structs.DCSpecificRequest{QueryOptions: structs.QueryOptions{MaxQueryTime: 300, MinQueryIndex: 1}},
			err:      nil,
			expected: false,
			timeout:  now.Add(-(config.RPCHoldTimeout + config.MaxQueryTime + time.Second)),
		},
		{
			name:     "HasTimedOut implementation timedOut blocking call (MaxQueryTime not set)",
			req:      &structs.DCSpecificRequest{QueryOptions: structs.QueryOptions{MinQueryIndex: 1}},
			err:      nil,
			expected: false,
			timeout:  now.Add(-(config.RPCHoldTimeout + config.MaxQueryTime + time.Second)),
		},
		{
			name:     "EOF on write request",
			err:      io.EOF,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

type isReadRequest struct {
	structs.RPCInfo
}

func (r isReadRequest) IsRead() bool {
	return true
}

func (r isReadRequest) HasTimedOut(_ time.Time, _, _, _ time.Duration) (bool, error) {
	return false, nil
}

func TestRPC_AuthorizeRaftRPC(t *testing.T) {
	caPEM, caPK, err := tlsutil.GenerateCA(tlsutil.CAOpts{Days: 5, Domain: "consul"})
	require.NoError(t, err)

	caSigner, err := tlsutil.ParseSigner(caPK)
	require.NoError(t, err)

	dir := testutil.TempDir(t, "certs")
	err = os.WriteFile(filepath.Join(dir, "ca.pem"), []byte(caPEM), 0600)
	require.NoError(t, err)

	intermediatePEM, intermediatePK, err := tlsutil.GenerateCert(tlsutil.CertOpts{IsCA: true, CA: caPEM, Signer: caSigner, Days: 5})
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "intermediate.pem"), []byte(intermediatePEM), 0600)
	require.NoError(t, err)

	newCert := func(t *testing.T, caPEM, pk, node, name string) {
		t.Helper()

		signer, err := tlsutil.ParseSigner(pk)
		require.NoError(t, err)

		pem, key, err := tlsutil.GenerateCert(tlsutil.CertOpts{
			Signer:      signer,
			CA:          caPEM,
			Name:        name,
			Days:        5,
			DNSNames:    []string{node + "." + name, name, "localhost"},
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		})
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(dir, node+"-"+name+".pem"), []byte(pem), 0600)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(dir, node+"-"+name+".key"), []byte(key), 0600)
		require.NoError(t, err)
	}

	newCert(t, caPEM, caPK, "srv1", "server.dc1.consul")

	_, connectCApk, err := connect.GeneratePrivateKey()
	require.NoError(t, err)

	_, srv := testServerWithConfig(t, func(c *Config) {
		c.TLSConfig.Domain = "consul." // consul. is the default value in agent/config
		c.TLSConfig.InternalRPC.CAFile = filepath.Join(dir, "ca.pem")
		c.TLSConfig.InternalRPC.CertFile = filepath.Join(dir, "srv1-server.dc1.consul.pem")
		c.TLSConfig.InternalRPC.KeyFile = filepath.Join(dir, "srv1-server.dc1.consul.key")
		c.TLSConfig.InternalRPC.VerifyIncoming = true
		c.TLSConfig.InternalRPC.VerifyServerHostname = true
		// Enable Auto-Encrypt so that Connect CA roots are added to the
		// tlsutil.Configurator.
		c.AutoEncryptAllowTLS = true
		c.CAConfig = &structs.CAConfiguration{
			ClusterID: connect.TestClusterID,
			Provider:  structs.ConsulCAProvider,
			Config:    map[string]interface{}{"PrivateKey": connectCApk},
		}

	})
	defer srv.Shutdown()

	// Wait for ConnectCA initiation to complete.
	retry.Run(t, func(r *retry.R) {
		_, root := srv.caManager.getCAProvider()
		if root == nil {
			r.Fatal("ConnectCA root is still nil")
		}
	})

	useTLSByte := func(t *testing.T, c *tlsutil.Configurator) net.Conn {
		wrapper := tlsutil.SpecificDC("dc1", c.OutgoingRPCWrapper())
		tlsEnabled := func(_ raft.ServerAddress) bool {
			return true
		}

		rl := NewRaftLayer(nil, nil, wrapper, tlsEnabled)
		conn, err := rl.Dial(raft.ServerAddress(srv.Listener.Addr().String()), 100*time.Millisecond)
		require.NoError(t, err)
		return conn
	}

	useNativeTLS := func(t *testing.T, c *tlsutil.Configurator) net.Conn {
		wrapper := c.OutgoingALPNRPCWrapper()
		dialer := &net.Dialer{Timeout: 100 * time.Millisecond}

		rawConn, err := dialer.Dial("tcp", srv.Listener.Addr().String())
		require.NoError(t, err)

		tlsConn, err := wrapper("dc1", "srv1", pool.ALPN_RPCRaft, rawConn)
		require.NoError(t, err)
		return tlsConn
	}

	setupAgentTLSCert := func(name string) func(t *testing.T) string {
		return func(t *testing.T) string {
			newCert(t, caPEM, caPK, "node1", name)
			return filepath.Join(dir, "node1-"+name)
		}
	}

	setupAgentTLSCertWithIntermediate := func(name string) func(t *testing.T) string {
		return func(t *testing.T) string {
			newCert(t, intermediatePEM, intermediatePK, "node1", name)
			certPrefix := filepath.Join(dir, "node1-"+name)
			f, err := os.OpenFile(certPrefix+".pem", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.Write([]byte(intermediatePEM)); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			return certPrefix
		}
	}

	setupConnectCACert := func(name string) func(t *testing.T) string {
		return func(t *testing.T) string {
			_, caRoot := srv.caManager.getCAProvider()
			newCert(t, caRoot.RootCert, connectCApk, "node1", name)
			return filepath.Join(dir, "node1-"+name)
		}
	}

	type testCase struct {
		name        string
		conn        func(t *testing.T, c *tlsutil.Configurator) net.Conn
		setupCert   func(t *testing.T) string
		expectError bool
	}

	run := func(t *testing.T, tc testCase) {
		certPath := tc.setupCert(t)

		cfg := tlsutil.Config{
			InternalRPC: tlsutil.ProtocolConfig{
				VerifyOutgoing:       true,
				VerifyServerHostname: true,
				CAFile:               filepath.Join(dir, "ca.pem"),
				CertFile:             certPath + ".pem",
				KeyFile:              certPath + ".key",
			},
			Domain: "consul",
		}
		c, err := tlsutil.NewConfigurator(cfg, hclog.New(nil))
		require.NoError(t, err)

		_, err = doRaftRPC(tc.conn(t, c), srv.config.NodeName)
		if tc.expectError {
			if !isConnectionClosedError(err) {
				t.Fatalf("expected a connection closed error, got: %v", err)
			}
			return
		}
		require.NoError(t, err)
	}

	var testCases = []testCase{
		{
			name:        "TLS byte with client cert",
			setupCert:   setupAgentTLSCert("client.dc1.consul"),
			conn:        useTLSByte,
			expectError: true,
		},
		{
			name:        "TLS byte with server cert in different DC",
			setupCert:   setupAgentTLSCert("server.dc2.consul"),
			conn:        useTLSByte,
			expectError: true,
		},
		{
			name:      "TLS byte with server cert in same DC",
			setupCert: setupAgentTLSCert("server.dc1.consul"),
			conn:      useTLSByte,
		},
		{
			name:      "TLS byte with server cert in same DC and with unknown intermediate",
			setupCert: setupAgentTLSCertWithIntermediate("server.dc1.consul"),
			conn:      useTLSByte,
		},
		{
			name:        "TLS byte with ConnectCA leaf cert",
			setupCert:   setupConnectCACert("server.dc1.consul"),
			conn:        useTLSByte,
			expectError: true,
		},
		{
			name:        "native TLS with client cert",
			setupCert:   setupAgentTLSCert("client.dc1.consul"),
			conn:        useNativeTLS,
			expectError: true,
		},
		{
			name:        "native TLS with server cert in different DC",
			setupCert:   setupAgentTLSCert("server.dc2.consul"),
			conn:        useNativeTLS,
			expectError: true,
		},
		{
			name:      "native TLS with server cert in same DC",
			setupCert: setupAgentTLSCert("server.dc1.consul"),
			conn:      useNativeTLS,
		},
		{
			name:        "native TLS with ConnectCA leaf cert",
			setupCert:   setupConnectCACert("server.dc1.consul"),
			conn:        useNativeTLS,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestGetWaitTime(t *testing.T) {
	type testCase struct {
		name           string
		RPCHoldTimeout time.Duration
		expected       time.Duration
		retryCount     int
	}
	config := DefaultConfig()

	run := func(t *testing.T, tc testCase) {
		config.RPCHoldTimeout = tc.RPCHoldTimeout
		require.Equal(t, tc.expected, getWaitTime(config.RPCHoldTimeout, tc.retryCount))
	}

	var testCases = []testCase{
		{
			name:           "init backoff small",
			RPCHoldTimeout: 7 * time.Millisecond,
			retryCount:     1,
			expected:       1 * time.Millisecond,
		},
		{
			name:           "first attempt",
			RPCHoldTimeout: 7 * time.Second,
			retryCount:     1,
			expected:       437 * time.Millisecond,
		},
		{
			name:           "second attempt",
			RPCHoldTimeout: 7 * time.Second,
			retryCount:     2,
			expected:       874 * time.Millisecond,
		},
		{
			name:           "third attempt",
			RPCHoldTimeout: 7 * time.Second,
			retryCount:     3,
			expected:       1748 * time.Millisecond,
		},
		{
			name:           "fourth attempt",
			RPCHoldTimeout: 7 * time.Second,
			retryCount:     4,
			expected:       3496 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func doRaftRPC(conn net.Conn, leader string) (raft.AppendEntriesResponse, error) {
	var resp raft.AppendEntriesResponse

	var term uint64 = 0xc
	a := raft.AppendEntriesRequest{
		RPCHeader:         raft.RPCHeader{ProtocolVersion: 3},
		Term:              0,
		Leader:            []byte(leader),
		PrevLogEntry:      0,
		PrevLogTerm:       term,
		LeaderCommitIndex: 50,
	}

	if err := appendEntries(conn, a, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func appendEntries(conn net.Conn, req raft.AppendEntriesRequest, resp *raft.AppendEntriesResponse) error {
	w := bufio.NewWriter(conn)
	enc := codec.NewEncoder(w, &codec.MsgpackHandle{})

	const rpcAppendEntries = 0
	if err := w.WriteByte(rpcAppendEntries); err != nil {
		return fmt.Errorf("failed to write raft-RPC byte: %w", err)
	}

	if err := enc.Encode(req); err != nil {
		return fmt.Errorf("failed to send append entries RPC: %w", err)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush RPC: %w", err)
	}

	if err := decodeRaftRPCResponse(conn, resp); err != nil {
		return fmt.Errorf("response error: %w", err)
	}
	return nil
}

// copied and modified from raft/net_transport.go
func decodeRaftRPCResponse(conn net.Conn, resp *raft.AppendEntriesResponse) error {
	r := bufio.NewReader(conn)
	dec := codec.NewDecoder(r, &codec.MsgpackHandle{})

	var rpcError string
	if err := dec.Decode(&rpcError); err != nil {
		return fmt.Errorf("failed to decode response error: %w", err)
	}
	if err := dec.Decode(resp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	if rpcError != "" {
		return fmt.Errorf("rpc error: %v", rpcError)
	}
	return nil
}

func isConnectionClosedError(err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, io.EOF):
		return true
	case strings.Contains(err.Error(), "connection reset by peer"):
		return true
	default:
		return false
	}
}

func getFirstSubscribeEventOrError(conn *grpc.ClientConn, req *pbsubscribe.SubscribeRequest) (*pbsubscribe.Event, error) {
	streamClient := pbsubscribe.NewStateChangeSubscriptionClient(conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handle, err := streamClient.Subscribe(ctx, req)
	if err != nil {
		return nil, err
	}

	event, err := handle.Recv()
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return event, nil
}

// channelCallRPC lets you execute an RPC async. Helpful in some
// tests.
func channelCallRPC(
	srv *Server,
	method string,
	args interface{},
	resp interface{},
	responseInterceptor func() error,
) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		codec, err := rpcClientNoClose(srv)
		if err != nil {
			errCh <- err
			return
		}
		defer codec.Close()

		err = msgpackrpc.CallWithCodec(codec, method, args, resp)
		if err == nil && responseInterceptor != nil {
			err = responseInterceptor()
		}
		errCh <- err
	}()
	return errCh
}

// rpcBlockingQueryTestHarness is specifically meant to test the
// errNotFound and errNotChanged mechanisms in blockingQuery()
func rpcBlockingQueryTestHarness(
	t *testing.T,
	readQueryFn func(minQueryIndex uint64) (*structs.QueryMeta, <-chan error),
	noisyWriteFn func(i int) <-chan error,
) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	launchWriters := func() {
		defer cancel()

		for i := 0; i < 200; i++ {
			time.Sleep(5 * time.Millisecond)

			errCh := noisyWriteFn(i)
			select {
			case <-ctx.Done():
				return
			case err := <-errCh:
				if err != nil {
					t.Errorf("[%d] unexpected error: %v", i, err)
					return
				}
			}
		}
	}

	var (
		count         int
		minQueryIndex uint64
	)

	for ctx.Err() == nil {
		// The first iteration is an orientation iteration, as we don't pass an
		// index value so there is no actual blocking that will happen.
		//
		// Since the data is not changing, we don't expect the second iteration
		// to return soon, so we wait a bit after kicking it off before
		// launching the write-storm.
		var timerCh <-chan time.Time
		if count == 1 {
			timerCh = time.After(50 * time.Millisecond)
		}

		qm, errCh := readQueryFn(minQueryIndex)

	RESUME:
		select {
		case err := <-errCh:
			if err != nil {
				require.NoError(t, err)
			}

			t.Log("blocking query index", qm.Index)
			count++
			minQueryIndex = qm.Index

		case <-timerCh:
			timerCh = nil
			go launchWriters()
			goto RESUME

		case <-ctx.Done():
			break
		}
	}

	require.Equal(t, 1, count, "if this fails, then the timer likely needs to be increased above")
}
