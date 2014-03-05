package consul

import (
	"bytes"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/raft"
	"os"
	"testing"
)

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

func makeLog(buf []byte) *raft.Log {
	return &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  buf,
	}
}

func TestFSM_RegisterNode(t *testing.T) {
	fsm, err := NewFSM(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if idx, found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	} else if idx != 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Verify service registered
	_, services := fsm.state.NodeServices("foo")
	if len(services.Services) != 0 {
		t.Fatalf("Services: %v", services)
	}
}

func TestFSM_RegisterNode_Service(t *testing.T) {
	fsm, err := NewFSM(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tag:     "master",
			Port:    8000,
		},
		Check: &structs.HealthCheck{
			Node:      "foo",
			CheckID:   "db",
			Name:      "db connectivity",
			Status:    structs.HealthPassing,
			ServiceID: "db",
		},
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if _, found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	}

	// Verify service registered
	_, services := fsm.state.NodeServices("foo")
	if _, ok := services.Services["db"]; !ok {
		t.Fatalf("not registered!")
	}

	// Verify check
	_, checks := fsm.state.NodeChecks("foo")
	if checks[0].CheckID != "db" {
		t.Fatalf("not registered!")
	}
}

func TestFSM_DeregisterService(t *testing.T) {
	fsm, err := NewFSM(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tag:     "master",
			Port:    8000,
		},
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	dereg := structs.DeregisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		ServiceID:  "db",
	}
	buf, err = structs.Encode(structs.DeregisterRequestType, dereg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if _, found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	}

	// Verify service not registered
	_, services := fsm.state.NodeServices("foo")
	if _, ok := services.Services["db"]; ok {
		t.Fatalf("db registered!")
	}
}

func TestFSM_DeregisterCheck(t *testing.T) {
	fsm, err := NewFSM(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Node:    "foo",
			CheckID: "mem",
			Name:    "memory util",
			Status:  structs.HealthPassing,
		},
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	dereg := structs.DeregisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		CheckID:    "mem",
	}
	buf, err = structs.Encode(structs.DeregisterRequestType, dereg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if _, found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	}

	// Verify check not registered
	_, checks := fsm.state.NodeChecks("foo")
	if len(checks) != 0 {
		t.Fatalf("check registered!")
	}
}

func TestFSM_DeregisterNode(t *testing.T) {
	fsm, err := NewFSM(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tag:     "master",
			Port:    8000,
		},
		Check: &structs.HealthCheck{
			Node:      "foo",
			CheckID:   "db",
			Name:      "db connectivity",
			Status:    structs.HealthPassing,
			ServiceID: "db",
		},
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	dereg := structs.DeregisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	buf, err = structs.Encode(structs.DeregisterRequestType, dereg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if _, found, _ := fsm.state.GetNode("foo"); found {
		t.Fatalf("found!")
	}

	// Verify service not registered
	_, services := fsm.state.NodeServices("foo")
	if services != nil {
		t.Fatalf("Services: %v", services)
	}

	// Verify checks not registered
	_, checks := fsm.state.NodeChecks("foo")
	if len(checks) != 0 {
		t.Fatalf("Services: %v", services)
	}
}

func TestFSM_SnapshotRestore(t *testing.T) {
	fsm, err := NewFSM(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	// Add some state
	fsm.state.EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	fsm.state.EnsureNode(2, structs.Node{"baz", "127.0.0.2"})
	fsm.state.EnsureService(3, "foo", &structs.NodeService{"web", "web", "", 80})
	fsm.state.EnsureService(4, "foo", &structs.NodeService{"db", "db", "primary", 5000})
	fsm.state.EnsureService(5, "baz", &structs.NodeService{"web", "web", "", 80})
	fsm.state.EnsureService(6, "baz", &structs.NodeService{"db", "db", "secondary", 5000})
	fsm.state.EnsureCheck(7, &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "web",
		Name:      "web connectivity",
		Status:    structs.HealthPassing,
		ServiceID: "web",
	})

	// Snapshot
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Release()

	// Persist
	buf := bytes.NewBuffer(nil)
	sink := &MockSink{buf, false}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try to restore on a new FSM
	fsm2, err := NewFSM(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm2.Close()

	// Do a restore
	if err := fsm2.Restore(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the contents
	_, nodes := fsm2.state.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("Bad: %v", nodes)
	}

	_, fooSrv := fsm2.state.NodeServices("foo")
	if len(fooSrv.Services) != 2 {
		t.Fatalf("Bad: %v", fooSrv)
	}
	if fooSrv.Services["db"].Tag != "primary" {
		t.Fatalf("Bad: %v", fooSrv)
	}
	if fooSrv.Services["db"].Port != 5000 {
		t.Fatalf("Bad: %v", fooSrv)
	}

	_, checks := fsm2.state.NodeChecks("foo")
	if len(checks) != 1 {
		t.Fatalf("Bad: %v", checks)
	}
}
