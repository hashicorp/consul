package consul

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/raft"
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
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
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
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
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
			Tags:    []string{"master"},
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
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
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
			Tags:    []string{"master"},
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
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
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
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
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
			Tags:    []string{"master"},
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
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	// Add some state
	fsm.state.EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	fsm.state.EnsureNode(2, structs.Node{"baz", "127.0.0.2"})
	fsm.state.EnsureService(3, "foo", &structs.NodeService{"web", "web", nil, "127.0.0.1", 80})
	fsm.state.EnsureService(4, "foo", &structs.NodeService{"db", "db", []string{"primary"}, "127.0.0.1", 5000})
	fsm.state.EnsureService(5, "baz", &structs.NodeService{"web", "web", nil, "127.0.0.2", 80})
	fsm.state.EnsureService(6, "baz", &structs.NodeService{"db", "db", []string{"secondary"}, "127.0.0.2", 5000})
	fsm.state.EnsureCheck(7, &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "web",
		Name:      "web connectivity",
		Status:    structs.HealthPassing,
		ServiceID: "web",
	})
	fsm.state.KVSSet(8, &structs.DirEntry{
		Key:   "/test",
		Value: []byte("foo"),
	})
	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	fsm.state.SessionCreate(9, session)
	acl := &structs.ACL{ID: generateUUID(), Name: "User Token"}
	fsm.state.ACLSet(10, acl)

	fsm.state.KVSSet(11, &structs.DirEntry{
		Key:   "/remove",
		Value: []byte("foo"),
	})
	fsm.state.KVSDelete(12, "/remove")

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
	fsm2, err := NewFSM(nil, path, os.Stderr)
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
	if !strContains(fooSrv.Services["db"].Tags, "primary") {
		t.Fatalf("Bad: %v", fooSrv)
	}
	if fooSrv.Services["db"].Port != 5000 {
		t.Fatalf("Bad: %v", fooSrv)
	}

	_, checks := fsm2.state.NodeChecks("foo")
	if len(checks) != 1 {
		t.Fatalf("Bad: %v", checks)
	}

	// Verify key is set
	_, d, err := fsm.state.KVSGet("/test")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(d.Value) != "foo" {
		t.Fatalf("bad: %v", d)
	}

	// Verify the index is restored
	idx, _, err := fsm.state.KVSListKeys("/blah", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx <= 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Verify session is restored
	idx, s, err := fsm.state.SessionGet(session.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Node != "foo" {
		t.Fatalf("bad: %v", s)
	}
	if idx <= 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Verify ACL is restored
	idx, a, err := fsm.state.ACLGet(acl.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if a.Name != "User Token" {
		t.Fatalf("bad: %v", a)
	}
	if idx <= 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Verify tombstones are restored
	_, res, err := fsm.state.tombstoneTable.Get("id", "/remove")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("bad: %v", res)
	}
}

func TestFSM_KVSSet(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         structs.KVSSet,
		DirEnt: structs.DirEntry{
			Key:   "/test/path",
			Flags: 0,
			Value: []byte("test"),
		},
	}
	buf, err := structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is set
	_, d, err := fsm.state.KVSGet("/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("missing")
	}
}

func TestFSM_KVSDelete(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         structs.KVSSet,
		DirEnt: structs.DirEntry{
			Key:   "/test/path",
			Flags: 0,
			Value: []byte("test"),
		},
	}
	buf, err := structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Run the delete
	req.Op = structs.KVSDelete
	buf, err = structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is not set
	_, d, err := fsm.state.KVSGet("/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("key present")
	}
}

func TestFSM_KVSDeleteTree(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         structs.KVSSet,
		DirEnt: structs.DirEntry{
			Key:   "/test/path",
			Flags: 0,
			Value: []byte("test"),
		},
	}
	buf, err := structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Run the delete tree
	req.Op = structs.KVSDeleteTree
	req.DirEnt.Key = "/test"
	buf, err = structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is not set
	_, d, err := fsm.state.KVSGet("/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("key present")
	}
}

func TestFSM_KVSDeleteCheckAndSet(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         structs.KVSSet,
		DirEnt: structs.DirEntry{
			Key:   "/test/path",
			Flags: 0,
			Value: []byte("test"),
		},
	}
	buf, err := structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is set
	_, d, err := fsm.state.KVSGet("/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("key missing")
	}

	// Run the check-and-set
	req.Op = structs.KVSDeleteCAS
	req.DirEnt.ModifyIndex = d.ModifyIndex
	buf, err = structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if resp.(bool) != true {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is gone
	_, d, err = fsm.state.KVSGet("/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("bad: %v", d)
	}
}

func TestFSM_KVSCheckAndSet(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         structs.KVSSet,
		DirEnt: structs.DirEntry{
			Key:   "/test/path",
			Flags: 0,
			Value: []byte("test"),
		},
	}
	buf, err := structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is set
	_, d, err := fsm.state.KVSGet("/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("key missing")
	}

	// Run the check-and-set
	req.Op = structs.KVSCAS
	req.DirEnt.ModifyIndex = d.ModifyIndex
	req.DirEnt.Value = []byte("zip")
	buf, err = structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if resp.(bool) != true {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is updated
	_, d, err = fsm.state.KVSGet("/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(d.Value) != "zip" {
		t.Fatalf("bad: %v", d)
	}
}

