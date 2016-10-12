package consul

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
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
			Op:         structs.KVSSet,
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
	addr := s.config.RPCAddr
	snap, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Close()
	args := structs.SnapshotRequest{
		Datacenter: dc,
		Token:      token,
		Op:         structs.SnapshotSave,
	}
	if err := SnapshotRPC(snap, &args, bytes.NewReader([]byte(""))); err != nil {
		t.Fatalf("err: %v", err)
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
		if string(d.Value) != "hello" {
			t.Fatalf("bad: %v", d)
		}
	}

	// Set a key to an after value.
	{
		args := structs.KVSRequest{
			Datacenter: dc,
			Op:         structs.KVSSet,
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
	restore, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer restore.Close()
	args.Op = structs.SnapshotRestore
	if err := SnapshotRPC(restore, &args, snap); err != nil {
		t.Fatalf("err: %v", err)
	}

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

	testutil.WaitForLeader(t, s1.RPC, "dc1")
	verifySnapshot(t, s1, "dc1", "")
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

	testutil.WaitForLeader(t, s1.RPC, "dc1")

	// Take a snapshot.
	func() {
		addr := s1.config.RPCAddr
		snap, err := net.DialTimeout("tcp", addr.String(), time.Second)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		defer snap.Close()
		args := structs.SnapshotRequest{
			Datacenter: "dc1",
			Op:         structs.SnapshotSave,
		}
		err = SnapshotRPC(snap, &args, bytes.NewReader([]byte("")))
		if err == nil || !strings.Contains(err.Error(), permissionDenied) {
			t.Fatalf("err: %v", err)
		}
	}()

	// Restore a snapshot.
	func() {
		addr := s1.config.RPCAddr
		snap, err := net.DialTimeout("tcp", addr.String(), time.Second)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		defer snap.Close()
		args := structs.SnapshotRequest{
			Datacenter: "dc1",
			Op:         structs.SnapshotRestore,
		}
		err = SnapshotRPC(snap, &args, bytes.NewReader([]byte("")))
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
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.WaitForLeader(t, s1.RPC, "dc1")
	testutil.WaitForLeader(t, s2.RPC, "dc1")

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

	testutil.WaitForLeader(t, s1.RPC, "dc1")
	testutil.WaitForLeader(t, s2.RPC, "dc2")

	// Try to WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	testutil.WaitForResult(
		func() (bool, error) {
			return len(s1.WANMembers()) > 1, nil
		},
		func(err error) {
			t.Fatalf("Failed waiting for WAN join: %v", err)
		})

	// Run a snapshot from each server locally and remotely to ensure we
	// forward.
	for _, s := range []*Server{s1, s2} {
		verifySnapshot(t, s, "dc1", "")
		verifySnapshot(t, s, "dc2", "")
	}
}
