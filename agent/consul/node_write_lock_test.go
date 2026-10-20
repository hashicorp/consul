// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestNodeWriteLockIndex(t *testing.T) {
	require.Equal(t,
		nodeWriteLockIndex("default", "", "Node-A"),
		nodeWriteLockIndex("default", "", "node-a"),
		"node names are matched case-insensitively by the state store")

	idx := nodeWriteLockIndex("default", "", "node-a")
	require.GreaterOrEqual(t, idx, 0)
	require.Less(t, idx, nodeWriteLockStripes)
}

func TestNodeWriteLocks_DuplicateIndexes(t *testing.T) {
	var locks nodeWriteLocks
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Locking the same stripe twice in one call must not self-deadlock.
		unlock := locks.lock([]int{5, 3, 5, 3})
		unlock()
		unlock = locks.lock([]int{3, 5})
		unlock()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("lock with duplicate indexes deadlocked")
	}
}

func TestNodeWriteLocks_CatalogWritesShareStripe(t *testing.T) {
	// Find two nodes on different, non-zero stripes.
	idxA, idxB := -1, -1
	for i := 0; idxB < 0; i++ {
		idx := nodeWriteLockIndex("default", "", fmt.Sprintf("node-%d", i))
		switch {
		case idx == 0:
		case idxA < 0:
			idxA = idx
		case idx != idxA:
			idxB = idx
		}
	}

	var locks nodeWriteLocks
	unlockA := locks.lock([]int{idxA})

	acquired := make(chan func())
	go func() { acquired <- locks.lock([]int{idxB}) }()

	select {
	case unlockB := <-acquired:
		unlockB()
		unlockA()
		t.Fatal("writes to different nodes must share a stripe, but the second write did not wait")
	case <-time.After(200 * time.Millisecond):
	}

	unlockA()
	select {
	case unlockB := <-acquired:
		unlockB()
	case <-time.After(5 * time.Second):
		t.Fatal("second write did not acquire the lock after the first released it")
	}
}

func TestNodeWriteLocks_NoIndexesTakesNoLock(t *testing.T) {
	var locks nodeWriteLocks
	unlock := locks.lock([]int{nodeWriteLockIndex("default", "", "node-a")})
	defer unlock()

	done := make(chan struct{})
	go func() {
		locks.lock(nil)()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a request without catalog operations must not wait for catalog write locks")
	}
}

func TestRegisterNodeWriteLockIndexes(t *testing.T) {
	args := &structs.RegisterRequest{
		Node: "node-a",
		Checks: []*structs.HealthCheck{
			{Node: "node-a", CheckID: "c1"},
			{Node: "node-b", CheckID: "c2"},
		},
	}
	require.ElementsMatch(t, []int{
		nodeWriteLockIndex("default", "", "node-a"),
		nodeWriteLockIndex("default", "", "node-a"),
		nodeWriteLockIndex("default", "", "node-b"),
	}, registerNodeWriteLockIndexes(args, "default"))
}

func TestTxnNodeWriteLockIndexes(t *testing.T) {
	ops := structs.TxnOps{
		{KV: &structs.TxnKVOp{Verb: api.KVSet, DirEnt: structs.DirEntry{Key: "k"}}},
		{Node: &structs.TxnNodeOp{Verb: api.NodeSet, Node: structs.Node{Node: "node-a"}}},
		{Service: &structs.TxnServiceOp{Verb: api.ServiceSet, Node: "node-b", Service: structs.NodeService{ID: "s"}}},
		{Check: &structs.TxnCheckOp{Verb: api.CheckSet, Check: structs.HealthCheck{Node: "node-c", CheckID: "c"}}},
	}
	require.ElementsMatch(t, []int{
		nodeWriteLockIndex("default", "", "node-a"),
		nodeWriteLockIndex("default", "", "node-b"),
		nodeWriteLockIndex("default", "", "node-c"),
	}, txnNodeWriteLockIndexes(ops))
}

// requireBlockedUntilUnlocked holds the node's write lock, starts call, and
// verifies that call only completes after the lock is released.
func requireBlockedUntilUnlocked(t *testing.T, s *Server, node string, call func() error) {
	t.Helper()
	unlock := s.nodeWriteLocks.lock([]int{nodeWriteLockIndex("default", "", node)})

	errCh := make(chan error, 1)
	go func() { errCh <- call() }()

	select {
	case err := <-errCh:
		unlock()
		t.Fatalf("write completed while the node write lock was held: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	unlock()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("write did not complete after the node write lock was released")
	}
}

func TestCatalog_Register_WaitsForNodeWriteLock(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir, s := testServer(t)
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	codec := rpcClient(t, s)
	defer codec.Close()

	requireBlockedUntilUnlocked(t, s, "lock-node", func() error {
		args := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "lock-node",
			Address:    "127.0.0.1",
			Checks: []*structs.HealthCheck{{
				Node:    "lock-node",
				CheckID: "lock-check",
				Name:    "lock check",
				Status:  api.HealthPassing,
			}},
		}
		var out struct{}
		return msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out)
	})
}

