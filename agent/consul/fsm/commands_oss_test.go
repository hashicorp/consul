package fsm

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/serf/coordinate"
	"github.com/mitchellh/mapstructure"
	"github.com/pascaldekloe/goe/verify"
	"github.com/stretchr/testify/assert"
)

func generateUUID() (ret string) {
	var err error
	if ret, err = uuid.GenerateUUID(); err != nil {
		panic(fmt.Sprintf("Unable to generate a UUID, %v", err))
	}
	return ret
}

func generateRandomCoordinate() *coordinate.Coordinate {
	config := coordinate.DefaultConfig()
	coord := coordinate.NewCoordinate(config)
	for i := range coord.Vec {
		coord.Vec[i] = rand.NormFloat64()
	}
	coord.Error = rand.NormFloat64()
	coord.Adjustment = rand.NormFloat64()
	return coord
}

func TestFSM_RegisterNode(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
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

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	_, node, err := fsm.state.GetNode("foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if node == nil {
		t.Fatalf("not found!")
	}
	if node.ModifyIndex != 1 {
		t.Fatalf("bad index: %d", node.ModifyIndex)
	}

	// Verify service registered
	_, services, err := fsm.state.NodeServices(nil, "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(services.Services) != 0 {
		t.Fatalf("Services: %v", services)
	}
}

func TestFSM_RegisterNode_Service(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
			Status:    api.HealthPassing,
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
	_, node, err := fsm.state.GetNode("foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if node == nil {
		t.Fatalf("not found!")
	}

	// Verify service registered
	_, services, err := fsm.state.NodeServices(nil, "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, ok := services.Services["db"]; !ok {
		t.Fatalf("not registered!")
	}

	// Verify check
	_, checks, err := fsm.state.NodeChecks(nil, "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if checks[0].CheckID != "db" {
		t.Fatalf("not registered!")
	}
}

func TestFSM_DeregisterService(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
	_, node, err := fsm.state.GetNode("foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if node == nil {
		t.Fatalf("not found!")
	}

	// Verify service not registered
	_, services, err := fsm.state.NodeServices(nil, "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, ok := services.Services["db"]; ok {
		t.Fatalf("db registered!")
	}
}

func TestFSM_DeregisterCheck(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Node:    "foo",
			CheckID: "mem",
			Name:    "memory util",
			Status:  api.HealthPassing,
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
	_, node, err := fsm.state.GetNode("foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if node == nil {
		t.Fatalf("not found!")
	}

	// Verify check not registered
	_, checks, err := fsm.state.NodeChecks(nil, "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 0 {
		t.Fatalf("check registered!")
	}
}

func TestFSM_DeregisterNode(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
			Status:    api.HealthPassing,
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

	// Verify we are not registered
	_, node, err := fsm.state.GetNode("foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if node != nil {
		t.Fatalf("found!")
	}

	// Verify service not registered
	_, services, err := fsm.state.NodeServices(nil, "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if services != nil {
		t.Fatalf("Services: %v", services)
	}

	// Verify checks not registered
	_, checks, err := fsm.state.NodeChecks(nil, "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 0 {
		t.Fatalf("Services: %v", services)
	}
}

func TestFSM_KVSDelete(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         api.KVSet,
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
	req.Op = api.KVDelete
	buf, err = structs.Encode(structs.KVSRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify key is not set
	_, d, err := fsm.state.KVSGet(nil, "/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("key present")
	}
}

func TestFSM_KVSDeleteTree(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         api.KVSet,
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
	req.Op = api.KVDeleteTree
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("key present")
	}
}

func TestFSM_KVSDeleteCheckAndSet(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         api.KVSet,
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("key missing")
	}

	// Run the check-and-set
	req.Op = api.KVDeleteCAS
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
	_, d, err = fsm.state.KVSGet(nil, "/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("bad: %v", d)
	}
}

func TestFSM_KVSCheckAndSet(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         api.KVSet,
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("key missing")
	}

	// Run the check-and-set
	req.Op = api.KVCAS
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
	_, d, err = fsm.state.KVSGet(nil, "/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(d.Value) != "zip" {
		t.Fatalf("bad: %v", d)
	}
}

