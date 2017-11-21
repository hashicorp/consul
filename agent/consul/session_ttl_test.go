package consul

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/net-rpc-msgpackrpc"
)

func TestInitializeSessionTimers(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	state := s1.fsm.State()
	if err := state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %s", err)
	}
	session := &structs.Session{
		ID:   generateUUID(),
		Node: "foo",
		TTL:  "10s",
	}
	if err := state.SessionCreate(100, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Reset the session timers
	err := s1.initializeSessionTimers()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that we have a timer
	if s1.sessionTimers.Get(session.ID) == nil {
		t.Fatalf("missing session timer")
	}
}

func TestResetSessionTimer_Fault(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Should not exist
	err := s1.resetSessionTimer(generateUUID(), nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err: %v", err)
	}

	// Create a session
	state := s1.fsm.State()
	if err := state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %s", err)
	}
	session := &structs.Session{
		ID:   generateUUID(),
		Node: "foo",
		TTL:  "10s",
	}
	if err := state.SessionCreate(100, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Reset the session timer
	err = s1.resetSessionTimer(session.ID, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that we have a timer
	if s1.sessionTimers.Get(session.ID) == nil {
		t.Fatalf("missing session timer")
	}
}

func TestResetSessionTimer_NoTTL(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create a session
	state := s1.fsm.State()
	if err := state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %s", err)
	}
	session := &structs.Session{
		ID:   generateUUID(),
		Node: "foo",
		TTL:  "0000s",
	}
	if err := state.SessionCreate(100, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Reset the session timer
	err := s1.resetSessionTimer(session.ID, session)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that we have a timer
	if s1.sessionTimers.Get(session.ID) != nil {
		t.Fatalf("should not have session timer")
	}
}

func TestResetSessionTimer_InvalidTTL(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	// Create a session
	session := &structs.Session{
		ID:   generateUUID(),
		Node: "foo",
		TTL:  "foo",
	}

	// Reset the session timer
	err := s1.resetSessionTimer(session.ID, session)
	if err == nil || !strings.Contains(err.Error(), "Invalid Session TTL") {
		t.Fatalf("err: %v", err)
	}
}

func TestResetSessionTimerLocked(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	s1.createSessionTimer("foo", 5*time.Millisecond)
	if s1.sessionTimers.Get("foo") == nil {
		t.Fatalf("missing timer")
	}

	time.Sleep(10 * time.Millisecond * structs.SessionTTLMultiplier)
	if s1.sessionTimers.Get("foo") != nil {
		t.Fatalf("timer should be gone")
	}
}

func TestResetSessionTimerLocked_Renew(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	ttl := 100 * time.Millisecond

	// create the timer
	s1.createSessionTimer("foo", ttl)
	if s1.sessionTimers.Get("foo") == nil {
		t.Fatalf("missing timer")
	}

	// wait until it is "expired" but at this point
	// the session still exists.
	time.Sleep(ttl)
	if s1.sessionTimers.Get("foo") == nil {
		t.Fatal("missing timer")
	}

	// renew the session which will reset the TTL to 2*ttl
	// since that is the current SessionTTLMultiplier
	s1.createSessionTimer("foo", ttl)

	// Watch for invalidation
	renew := time.Now()
	deadline := renew.Add(2 * structs.SessionTTLMultiplier * ttl)
	for {
		now := time.Now()
		if now.After(deadline) {
			t.Fatal("should have expired by now")
		}

		// timer still exists
		if s1.sessionTimers.Get("foo") != nil {
			time.Sleep(time.Millisecond)
			continue
		}

		// timer gone
		if now.Sub(renew) < ttl {
			t.Fatalf("early invalidate")
		}
		break
	}
}

func TestInvalidateSession(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create a session
	state := s1.fsm.State()
	if err := state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %s", err)
	}
	session := &structs.Session{
		ID:   generateUUID(),
		Node: "foo",
		TTL:  "10s",
	}
	if err := state.SessionCreate(100, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// This should cause a destroy
	s1.invalidateSession(session.ID)

	// Check it is gone
	_, sess, err := state.SessionGet(nil, session.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sess != nil {
		t.Fatalf("should destroy session")
	}
}

func TestClearSessionTimer(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	s1.createSessionTimer("foo", 5*time.Millisecond)

	err := s1.clearSessionTimer("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if s1.sessionTimers.Get("foo") != nil {
		t.Fatalf("timer should be gone")
	}
}

func TestClearAllSessionTimers(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	s1.createSessionTimer("foo", 10*time.Millisecond)
	s1.createSessionTimer("bar", 10*time.Millisecond)
	s1.createSessionTimer("baz", 10*time.Millisecond)

	err := s1.clearAllSessionTimers()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// sessionTimers is guarded by the lock
	if s1.sessionTimers.Len() != 0 {
		t.Fatalf("timers should be gone")
	}
}

func TestServer_SessionTTL_Failover(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()
	servers := []*Server{s1, s2, s3}

	// Try to join
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s1, 3)) })

	// Find the leader
	var leader *Server
	for _, s := range servers {
		// Check that s.sessionTimers is empty
		if s.sessionTimers.Len() != 0 {
			t.Fatalf("should have no sessionTimers")
		}
		// Find the leader too
		if s.IsLeader() {
			leader = s
		}
	}
	if leader == nil {
		t.Fatalf("Should have a leader")
	}

	codec := rpcClient(t, leader)
	defer codec.Close()

	// Register a node
	node := structs.RegisterRequest{
		Datacenter: s1.config.Datacenter,
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	var out struct{}
	if err := s1.RPC("Catalog.Register", &node, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a TTL session
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
			TTL:  "10s",
		},
	}
	var id1 string
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &id1); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that sessionTimers has the session ID
	if leader.sessionTimers.Get(id1) == nil {
		t.Fatalf("missing session timer")
	}

	// Shutdown the leader!
	leader.Shutdown()

	// sessionTimers should be cleared on leader shutdown
	if leader.sessionTimers.Len() != 0 {
		t.Fatalf("session timers should be empty on the shutdown leader")
	}
	// Find the new leader
	retry.Run(t, func(r *retry.R) {
		leader = nil
		for _, s := range servers {
			if s.IsLeader() {
				leader = s
			}
		}
		if leader == nil {
			r.Fatal("Should have a new leader")
		}

		// Ensure session timer is restored
		if leader.sessionTimers.Get(id1) == nil {
			r.Fatal("missing session timer")
		}
	})
}