func TestTxn_Apply_WaitsForNodeWriteLock(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir, s := testServer(t)
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	codec := rpcClient(t, s)
	defer codec.Close()

	var out struct{}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "lock-node",
		Address:    "127.0.0.1",
	}, &out))

	requireBlockedUntilUnlocked(t, s, "lock-node", func() error {
		args := structs.TxnRequest{
			Datacenter: "dc1",
			Ops: structs.TxnOps{{
				Check: &structs.TxnCheckOp{
					Verb: api.CheckSet,
					Check: structs.HealthCheck{
						Node:    "lock-node",
						CheckID: "lock-check",
						Name:    "lock check",
						Status:  api.HealthPassing,
					},
				},
			}},
		}
		var reply structs.TxnResponse
		if err := msgpackrpc.CallWithCodec(codec, "Txn.Apply", &args, &reply); err != nil {
			return err
		}
		if len(reply.Errors) > 0 {
			return fmt.Errorf("txn errors: %v", reply.Errors)
		}
		return nil
	})
}

func TestCatalog_Deregister_WaitsForNodeWriteLock(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir, s := testServer(t)
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	codec := rpcClient(t, s)
	defer codec.Close()

	var out struct{}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "lock-node",
		Address:    "127.0.0.1",
		Checks: []*structs.HealthCheck{{
			Node:    "lock-node",
			CheckID: "lock-check",
			Name:    "lock check",
			Status:  api.HealthPassing,
		}},
	}, &out))

	requireBlockedUntilUnlocked(t, s, "lock-node", func() error {
		args := structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "lock-node",
			CheckID:    "lock-check",
		}
		var out struct{}
		return msgpackrpc.CallWithCodec(codec, "Catalog.Deregister", &args, &out)
	})
}

func TestTxn_Apply_KVOnlyDoesNotWaitForNodeWriteLock(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir, s := testServer(t)
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	testrpc.WaitForLeader(t, s.RPC, "dc1")
	codec := rpcClient(t, s)
	defer codec.Close()

	unlock := s.nodeWriteLocks.lock([]int{nodeWriteLockIndex("default", "", "lock-node")})
	defer unlock()

	errCh := make(chan error, 1)
	go func() {
		args := structs.TxnRequest{
			Datacenter: "dc1",
			Ops: structs.TxnOps{{
				KV: &structs.TxnKVOp{
					Verb:   api.KVSet,
					DirEnt: structs.DirEntry{Key: "kv-only", Value: []byte("v")},
				},
			}},
		}
		var reply structs.TxnResponse
		err := msgpackrpc.CallWithCodec(codec, "Txn.Apply", &args, &reply)
		if err == nil && len(reply.Errors) > 0 {
			err = fmt.Errorf("txn errors: %v", reply.Errors)
		}
		errCh <- err
	}()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("KV-only transaction waited for the catalog write lock")
	}
}

// TestCatalog_Register_ConcurrentCheckOwnership races a victim and an attacker
// registering the same new CheckID on a shared node. Each one is only
// authorized for its own service, so exactly one registration may succeed: the
// other must be denied once it observes the check the first one created.
func TestCatalog_Register_ConcurrentCheckOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir, s := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	testrpc.WaitForTestAgent(t, s.RPC, "dc1", testrpc.WithToken("root"))

	setupCodec := rpcClient(t, s)
	defer setupCodec.Close()

	var out struct{}
	for _, svc := range []string{"victim-svc", "attacker-svc"} {
		require.NoError(t, msgpackrpc.CallWithCodec(setupCodec, "Catalog.Register", &structs.RegisterRequest{
			Datacenter:   "dc1",
			Node:         "shared-node",
			Address:      "127.0.0.1",
			Service:      &structs.NodeService{ID: svc, Service: svc, Port: 8080},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}, &out))
	}
	victimToken := createTokenWithPolicyNameFull(t, setupCodec, "race-victim", `service "victim-svc" { policy = "write" }`, "root").SecretID
	attackerToken := createTokenWithPolicyNameFull(t, setupCodec, "race-attacker", `service "attacker-svc" { policy = "write" }`, "root").SecretID

	victimCodec := rpcClient(t, s)
	defer victimCodec.Close()
	attackerCodec := rpcClient(t, s)
	defer attackerCodec.Close()

	for i := 0; i < 40; i++ {
		checkID := fmt.Sprintf("race-check-%d", i)
		var wg sync.WaitGroup
		start := make(chan struct{})
		var victimErr, attackerErr error

		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			victimErr = registerRaceCheck(victimCodec, checkID, "victim-svc", victimToken)
		}()
		go func() {
			defer wg.Done()
			<-start
			attackerErr = registerRaceCheck(attackerCodec, checkID, "attacker-svc", attackerToken)
		}()
		close(start)
		wg.Wait()

		require.False(t, victimErr == nil && attackerErr == nil,
			"iteration %d: both registrations of %q succeeded, so one overwrote the other's check", i, checkID)
		require.False(t, victimErr != nil && attackerErr != nil,
			"iteration %d: both registrations failed: victim=%v attacker=%v", i, victimErr, attackerErr)
		if victimErr != nil {
			require.True(t, acl.IsErrPermissionDenied(victimErr), "victim: %v", victimErr)
		} else {
			require.True(t, acl.IsErrPermissionDenied(attackerErr), "attacker: %v", attackerErr)
		}
	}
}

func registerRaceCheck(codec rpc.ClientCodec, checkID, serviceID, token string) error {
	args := structs.RegisterRequest{
		Datacenter:     "dc1",
		Node:           "shared-node",
		SkipNodeUpdate: true,
		Checks: []*structs.HealthCheck{{
			Node:      "shared-node",
			CheckID:   types.CheckID(checkID),
			Name:      checkID,
			Status:    api.HealthPassing,
			ServiceID: serviceID,
		}},
		WriteRequest: structs.WriteRequest{Token: token},
	}
	var out struct{}
	return msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out)
}
