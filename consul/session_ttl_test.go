package consul

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
)

func TestServer_sessionTTL(t *testing.T) {
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
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := s3.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.raftPeers.Peers()
			return len(peers) == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}

	// Find the leader
	var leader *Server
	for _, s := range servers {
		// check that s.sessionTimers is empty
		if len(s.sessionTimers) != 0 {
			t.Fatalf("should have no sessionTimers")
		}
		// find the leader too
		if s.IsLeader() {
			leader = s
		}
	}

	if leader == nil {
		t.Fatalf("Should have a leader")
	}

	client := rpcClient(t, leader)
	defer client.Close()

	leader.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})

	// create a TTL session
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
			TTL:  "10s",
		},
	}
	var id1 string
	if err := client.Call("Session.Apply", &arg, &id1); err != nil {
		t.Fatalf("err: %v", err)
	}

	// check that leader.sessionTimers has the session id in it
	// means initializeSessionTimers was called and resetSessionTimer was called
	if len(leader.sessionTimers) == 0 || leader.sessionTimers[id1] == nil {
		t.Fatalf("sessionTimers not initialized and does not contain session timer for session")
	}

	time.Sleep(100 * time.Millisecond)
	leader.Leave()
	leader.Shutdown()

	// leader.sessionTimers should be empty due to clearAllSessionTimers getting called
	if len(leader.sessionTimers) != 0 {
		t.Fatalf("session timers should be empty on the shutdown leader")
	}

	time.Sleep(100 * time.Millisecond)

	var remain *Server
	for _, s := range servers {
		if s == leader {
			continue
		}
		remain = s
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.raftPeers.Peers()
			return len(peers) == 2, errors.New(fmt.Sprintf("%v", peers))
		}, func(err error) {
			t.Fatalf("should have 2 peers: %v", err)
		})
	}

	// Verify the old leader is deregistered
	state := remain.fsm.State()
	testutil.WaitForResult(func() (bool, error) {
		_, found, _ := state.GetNode(leader.config.NodeName)
		return !found, nil
	}, func(err error) {
		t.Fatalf("leader should be deregistered")
	})

	// Find the new leader
	leader = nil
	for _, s := range servers {
		// find the leader too
		if s.IsLeader() {
			leader = s
		}
	}

	if leader == nil {
		t.Fatalf("Should have a new leader")
	}

	// check that new leader.sessionTimers has the session id in it
	if len(leader.sessionTimers) == 0 || leader.sessionTimers[id1] == nil {
		t.Fatalf("sessionTimers not initialized and does not contain session timer for session")
	}

	// create another TTL session with the same parameters
	var id2 string
	if err := client.Call("Session.Apply", &arg, &id2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(leader.sessionTimers) != 2 {
		t.Fatalf("sessionTimes length should be 2")
	}

	// destroy the id1 session (test clearSessionTimer)
	arg.Op = structs.SessionDestroy
	arg.Session.ID = id1
	if err := client.Call("Session.Apply", &arg, &id1); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(leader.sessionTimers) != 1 {
		t.Fatalf("sessionTimers length should 1")
	}

	// destroy the id2 session (test clearSessionTimer)
	arg.Op = structs.SessionDestroy
	arg.Session.ID = id2
	if err := client.Call("Session.Apply", &arg, &id2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(leader.sessionTimers) != 0 {
		t.Fatalf("sessionTimers length should be 0")
	}
}
