package consul

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRPC_NoLeader_Fail(t *testing.T) {
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
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
}

func TestRPC_NoLeader_Fail_on_stale_read(t *testing.T) {
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

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	if err := msgpackrpc.CallWithCodec(codec, "KVS.ListKeys", &getKeysReq, &keyList); err != nil {
		t.Fatalf("Did not expect any error but got err: %v", err)
	}
}

func TestRPC_NoLeader_Retry(t *testing.T) {
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
	if ok, _ := s1.getLeader(); ok {
		t.Fatalf("should not have a leader yet")
	}

	// The timeout is long enough to ride out any reasonable leader
	// election.
	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
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

func TestRPC_blockingQuery(t *testing.T) {
	t.Parallel()
	dir, s := testServer(t)
	defer os.RemoveAll(dir)
	defer s.Shutdown()

	require := require.New(t)
	assert := assert.New(t)

	// Perform a non-blocking query. Note that it's significant that the meta has
	// a zero index in response - the implied opts.MinQueryIndex is also zero but
	// this should not block still.
	{
		var opts structs.QueryOptions
		var meta structs.QueryMeta
		var calls int
		fn := func(ws memdb.WatchSet, state *state.Store) error {
			calls++
			return nil
		}
		if err := s.blockingQuery(&opts, &meta, fn); err != nil {
			t.Fatalf("err: %v", err)
		}
		if calls != 1 {
			t.Fatalf("bad: %d", calls)
		}
	}

	// Perform a blocking query that gets woken up and loops around once.
	{
		opts := structs.QueryOptions{
			MinQueryIndex: 3,
		}
		var meta structs.QueryMeta
		var calls int
		fn := func(ws memdb.WatchSet, state *state.Store) error {
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
		if err := s.blockingQuery(&opts, &meta, fn); err != nil {
			t.Fatalf("err: %v", err)
		}
		if calls != 2 {
			t.Fatalf("bad: %d", calls)
		}
	}

	// Perform a blocking query that returns a zero index from blocking func (e.g.
	// no state yet). This should still return an empty response immediately, but
	// with index of 1 and then block on the next attempt. In one sense zero index
	// is not really a valid response from a state method that is not an error but
	// in practice a lot of state store operations do return it unless they
	// explicitly special checks to turn 0 into 1. Often this is not caught or
	// covered by tests but eventually when hit in the wild causes blocking
	// clients to busy loop and burn CPU. This test ensure that blockingQuery
	// systematically does the right thing to prevent future bugs like that.
	{
		opts := structs.QueryOptions{
			MinQueryIndex: 0,
		}
		var meta structs.QueryMeta
		var calls int
		fn := func(ws memdb.WatchSet, state *state.Store) error {
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
		require.NoError(s.blockingQuery(&opts, &meta, fn))
		assert.Equal(1, calls)
		assert.Equal(uint64(1), meta.Index,
			"expect fake index of 1 to force client to block on next update")

		// Simulate client making next request
		opts.MinQueryIndex = 1
		opts.MaxQueryTime = 20 * time.Millisecond // Don't wait too long

		// This time we should block even though the func returns index 0 still
		t0 := time.Now()
		require.NoError(s.blockingQuery(&opts, &meta, fn))
		t1 := time.Now()
		assert.Equal(2, calls)
		assert.Equal(uint64(1), meta.Index,
			"expect fake index of 1 to force client to block on next update")
		assert.True(t1.Sub(t0) > 20*time.Millisecond,
			"should have actually blocked waiting for timeout")

	}

	// Perform a query that blocks and gets interrupted when the state store
	// is abandoned.
	{
		opts := structs.QueryOptions{
			MinQueryIndex: 3,
		}
		var meta structs.QueryMeta
		var calls int
		fn := func(ws memdb.WatchSet, state *state.Store) error {
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
		if err := s.blockingQuery(&opts, &meta, fn); err != nil {
			t.Fatalf("err: %v", err)
		}
		if calls != 1 {
			t.Fatalf("bad: %d", calls)
		}
	}
}

func TestRPC_ReadyForConsistentReads(t *testing.T) {
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
