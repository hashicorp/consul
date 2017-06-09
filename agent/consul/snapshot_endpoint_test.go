package consul

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/net-rpc-msgpackrpc"
)

// verifySnapshot is a helper that does a snapshot and restore.
func verifySnapshot(t *testing.T, s *Server, dc, token string) {
	codec := rpcClient(t, s)
	defer codec.Close()

	// Set a key to a before value.
	{
		args := structs.KVSRequest{
			Datacenter: dc,
			Op:         api.KVSet,
			DirEnt: structs.DirEntry{
				Key:   "test",
				Value: []byte("hello"),
			},
			WriteRequest: structs.WriteRequest{
				Token: token,
			},
		}
		var out bool
		if err := msgpackrpc.CallWithCodec(codec, "KVS.Apply", &args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Take a snapshot.
	args := structs.SnapshotRequest{
		Datacenter: dc,
		Token:      token,
		Op:         structs.SnapshotSave,
	}
	var reply structs.SnapshotResponse
	snap, err := SnapshotRPC(s.connPool, s.config.Datacenter, s.config.RPCAddr, false,
		&args, bytes.NewReader([]byte("")), &reply)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Close()

	// Read back the before value.
	{
		getR := structs.KeyRequest{
			Datacenter: dc,
			Key:        "test",
			QueryOptions: structs.QueryOptions{
				Token: token,
			},
		}
		var dirent structs.IndexedDirEntries
		if err := msgpackrpc.CallWithCodec(codec, "KVS.Get", &getR, &dirent); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(dirent.Entries) != 1 {
			t.Fatalf("Bad: %v", dirent)
		}
		d := dirent.Entries[0]
		if string(d.Value) != "hello" {
			t.Fatalf("bad: %v", d)
		}
	}

	// Set a key to an after value.
	{
		args := structs.KVSRequest{
			Datacenter: dc,
			Op:         api.KVSet,
			DirEnt: structs.DirEntry{
				Key:   "test",
				Value: []byte("goodbye"),
			},
			WriteRequest: structs.WriteRequest{
				Token: token,
			},
		}
		var out bool
		if err := msgpackrpc.CallWithCodec(codec, "KVS.Apply", &args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Read back the before value.
	{
		getR := structs.KeyRequest{
			Datacenter: dc,
			Key:        "test",
			QueryOptions: structs.QueryOptions{
				Token: token,
			},
		}
		var dirent structs.IndexedDirEntries
		if err := msgpackrpc.CallWithCodec(codec, "KVS.Get", &getR, &dirent); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(dirent.Entries) != 1 {
			t.Fatalf("Bad: %v", dirent)
		}
		d := dirent.Entries[0]
		if string(d.Value) != "goodbye" {
			t.Fatalf("bad: %v", d)
		}
	}

	// Restore the snapshot.
	args.Op = structs.SnapshotRestore
	restore, err := SnapshotRPC(s.connPool, s.config.Datacenter, s.config.RPCAddr, false,
		&args, snap, &reply)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer restore.Close()

	// Read back the before value post-snapshot.
	{
		getR := structs.KeyRequest{
			Datacenter: dc,
			Key:        "test",
			QueryOptions: structs.QueryOptions{
				Token: token,
			},
		}
		var dirent structs.IndexedDirEntries
		if err := msgpackrpc.CallWithCodec(codec, "KVS.Get", &getR, &dirent); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(dirent.Entries) != 1 {
			t.Fatalf("Bad: %v", dirent)
		}
		d := dirent.Entries[0]
		if string(d.Value) != "hello" {
			t.Fatalf("bad: %v", d)
		}
	}
}

func TestSnapshot(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	verifySnapshot(t, s1, "dc1", "")
}

func TestSnapshot_LeaderState(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// Make a before session.
	var before string
	{
		args := structs.SessionRequest{
			Datacenter: s1.config.Datacenter,
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: s1.config.NodeName,
				TTL:  "60s",
			},
		}
		if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &args, &before); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Take a snapshot.
	args := structs.SnapshotRequest{
		Datacenter: s1.config.Datacenter,
		Op:         structs.SnapshotSave,
	}
	var reply structs.SnapshotResponse
	snap, err := SnapshotRPC(s1.connPool, s1.config.Datacenter, s1.config.RPCAddr, false,
		&args, bytes.NewReader([]byte("")), &reply)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Close()

	// Make an after session.
	var after string
	{
		args := structs.SessionRequest{
			Datacenter: s1.config.Datacenter,
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: s1.config.NodeName,
				TTL:  "60s",
			},
		}
		if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &args, &after); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Make sure the leader has timers setup.
	if _, ok := s1.sessionTimers[before]; !ok {
		t.Fatalf("missing session timer")
	}
	if _, ok := s1.sessionTimers[after]; !ok {
		t.Fatalf("missing session timer")
	}

	// Restore the snapshot.
	args.Op = structs.SnapshotRestore
	restore, err := SnapshotRPC(s1.connPool, s1.config.Datacenter, s1.config.RPCAddr, false,
		&args, snap, &reply)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer restore.Close()

	// Make sure the before time is still there, and that the after timer
	// got reverted. This proves we fully cycled the leader state.
	if _, ok := s1.sessionTimers[before]; !ok {
		t.Fatalf("missing session timer")
	}
	if _, ok := s1.sessionTimers[after]; ok {
		t.Fatalf("unexpected session timer")
	}
}

func TestSnapshot_ACLDeny(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Take a snapshot.
	func() {
		args := structs.SnapshotRequest{
			Datacenter: "dc1",
			Op:         structs.SnapshotSave,
		}
		var reply structs.SnapshotResponse
		_, err := SnapshotRPC(s1.connPool, s1.config.Datacenter, s1.config.RPCAddr, false,
			&args, bytes.NewReader([]byte("")), &reply)
		if err == nil || !strings.Contains(err.Error(), permissionDenied) {
			t.Fatalf("err: %v", err)
		}
	}()

	// Restore a snapshot.
	func() {
		args := structs.SnapshotRequest{
			Datacenter: "dc1",
			Op:         structs.SnapshotRestore,
		}
		var reply structs.SnapshotResponse
		_, err := SnapshotRPC(s1.connPool, s1.config.Datacenter, s1.config.RPCAddr, false,
			&args, bytes.NewReader([]byte("")), &reply)
		if err == nil || !strings.Contains(err.Error(), permissionDenied) {
			t.Fatalf("err: %v", err)
		}
	}()

	// With the token in place everything should go through.
	verifySnapshot(t, s1, "dc1", "root")
}

func TestSnapshot_Forward_Leader(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinLAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// Run against the leader and the follower to ensure we forward.
	for _, s := range []*Server{s1, s2} {
		verifySnapshot(t, s, "dc1", "")
		verifySnapshot(t, s, "dc1", "")
	}
}

func TestSnapshot_Forward_Datacenter(t *testing.T) {
	dir1, s1 := testServerDC(t, "dc1")
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Try to WAN join.
	joinWAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.WANMembers()), 2; got < want {
			r.Fatalf("got %d WAN members want at least %d", got, want)
		}
	})

	// Run a snapshot from each server locally and remotely to ensure we
	// forward.
	for _, s := range []*Server{s1, s2} {
		verifySnapshot(t, s, "dc1", "")
		verifySnapshot(t, s, "dc2", "")
	}
}

