package consul

import (
	"bytes"
	"github.com/hashicorp/consul/consul/structs"
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

func TestFSM_RegisterNode(t *testing.T) {
	fsm, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	}

	// Verify service registered
	services := fsm.state.NodeServices("foo")
	if len(services) != 0 {
		t.Fatalf("Services: %v", services)
	}
}

func TestFSM_RegisterNode_Service(t *testing.T) {
	fsm, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 8000,
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	}

	// Verify service registered
	services := fsm.state.NodeServices("foo")
	if _, ok := services["db"]; !ok {
		t.Fatalf("not registered!")
	}
}

func TestFSM_DeregisterService(t *testing.T) {
	fsm, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 8000,
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	dereg := structs.DeregisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		ServiceName: "db",
	}
	buf, err = structs.Encode(structs.DeregisterRequestType, dereg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	}

	// Verify service not registered
	services := fsm.state.NodeServices("foo")
	if _, ok := services["db"]; ok {
		t.Fatalf("db registered!")
	}
}

func TestFSM_DeregisterNode(t *testing.T) {
	fsm, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 8000,
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(buf)
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

	resp = fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if found, _ := fsm.state.GetNode("foo"); found {
		t.Fatalf("found!")
	}

	// Verify service not registered
	services := fsm.state.NodeServices("foo")
	if len(services) != 0 {
		t.Fatalf("Services: %v", services)
	}
}

func TestFSM_SnapshotRestore(t *testing.T) {
	fsm, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Add some state
	fsm.state.EnsureNode("foo", "127.0.0.1")
	fsm.state.EnsureNode("baz", "127.0.0.2")
	fsm.state.EnsureService("foo", "web", "", 80)
	fsm.state.EnsureService("foo", "db", "primary", 5000)
	fsm.state.EnsureService("baz", "web", "", 80)
	fsm.state.EnsureService("baz", "db", "secondary", 5000)

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
	fsm2, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Do a restore
	if err := fsm2.Restore(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the contents
	nodes := fsm2.state.Nodes()
	if len(nodes) != 4 {
		t.Fatalf("Bad: %v", nodes)
	}

	fooSrv := fsm2.state.NodeServices("foo")
	if len(fooSrv) != 2 {
		t.Fatalf("Bad: %v", fooSrv)
	}
	if fooSrv["db"].Tag != "primary" {
		t.Fatalf("Bad: %v", fooSrv)
	}
	if fooSrv["db"].Port != 5000 {
		t.Fatalf("Bad: %v", fooSrv)
	}
}
