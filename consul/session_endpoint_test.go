package consul

import (
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"os"
	"testing"
	"time"
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
			Name: "my-session",
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
	if s.Node != "foo" {
		t.Fatalf("bad: %v", s)
	}
	if s.Name != "my-session" {
		t.Fatalf("bad: %v", s)
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

func TestSessionEndpoint_DeleteApply(t *testing.T) {
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
			Node:     "foo",
			Name:     "my-session",
			Behavior: structs.SessionKeysDelete,
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
	if s.Node != "foo" {
		t.Fatalf("bad: %v", s)
	}
	if s.Name != "my-session" {
		t.Fatalf("bad: %v", s)
	}
	if s.Behavior != structs.SessionKeysDelete {
		t.Fatalf("bad: %v", s)
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

func TestSessionEndpoint_Renew(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")
	TTL := "10s"

	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	ids := []string{}
	for i := 0; i < 5; i++ {
		arg := structs.SessionRequest{
			Datacenter: "dc1",
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: "foo",
				TTL:  TTL,
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
		if s.TTL != TTL {
			t.Fatalf("bad session TTL: %s %v", s.TTL, s)
		}
		t.Logf("Created session '%s'", s.ID)
	}

	// now sleep for ttl - since internally we use ttl*2 to destroy, this is ok
	time.Sleep(10 * time.Second)

	// renew 3 out of 5 sessions
	for i := 0; i < 3; i++ {
		renewR := structs.SessionSpecificRequest{
			Datacenter: "dc1",
			Session:    ids[i],
		}
		var session structs.IndexedSessions
		if err := client.Call("Session.Renew", &renewR, &session); err != nil {
			t.Fatalf("err: %v", err)
		}

		if session.Index == 0 {
			t.Fatalf("Bad: %v", session)
		}
		if len(session.Sessions) != 1 {
			t.Fatalf("Bad: %v", session.Sessions)
		}

		s := session.Sessions[0]
		if !strContains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Node != "foo" {
			t.Fatalf("bad: %v", s)
		}

		t.Logf("Renewed session '%s'", s.ID)
	}

	// now sleep for ttl*2 - 3 sessions should still be alive
	time.Sleep(2 * 10 * time.Second)

	if err := client.Call("Session.List", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index == 0 {
		t.Fatalf("Bad: %v", sessions)
	}

	t.Logf("Expect 2 sessions to be destroyed")

	for i := 0; i < len(sessions.Sessions); i++ {
		s := sessions.Sessions[i]
		if !strContains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Node != "foo" {
			t.Fatalf("bad: %v", s)
		}
		if s.TTL != TTL {
			t.Fatalf("bad: %v", s)
		}
		if i > 2 {
			t.Errorf("session '%s' should be destroyed", s.ID)
		}
	}

	if len(sessions.Sessions) > 3 {
		t.Fatalf("Bad: %v", sessions.Sessions)
	}

	// now sleep again for ttl*2 - no sessions should still be alive
	time.Sleep(20 * time.Second)

	if err := client.Call("Session.List", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index != 0 {
		t.Fatalf("Bad: %v", sessions)
	}
	if len(sessions.Sessions) != 0 {
		for i := 0; i < len(sessions.Sessions); i++ {
			s := sessions.Sessions[i]
			if !strContains(ids, s.ID) {
				t.Fatalf("bad: %v", s)
			}
			if s.Node != "foo" {
				t.Fatalf("bad: %v", s)
			}
			if s.TTL != TTL {
				t.Fatalf("bad: %v", s)
			}
			t.Errorf("session '%s' should be destroyed", s.ID)
		}
		
		t.Fatalf("Bad: %v", sessions.Sessions)
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