func TestSnapshot_AllowStale(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Run against the servers which aren't haven't been set up to establish
	// a leader and make sure we get a no leader error.
	for _, s := range []*Server{s1, s2} {
		// Take a snapshot.
		args := structs.SnapshotRequest{
			Datacenter: s.config.Datacenter,
			Op:         structs.SnapshotSave,
		}
		var reply structs.SnapshotResponse
		_, err := SnapshotRPC(s.connPool, s.config.Datacenter, s.config.RPCAddr, false,
			&args, bytes.NewReader([]byte("")), &reply)
		if err == nil || !strings.Contains(err.Error(), structs.ErrNoLeader.Error()) {
			t.Fatalf("err: %v", err)
		}
	}

	// Run in stale mode and make sure we get an error from Raft (snapshot
	// was attempted), and not a no leader error.
	for _, s := range []*Server{s1, s2} {
		// Take a snapshot.
		args := structs.SnapshotRequest{
			Datacenter: s.config.Datacenter,
			AllowStale: true,
			Op:         structs.SnapshotSave,
		}
		var reply structs.SnapshotResponse
		_, err := SnapshotRPC(s.connPool, s.config.Datacenter, s.config.RPCAddr, false,
			&args, bytes.NewReader([]byte("")), &reply)
		if err == nil || !strings.Contains(err.Error(), "Raft error when taking snapshot") {
			t.Fatalf("err: %v", err)
		}
	}
}