func TestFSM_SessionCreate_Destroy(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	fsm.state.EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	fsm.state.EnsureCheck(2, &structs.HealthCheck{
		Node:    "foo",
		CheckID: "web",
		Status:  structs.HealthPassing,
	})

	// Create a new session
	req := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			ID:     generateUUID(),
			Node:   "foo",
			Checks: []string{"web"},
		},
	}
	buf, err := structs.Encode(structs.SessionRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if err, ok := resp.(error); ok {
		t.Fatalf("resp: %v", err)
	}

	// Get the session
	id := resp.(string)
	_, session, err := fsm.state.SessionGet(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if session == nil {
		t.Fatalf("missing")
	}

	// Verify the session
	if session.ID != id {
		t.Fatalf("bad: %v", *session)
	}
	if session.Node != "foo" {
		t.Fatalf("bad: %v", *session)
	}
	if session.Checks[0] != "web" {
		t.Fatalf("bad: %v", *session)
	}

	// Try to destroy
	destroy := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionDestroy,
		Session: structs.Session{
			ID: id,
		},
	}
	buf, err = structs.Encode(structs.SessionRequestType, destroy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	_, session, err = fsm.state.SessionGet(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if session != nil {
		t.Fatalf("should be destroyed")
	}
}

func TestFSM_KVSLock(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	fsm.state.EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	fsm.state.SessionCreate(2, session)

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         structs.KVSLock,
		DirEnt: structs.DirEntry{
			Key:     "/test/path",
			Value:   []byte("test"),
			Session: session.ID,
		},
	}
	buf, err := structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != true {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is locked
	_, d, err := fsm.state.KVSGet("/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("missing")
	}
	if d.LockIndex != 1 {
		t.Fatalf("bad: %v", *d)
	}
	if d.Session != session.ID {
		t.Fatalf("bad: %v", *d)
	}
}

func TestFSM_KVSUnlock(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	fsm.state.EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	fsm.state.SessionCreate(2, session)

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         structs.KVSLock,
		DirEnt: structs.DirEntry{
			Key:     "/test/path",
			Value:   []byte("test"),
			Session: session.ID,
		},
	}
	buf, err := structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != true {
		t.Fatalf("resp: %v", resp)
	}

	req = structs.KVSRequest{
		Datacenter: "dc1",
		Op:         structs.KVSUnlock,
		DirEnt: structs.DirEntry{
			Key:     "/test/path",
			Value:   []byte("test"),
			Session: session.ID,
		},
	}
	buf, err = structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if resp != true {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is unlocked
	_, d, err := fsm.state.KVSGet("/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("missing")
	}
	if d.LockIndex != 1 {
		t.Fatalf("bad: %v", *d)
	}
	if d.Session != "" {
		t.Fatalf("bad: %v", *d)
	}
}

func TestFSM_ACL_Set_Delete(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	// Create a new ACL
	req := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			ID:   generateUUID(),
			Name: "User token",
			Type: structs.ACLTypeClient,
		},
	}
	buf, err := structs.Encode(structs.ACLRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if err, ok := resp.(error); ok {
		t.Fatalf("resp: %v", err)
	}

	// Get the ACL
	id := resp.(string)
	_, acl, err := fsm.state.ACLGet(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("missing")
	}

	// Verify the ACL
	if acl.ID != id {
		t.Fatalf("bad: %v", *acl)
	}
	if acl.Name != "User token" {
		t.Fatalf("bad: %v", *acl)
	}
	if acl.Type != structs.ACLTypeClient {
		t.Fatalf("bad: %v", *acl)
	}

	// Try to destroy
	destroy := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLDelete,
		ACL: structs.ACL{
			ID: id,
		},
	}
	buf, err = structs.Encode(structs.ACLRequestType, destroy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	_, acl, err = fsm.state.ACLGet(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl != nil {
		t.Fatalf("should be destroyed")
	}
}

func TestFSM_TombstoneReap(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	// Create some  tombstones
	fsm.state.KVSSet(11, &structs.DirEntry{
		Key:   "/remove",
		Value: []byte("foo"),
	})
	fsm.state.KVSDelete(12, "/remove")

	// Create a new reap request
	req := structs.TombstoneRequest{
		Datacenter: "dc1",
		Op:         structs.TombstoneReap,
		ReapIndex:  12,
	}
	buf, err := structs.Encode(structs.TombstoneRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if err, ok := resp.(error); ok {
		t.Fatalf("resp: %v", err)
	}

	// Verify the tombstones are gone
	_, res, err := fsm.state.tombstoneTable.Get("id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("bad: %v", res)
	}
}

func TestFSM_IgnoreUnknown(t *testing.T) {
	path, err := ioutil.TempDir("", "fsm")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(path)
	fsm, err := NewFSM(nil, path, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer fsm.Close()

	// Create a new reap request
	type UnknownRequest struct {
		Foo string
	}
	req := UnknownRequest{Foo: "bar"}
	msgType := structs.IgnoreUnknownTypeFlag | 64
	buf, err := structs.Encode(msgType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Apply should work, even though not supported
	resp := fsm.Apply(makeLog(buf))
	if err, ok := resp.(error); ok {
		t.Fatalf("resp: %v", err)
	}
}