func TestFSM_KVSLock(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	fsm.state.SessionCreate(2, session)

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         api.KVLock,
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path")
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
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	fsm.state.SessionCreate(2, session)

	req := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         api.KVLock,
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
		Op:         api.KVUnlock,
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path")
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

func TestFSM_CoordinateUpdate(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register some nodes.
	fsm.state.EnsureNode(1, &structs.Node{Node: "node1", Address: "127.0.0.1"})
	fsm.state.EnsureNode(2, &structs.Node{Node: "node2", Address: "127.0.0.1"})

	// Write a batch of two coordinates.
	updates := structs.Coordinates{
		&structs.Coordinate{
			Node:  "node1",
			Coord: generateRandomCoordinate(),
		},
		&structs.Coordinate{
			Node:  "node2",
			Coord: generateRandomCoordinate(),
		},
	}
	buf, err := structs.Encode(structs.CoordinateBatchUpdateType, updates)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Read back the two coordinates to make sure they got updated.
	_, coords, err := fsm.state.Coordinates(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !reflect.DeepEqual(coords, updates) {
		t.Fatalf("bad: %#v", coords)
	}
}

func TestFSM_SessionCreate_Destroy(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	fsm.state.EnsureCheck(2, &structs.HealthCheck{
		Node:    "foo",
		CheckID: "web",
		Status:  api.HealthPassing,
	})

	// Create a new session
	req := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			ID:     generateUUID(),
			Node:   "foo",
			Checks: []types.CheckID{"web"},
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
	_, session, err := fsm.state.SessionGet(nil, id)
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

	_, session, err = fsm.state.SessionGet(nil, id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if session != nil {
		t.Fatalf("should be destroyed")
	}
}

func TestFSM_ACL_CRUD(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new ACL.
	req := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			ID:   generateUUID(),
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
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

	// Get the ACL.
	id := resp.(string)
	_, acl, err := fsm.state.ACLTokenGetBySecret(nil, id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("missing")
	}

	// Verify the ACL.
	if acl.SecretID != id {
		t.Fatalf("bad: %v", *acl)
	}
	if acl.Description != "User token" {
		t.Fatalf("bad: %v", *acl)
	}
	if acl.Type != structs.ACLTokenTypeClient {
		t.Fatalf("bad: %v", *acl)
	}

	// Try to destroy.
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

	_, acl, err = fsm.state.ACLTokenGetBySecret(nil, id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl != nil {
		t.Fatalf("should be destroyed")
	}

	// Initialize bootstrap (should work since we haven't made a management
	// token).
	init := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLBootstrapInit,
	}
	buf, err = structs.Encode(structs.ACLRequestType, init)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if enabled, ok := resp.(bool); !ok || !enabled {
		t.Fatalf("resp: %v", resp)
	}
	canBootstrap, _, err := fsm.state.CanBootstrapACLToken()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !canBootstrap {
		t.Fatalf("bad: shouldn't be able to bootstrap")
	}

	// Do a bootstrap.
	bootstrap := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLBootstrapNow,
		ACL: structs.ACL{
			ID:   generateUUID(),
			Name: "Bootstrap Token",
			Type: structs.ACLTokenTypeManagement,
		},
	}
	buf, err = structs.Encode(structs.ACLRequestType, bootstrap)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	respACL, ok := resp.(*structs.ACL)
	if !ok {
		t.Fatalf("resp: %v", resp)
	}
	bootstrap.ACL.CreateIndex = respACL.CreateIndex
	bootstrap.ACL.ModifyIndex = respACL.ModifyIndex
	verify.Values(t, "", respACL, &bootstrap.ACL)
}

func TestFSM_PreparedQuery_CRUD(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a service to query on.
	fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	fsm.state.EnsureService(2, "foo", &structs.NodeService{ID: "web", Service: "web", Tags: nil, Address: "127.0.0.1", Port: 80})

	// Create a new query.
	query := structs.PreparedQueryRequest{
		Op: structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			ID: generateUUID(),
			Service: structs.ServiceQuery{
				Service: "web",
			},
		},
	}
	{
		buf, err := structs.Encode(structs.PreparedQueryRequestType, query)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp := fsm.Apply(makeLog(buf))
		if resp != nil {
			t.Fatalf("resp: %v", resp)
		}
	}

	// Verify it's in the state store.
	{
		_, actual, err := fsm.state.PreparedQueryGet(nil, query.Query.ID)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Make an update to the query.
	query.Op = structs.PreparedQueryUpdate
	query.Query.Name = "my-query"
	{
		buf, err := structs.Encode(structs.PreparedQueryRequestType, query)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp := fsm.Apply(makeLog(buf))
		if resp != nil {
			t.Fatalf("resp: %v", resp)
		}
	}

	// Verify the update.
	{
		_, actual, err := fsm.state.PreparedQueryGet(nil, query.Query.ID)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Delete the query.
	query.Op = structs.PreparedQueryDelete
	{
		buf, err := structs.Encode(structs.PreparedQueryRequestType, query)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp := fsm.Apply(makeLog(buf))
		if resp != nil {
			t.Fatalf("resp: %v", resp)
		}
	}

	// Make sure it's gone.
	{
		_, actual, err := fsm.state.PreparedQueryGet(nil, query.Query.ID)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if actual != nil {
			t.Fatalf("bad: %v", actual)
		}
	}
}

func TestFSM_TombstoneReap(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create some tombstones
	fsm.state.KVSSet(11, &structs.DirEntry{
		Key:   "/remove",
		Value: []byte("foo"),
	})
	fsm.state.KVSDelete(12, "/remove")
	idx, _, err := fsm.state.KVSList(nil, "/remove")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 12 {
		t.Fatalf("bad index: %d", idx)
	}

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
	snap := fsm.state.Snapshot()
	defer snap.Close()
	stones, err := snap.Tombstones()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if stones.Next() != nil {
		t.Fatalf("unexpected extra tombstones")
	}
}

func TestFSM_Txn(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Set a key using a transaction.
	req := structs.TxnRequest{
		Datacenter: "dc1",
		Ops: structs.TxnOps{
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVSet,
					DirEnt: structs.DirEntry{
						Key:   "/test/path",
						Flags: 0,
						Value: []byte("test"),
					},
				},
			},
		},
	}
	buf, err := structs.Encode(structs.TxnRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if _, ok := resp.(structs.TxnResponse); !ok {
		t.Fatalf("bad response type: %T", resp)
	}

	// Verify key is set directly in the state store.
	_, d, err := fsm.state.KVSGet(nil, "/test/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("missing")
	}
}

func TestFSM_Autopilot(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Set the autopilot config using a request.
	req := structs.AutopilotSetConfigRequest{
		Datacenter: "dc1",
		Config: autopilot.Config{
			CleanupDeadServers:   true,
			LastContactThreshold: 10 * time.Second,
			MaxTrailingLogs:      300,
		},
	}
	buf, err := structs.Encode(structs.AutopilotRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if _, ok := resp.(error); ok {
		t.Fatalf("bad: %v", resp)
	}

	// Verify key is set directly in the state store.
	_, config, err := fsm.state.AutopilotConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if config.CleanupDeadServers != req.Config.CleanupDeadServers {
		t.Fatalf("bad: %v", config.CleanupDeadServers)
	}
	if config.LastContactThreshold != req.Config.LastContactThreshold {
		t.Fatalf("bad: %v", config.LastContactThreshold)
	}
	if config.MaxTrailingLogs != req.Config.MaxTrailingLogs {
		t.Fatalf("bad: %v", config.MaxTrailingLogs)
	}

	// Now use CAS and provide an old index
	req.CAS = true
	req.Config.CleanupDeadServers = false
	req.Config.ModifyIndex = config.ModifyIndex - 1
	buf, err = structs.Encode(structs.AutopilotRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if _, ok := resp.(error); ok {
		t.Fatalf("bad: %v", resp)
	}

	_, config, err = fsm.state.AutopilotConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !config.CleanupDeadServers {
		t.Fatalf("bad: %v", config.CleanupDeadServers)
	}
}

func TestFSM_Intention_CRUD(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	fsm, err := New(nil, os.Stderr)
	assert.Nil(err)

	// Create a new intention.
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.ID = generateUUID()
	ixn.Intention.UpdatePrecedence()

	{
		buf, err := structs.Encode(structs.IntentionRequestType, ixn)
		assert.Nil(err)
		assert.Nil(fsm.Apply(makeLog(buf)))
	}

	// Verify it's in the state store.
	{
		_, actual, err := fsm.state.IntentionGet(nil, ixn.Intention.ID)
		assert.Nil(err)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		assert.Equal(ixn.Intention, actual)
	}

	// Make an update
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.SourceName = "api"
	{
		buf, err := structs.Encode(structs.IntentionRequestType, ixn)
		assert.Nil(err)
		assert.Nil(fsm.Apply(makeLog(buf)))
	}

	// Verify the update.
	{
		_, actual, err := fsm.state.IntentionGet(nil, ixn.Intention.ID)
		assert.Nil(err)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		assert.Equal(ixn.Intention, actual)
	}

	// Delete
	ixn.Op = structs.IntentionOpDelete
	{
		buf, err := structs.Encode(structs.IntentionRequestType, ixn)
		assert.Nil(err)
		assert.Nil(fsm.Apply(makeLog(buf)))
	}

	// Make sure it's gone.
	{
		_, actual, err := fsm.state.IntentionGet(nil, ixn.Intention.ID)
		assert.Nil(err)
		assert.Nil(actual)
	}
}

func TestFSM_CAConfig(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	fsm, err := New(nil, os.Stderr)
	assert.Nil(err)

	// Set the autopilot config using a request.
	req := structs.CARequest{
		Op: structs.CAOpSetConfig,
		Config: &structs.CAConfiguration{
			Provider: "consul",
			Config: map[string]interface{}{
				"PrivateKey":     "asdf",
				"RootCert":       "qwer",
				"RotationPeriod": 90 * 24 * time.Hour,
			},
		},
	}
	buf, err := structs.Encode(structs.ConnectCARequestType, req)
	assert.Nil(err)
	resp := fsm.Apply(makeLog(buf))
	if _, ok := resp.(error); ok {
		t.Fatalf("bad: %v", resp)
	}

	// Verify key is set directly in the state store.
	_, config, err := fsm.state.CAConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var conf *structs.ConsulCAProviderConfig
	if err := mapstructure.WeakDecode(config.Config, &conf); err != nil {
		t.Fatalf("error decoding config: %s, %v", err, config.Config)
	}
	if got, want := config.Provider, req.Config.Provider; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := conf.PrivateKey, "asdf"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := conf.RootCert, "qwer"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := conf.RotationPeriod, 90*24*time.Hour; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Now use CAS and provide an old index
	req.Config.Provider = "static"
	req.Config.ModifyIndex = config.ModifyIndex - 1
	buf, err = structs.Encode(structs.ConnectCARequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if _, ok := resp.(error); ok {
		t.Fatalf("bad: %v", resp)
	}

	_, config, err = fsm.state.CAConfig()
	assert.Nil(err)
	if config.Provider != "static" {
		t.Fatalf("bad: %v", config.Provider)
	}
}

func TestFSM_CARoots(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	fsm, err := New(nil, os.Stderr)
	assert.Nil(err)

	// Roots
	ca1 := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, nil)
	ca2.Active = false

	// Create a new request.
	req := structs.CARequest{
		Op:    structs.CAOpSetRoots,
		Roots: []*structs.CARoot{ca1, ca2},
	}

	{
		buf, err := structs.Encode(structs.ConnectCARequestType, req)
		assert.Nil(err)
		assert.True(fsm.Apply(makeLog(buf)).(bool))
	}

	// Verify it's in the state store.
	{
		_, roots, err := fsm.state.CARoots(nil)
		assert.Nil(err)
		assert.Len(roots, 2)
	}
}

func TestFSM_CABuiltinProvider(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	fsm, err := New(nil, os.Stderr)
	assert.Nil(err)

	// Provider state.
	expected := &structs.CAConsulProviderState{
		ID:         "foo",
		PrivateKey: "a",
		RootCert:   "b",
		RaftIndex: structs.RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}

	// Create a new request.
	req := structs.CARequest{
		Op:            structs.CAOpSetProviderState,
		ProviderState: expected,
	}

	{
		buf, err := structs.Encode(structs.ConnectCARequestType, req)
		assert.Nil(err)
		assert.True(fsm.Apply(makeLog(buf)).(bool))
	}

	// Verify it's in the state store.
	{
		_, state, err := fsm.state.CAProviderState("foo")
		assert.Nil(err)
		assert.Equal(expected, state)
	}
}
