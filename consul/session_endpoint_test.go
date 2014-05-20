package consul

import (
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"os"
	"testing"
)

func TestSessionEndpoint_Apply(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	// Just add a node
	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})

	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
		},
	}
	var out string
	if err := client.Call("Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	id := out

	// Verify
	state := s1.fsm.State()
	_, s, err := state.SessionGet(out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s == nil {
		t.Fatalf("should not be nil")
	}

	// Do a delete
	arg.Op = structs.SessionDestroy
	arg.Session.ID = out
	if err := client.Call("Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify
	_, s, err = state.SessionGet(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s != nil {
		t.Fatalf("bad: %v", s)
	}
}

func TestSessionEndpoint_Get(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
		},
	}
	var out string
	if err := client.Call("Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	getR := structs.SessionSpecificRequest{
		Datacenter: "dc1",
		Session:    out,
	}
	var sessions structs.IndexedSessions
	if err := client.Call("Session.Get", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index == 0 {
		t.Fatalf("Bad: %v", sessions)
	}
	if len(sessions.Sessions) != 1 {
		t.Fatalf("Bad: %v", sessions)
	}
	s := sessions.Sessions[0]
	if s.ID != out {
		t.Fatalf("bad: %v", s)
	}
}

func TestSessionEndpoint_List(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	ids := []string{}
	for i := 0; i < 5; i++ {
		arg := structs.SessionRequest{
			Datacenter: "dc1",
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: "foo",
			},
		}
		var out string
		if err := client.Call("Session.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		ids = append(ids, out)
	}

	getR := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var sessions structs.IndexedSessions
	if err := client.Call("Session.List", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index == 0 {
		t.Fatalf("Bad: %v", sessions)
	}
	if len(sessions.Sessions) != 5 {
		t.Fatalf("Bad: %v", sessions.Sessions)
	}
	for i := 0; i < len(sessions.Sessions); i++ {
		s := sessions.Sessions[i]
		if !strContains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Node != "foo" {
			t.Fatalf("bad: %v", s)
		}
	}
}

func TestSessionEndpoint_NodeSessions(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	s1.fsm.State().EnsureNode(1, structs.Node{"bar", "127.0.0.1"})
	ids := []string{}
	for i := 0; i < 10; i++ {
		arg := structs.SessionRequest{
			Datacenter: "dc1",
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: "bar",
			},
		}
		if i < 5 {
			arg.Session.Node = "foo"
		}
		var out string
		if err := client.Call("Session.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		if i < 5 {
			ids = append(ids, out)
		}
	}

	getR := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	var sessions structs.IndexedSessions
	if err := client.Call("Session.NodeSessions", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index == 0 {
		t.Fatalf("Bad: %v", sessions)
	}
	if len(sessions.Sessions) != 5 {
		t.Fatalf("Bad: %v", sessions.Sessions)
	}
	for i := 0; i < len(sessions.Sessions); i++ {
		s := sessions.Sessions[i]
		if !strContains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Node != "foo" {
			t.Fatalf("bad: %v", s)
		}
	}
}
