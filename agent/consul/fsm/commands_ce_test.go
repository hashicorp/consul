// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fsm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/internal/storage"
	raftstorage "github.com/hashicorp/consul/internal/storage/raft"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-raftchunking"
	raftchunkingtypes "github.com/hashicorp/go-raftchunking/types"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/coordinate"
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
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.RegisterRequest{
		Datacenter:     "dc1",
		Node:           "foo",
		Address:        "127.0.0.1",
		EnterpriseMeta: *structs.NodeEnterpriseMetaInDefaultPartition(),
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
	_, node, err := fsm.state.GetNode("foo", nil, "")
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
	_, services, err := fsm.state.NodeServices(nil, "foo", structs.DefaultEnterpriseMetaInDefaultPartition(), "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(services.Services) != 0 {
		t.Fatalf("Services: %v", services)
	}
}

func TestFSM_RegisterNode_Service(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
			Tags:    []string{"primary"},
			Port:    8000,
		},
		Check: &structs.HealthCheck{
			Node:      "foo",
			CheckID:   "db",
			Name:      "db connectivity",
			Status:    api.HealthPassing,
			ServiceID: "db",
		},
		EnterpriseMeta: *structs.NodeEnterpriseMetaInDefaultPartition(),
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
	_, node, err := fsm.state.GetNode("foo", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if node == nil {
		t.Fatalf("not found!")
	}

	// Verify service registered
	_, services, err := fsm.state.NodeServices(nil, "foo", structs.DefaultEnterpriseMetaInDefaultPartition(), "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, ok := services.Services["db"]; !ok {
		t.Fatalf("not registered!")
	}

	// Verify check
	_, checks, err := fsm.state.NodeChecks(nil, "foo", structs.DefaultEnterpriseMetaInDefaultPartition(), "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if checks[0].CheckID != "db" {
		t.Fatalf("not registered!")
	}
}

func TestFSM_DeregisterService(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
			Tags:    []string{"primary"},
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
	_, node, err := fsm.state.GetNode("foo", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if node == nil {
		t.Fatalf("not found!")
	}

	// Verify service not registered
	_, services, err := fsm.state.NodeServices(nil, "foo", structs.DefaultEnterpriseMetaInDefaultPartition(), "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, ok := services.Services["db"]; ok {
		t.Fatalf("db registered!")
	}
}

func TestFSM_DeregisterCheck(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
	_, node, err := fsm.state.GetNode("foo", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if node == nil {
		t.Fatalf("not found!")
	}

	// Verify check not registered
	_, checks, err := fsm.state.NodeChecks(nil, "foo", structs.DefaultEnterpriseMetaInDefaultPartition(), "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 0 {
		t.Fatalf("check registered!")
	}
}

func TestFSM_DeregisterNode(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
			Tags:    []string{"primary"},
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
	_, node, err := fsm.state.GetNode("foo", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if node != nil {
		t.Fatalf("found!")
	}

	// Verify service not registered
	_, services, err := fsm.state.NodeServices(nil, "foo", structs.DefaultEnterpriseMetaInDefaultPartition(), "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if services != nil {
		t.Fatalf("Services: %v", services)
	}

	// Verify checks not registered
	_, checks, err := fsm.state.NodeChecks(nil, "foo", structs.DefaultEnterpriseMetaInDefaultPartition(), "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 0 {
		t.Fatalf("Services: %v", services)
	}
}

func TestFSM_KVSDelete(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("key present")
	}
}

func TestFSM_KVSDeleteTree(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("key present")
	}
}

func TestFSM_KVSDeleteCheckAndSet(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path", nil)
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
	_, d, err = fsm.state.KVSGet(nil, "/test/path", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("bad: %v", d)
	}
}

func TestFSM_KVSCheckAndSet(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path", nil)
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
	_, d, err = fsm.state.KVSGet(nil, "/test/path", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(d.Value) != "zip" {
		t.Fatalf("bad: %v", d)
	}
}

func TestFSM_KVSLock(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	err = fsm.state.SessionCreate(2, session)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
	_, d, err := fsm.state.KVSGet(nil, "/test/path", nil)
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
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	err = fsm.state.SessionCreate(2, session)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
	_, d, err := fsm.state.KVSGet(nil, "/test/path", nil)
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
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register some nodes.
	err = fsm.state.EnsureNode(1, &structs.Node{Node: "node1", Address: "127.0.0.1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = fsm.state.EnsureNode(2, &structs.Node{Node: "node2", Address: "127.0.0.1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Write a batch of two coordinates.
	updates := structs.Coordinates{
		&structs.Coordinate{
			Node:      "node1",
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			Coord:     generateRandomCoordinate(),
		},
		&structs.Coordinate{
			Node:      "node2",
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			Coord:     generateRandomCoordinate(),
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
	_, coords, err := fsm.state.Coordinates(nil, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	require.Equal(t, updates, coords)
}

func TestFSM_SessionCreate_Destroy(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = fsm.state.EnsureCheck(2, &structs.HealthCheck{
		Node:    "foo",
		CheckID: "web",
		Status:  api.HealthPassing,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
	_, session, err := fsm.state.SessionGet(nil, id, nil)
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

	_, session, err = fsm.state.SessionGet(nil, id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if session != nil {
		t.Fatalf("should be destroyed")
	}
}

func TestFSM_PreparedQuery_CRUD(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a service to query on.
	err = fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = fsm.state.EnsureService(2, "foo", &structs.NodeService{ID: "web", Service: "web", Tags: nil, Address: "127.0.0.1", Port: 80})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create some tombstones
	err = fsm.state.KVSSet(11, &structs.DirEntry{
		Key:   "/remove",
		Value: []byte("foo"),
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = fsm.state.KVSDelete(12, "/remove", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, _, err := fsm.state.KVSList(nil, "/remove", nil)
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
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
	_, d, err := fsm.state.KVSGet(nil, "/test/path", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("missing")
	}
}

func TestFSM_Autopilot(t *testing.T) {
	t.Parallel()
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Set the autopilot config using a request.
	req := structs.AutopilotSetConfigRequest{
		Datacenter: "dc1",
		Config: structs.AutopilotConfig{
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

	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	assert.Nil(t, err)

	// Create a new intention.
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.ID = generateUUID()
	//nolint:staticcheck
	ixn.Intention.UpdatePrecedence()

	{
		buf, err := structs.Encode(structs.IntentionRequestType, ixn)
		assert.Nil(t, err)
		assert.Nil(t, fsm.Apply(makeLog(buf)))
	}

	// Verify it's in the state store.
	{
		_, _, actual, err := fsm.state.IntentionGet(nil, ixn.Intention.ID)
		assert.Nil(t, err)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		assert.Equal(t, ixn.Intention, actual)
	}

	// Make an update
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.SourceName = "api"
	{
		buf, err := structs.Encode(structs.IntentionRequestType, ixn)
		assert.Nil(t, err)
		assert.Nil(t, fsm.Apply(makeLog(buf)))
	}

	// Verify the update.
	{
		_, _, actual, err := fsm.state.IntentionGet(nil, ixn.Intention.ID)
		assert.Nil(t, err)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		assert.Equal(t, ixn.Intention, actual)
	}

	// Delete
	ixn.Op = structs.IntentionOpDelete
	{
		buf, err := structs.Encode(structs.IntentionRequestType, ixn)
		assert.Nil(t, err)
		assert.Nil(t, fsm.Apply(makeLog(buf)))
	}

	// Make sure it's gone.
	{
		_, _, actual, err := fsm.state.IntentionGet(nil, ixn.Intention.ID)
		assert.Nil(t, err)
		assert.Nil(t, actual)
	}
}

func TestFSM_CAConfig(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	assert.Nil(t, err)

	// Set the autopilot config using a request.
	req := structs.CARequest{
		Op: structs.CAOpSetConfig,
		Config: &structs.CAConfiguration{
			Provider: "consul",
			Config: map[string]interface{}{
				"PrivateKey":          "asdf",
				"RootCert":            "qwer",
				"IntermediateCertTTL": 365 * 24 * time.Hour,
			},
		},
	}
	buf, err := structs.Encode(structs.ConnectCARequestType, req)
	assert.Nil(t, err)
	resp := fsm.Apply(makeLog(buf))
	if _, ok := resp.(error); ok {
		t.Fatalf("bad: %v", resp)
	}

	// Verify key is set directly in the state store.
	_, config, err := fsm.state.CAConfig(nil)
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
	if got, want := conf.IntermediateCertTTL, 365*24*time.Hour; got != want {
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

	_, config, err = fsm.state.CAConfig(nil)
	assert.Nil(t, err)
	if config.Provider != "static" {
		t.Fatalf("bad: %v", config.Provider)
	}
}

func TestFSM_CARoots(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	assert.Nil(t, err)

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
		assert.Nil(t, err)
		assert.True(t, fsm.Apply(makeLog(buf)).(bool))
	}

	// Verify it's in the state store.
	{
		_, roots, err := fsm.state.CARoots(nil)
		assert.Nil(t, err)
		assert.Len(t, roots, 2)
	}
}

func TestFSM_CABuiltinProvider(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	assert.Nil(t, err)

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
		assert.Nil(t, err)
		assert.True(t, fsm.Apply(makeLog(buf)).(bool))
	}

	// Verify it's in the state store.
	{
		_, state, err := fsm.state.CAProviderState("foo")
		assert.Nil(t, err)
		assert.Equal(t, expected, state)
	}
}

func TestFSM_ConfigEntry(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	require.NoError(t, err)

	// Create a simple config entry
	entry := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"foo": "bar",
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	// Create a new request.
	req := &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryUpsert,
		Entry: entry,
	}

	{
		buf, err := structs.Encode(structs.ConfigEntryRequestType, req)
		require.NoError(t, err)
		resp := fsm.Apply(makeLog(buf))
		if _, ok := resp.(error); ok {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Verify it's in the state store.
	{
		_, config, err := fsm.state.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
		require.NoError(t, err)
		entry.RaftIndex.CreateIndex = 1
		entry.RaftIndex.ModifyIndex = 1
		require.Equal(t, entry, config)
	}
}

func TestFSM_ConfigEntry_StatusCAS(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	require.NoError(t, err)

	// Create a simple config entry
	entry := &structs.APIGatewayConfigEntry{
		Kind:           structs.APIGateway,
		Name:           "global",
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		Status: structs.Status{
			Conditions: []structs.Condition{{
				Status: string(api.ConditionStatusTrue),
			}},
		},
	}

	// Create a new request.
	req := &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryUpsertCAS,
		Entry: entry,
	}

	{
		buf, err := structs.Encode(structs.ConfigEntryRequestType, req)
		require.NoError(t, err)
		resp := fsm.Apply(makeLog(buf))
		if _, ok := resp.(error); ok {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Verify it's in the state store.
	{
		_, config, err := fsm.state.ConfigEntry(nil, structs.APIGateway, "global", nil)
		require.NoError(t, err)
		entry.RaftIndex.CreateIndex = 1
		entry.RaftIndex.ModifyIndex = 1
		require.Equal(t, entry.DefaultStatus(), config.(*structs.APIGatewayConfigEntry).GetStatus())
	}

	// do a status update
	entry.Status = structs.Status{
		Conditions: []structs.Condition{{
			Status: string(api.ConditionStatusTrue),
		}},
	}
	req = &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryUpsertWithStatusCAS,
		Entry: entry,
	}

	{
		buf, err := structs.Encode(structs.ConfigEntryRequestType, req)
		require.NoError(t, err)
		resp := fsm.Apply(makeLog(buf))
		if _, ok := resp.(error); ok {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Verify it's in the state store.
	{
		_, config, err := fsm.state.ConfigEntry(nil, structs.APIGateway, "global", nil)
		require.NoError(t, err)
		entry.RaftIndex.ModifyIndex = 2
		conditions := config.(*structs.APIGatewayConfigEntry).Status.Conditions
		require.Len(t, conditions, 1)
		require.Equal(t, string(api.ConditionStatusTrue), conditions[0].Status)
	}

	// attempt to change the status with a regular update and make sure it's ignored
	entry.Status = structs.Status{
		Conditions: []structs.Condition{{
			Status: "Bar",
		}},
	}
	req = &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryUpsertCAS,
		Entry: entry,
	}

	{
		buf, err := structs.Encode(structs.ConfigEntryRequestType, req)
		require.NoError(t, err)
		resp := fsm.Apply(makeLog(buf))
		if _, ok := resp.(error); ok {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Verify it's in the state store.
	{
		_, config, err := fsm.state.ConfigEntry(nil, structs.APIGateway, "global", nil)
		require.NoError(t, err)
		conditions := config.(*structs.APIGatewayConfigEntry).Status.Conditions
		require.Len(t, conditions, 1)
		require.Equal(t, string(api.ConditionStatusTrue), conditions[0].Status)
	}
}

func TestFSM_ConfigEntry_DeleteCAS(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	require.NoError(t, err)

	// Create a simple config entry and write it to the state store.
	entry := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "global",
	}
	require.NoError(t, fsm.state.EnsureConfigEntry(1, entry))

	// Raft index is populated by EnsureConfigEntry, hold on to it so that we can
	// restore it later.
	raftIndex := entry.RaftIndex
	require.NotZero(t, raftIndex.ModifyIndex)

	// Attempt a CAS delete with an invalid index.
	entry = entry.Clone()
	entry.RaftIndex = structs.RaftIndex{
		ModifyIndex: 99,
	}
	req := &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryDeleteCAS,
		Entry: entry,
	}
	buf, err := structs.Encode(structs.ConfigEntryRequestType, req)
	require.NoError(t, err)

	// Expect to get boolean false back.
	rsp := fsm.Apply(makeLog(buf))
	didDelete, isBool := rsp.(bool)
	require.True(t, isBool)
	require.False(t, didDelete)

	// Attempt a CAS delete with a valid index.
	entry.RaftIndex = raftIndex
	buf, err = structs.Encode(structs.ConfigEntryRequestType, req)
	require.NoError(t, err)

	// Expect to get boolean true back.
	rsp = fsm.Apply(makeLog(buf))
	didDelete, isBool = rsp.(bool)
	require.True(t, isBool)
	require.True(t, didDelete)
}

func TestFSM_Resources(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)

	handle := &testRaftHandle{}
	storageBackend := newStorageBackend(t, handle)
	fsm := NewFromDeps(Deps{
		Logger:         logger,
		NewStateStore:  func() *state.Store { return state.NewStateStore(nil) },
		StorageBackend: storageBackend,
	})
	handle.apply = func(msg []byte) (any, error) {
		buf := append(
			[]byte{uint8(structs.ResourceOperationType)},
			msg...,
		)
		return fsm.Apply(makeLog(buf)), nil
	}

	resource, err := storageBackend.WriteCAS(context.Background(), &pbresource.Resource{
		Id: &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "test",
				GroupVersion: "v1",
				Kind:         "foo",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				PeerName:  "local",
				Namespace: "default",
			},
			Name: "bar",
			Uid:  "a",
		},
	})
	require.NoError(t, err)

	storedResource, err := storageBackend.Read(context.Background(), storage.EventualConsistency, resource.Id)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, resource, storedResource)
}

// This adapts another test by chunking the encoded data and then performing
// out-of-order applies of half the logs. It then snapshots, restores to a new
// FSM, and applies the rest. The goal is to verify that chunking snapshotting
// works as expected.
func TestFSM_Chunking_Lifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	logger := testutil.Logger(t)

	fsm := NewFromDeps(Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: newStorageBackend(t, nil),
	})

	var logOfLogs [][]*raft.Log
	for i := 0; i < 10; i++ {
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
				Tags:    []string{"primary"},
				Port:    8000,
			},
			Check: &structs.HealthCheck{
				Node:      fmt.Sprintf("foo%d", i),
				CheckID:   "db",
				Name:      "db connectivity",
				Status:    api.HealthPassing,
				ServiceID: "db",
			},
		}

		buf, err := structs.Encode(structs.RegisterRequestType, req)
		require.NoError(t, err)

		var logs []*raft.Log

		for j, b := range buf {
			chunkInfo := &raftchunkingtypes.ChunkInfo{
				OpNum:       uint64(32 + i),
				SequenceNum: uint32(j),
				NumChunks:   uint32(len(buf)),
			}
			chunkBytes, err := proto.Marshal(chunkInfo)
			require.NoError(t, err)

			logs = append(logs, &raft.Log{
				Data:       []byte{b},
				Extensions: chunkBytes,
			})
		}
		logOfLogs = append(logOfLogs, logs)
	}

	// The reason for the skipping is to test out-of-order applies which are
	// theoretically possible. Apply some logs from each set of chunks, but not
	// the full set, and out of order.
	for _, logs := range logOfLogs {
		resp := fsm.chunker.Apply(logs[8])
		assert.Nil(t, resp)
		resp = fsm.chunker.Apply(logs[0])
		assert.Nil(t, resp)
		resp = fsm.chunker.Apply(logs[3])
		assert.Nil(t, resp)
	}

	// Verify we are not registered
	for i := 0; i < 10; i++ {
		_, node, err := fsm.state.GetNode(fmt.Sprintf("foo%d", i), nil, "")
		require.NoError(t, err)
		assert.Nil(t, node)
	}

	// Snapshot, restore elsewhere, apply the rest of the logs, make sure it
	// looks right
	snap, err := fsm.Snapshot()
	require.NoError(t, err)
	defer snap.Release()

	sinkBuf := bytes.NewBuffer(nil)
	sink := &MockSink{sinkBuf, false}
	err = snap.Persist(sink)
	require.NoError(t, err)

	fsm2 := NewFromDeps(Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: newStorageBackend(t, nil),
	})
	err = fsm2.Restore(sink)
	require.NoError(t, err)

	// Verify we are still not registered
	for i := 0; i < 10; i++ {
		_, node, err := fsm2.state.GetNode(fmt.Sprintf("foo%d", i), nil, "")
		require.NoError(t, err)
		assert.Nil(t, node)
	}

	// Apply the rest of the logs
	for _, logs := range logOfLogs {
		var resp interface{}
		for i, log := range logs {
			switch i {
			case 0, 3, 8:
			default:
				resp = fsm2.chunker.Apply(log)
				if i != len(logs)-1 {
					assert.Nil(t, resp)
				}
			}
		}
		_, ok := resp.(raftchunking.ChunkingSuccess)
		assert.True(t, ok)
	}

	// Verify we are registered
	for i := 0; i < 10; i++ {
		_, node, err := fsm2.state.GetNode(fmt.Sprintf("foo%d", i), nil, "")
		require.NoError(t, err)
		assert.NotNil(t, node)

		// Verify service registered
		_, services, err := fsm2.state.NodeServices(nil, fmt.Sprintf("foo%d", i), structs.DefaultEnterpriseMetaInDefaultPartition(), "")
		require.NoError(t, err)
		require.NotNil(t, services)
		_, ok := services.Services["db"]
		assert.True(t, ok)

		// Verify check
		_, checks, err := fsm2.state.NodeChecks(nil, fmt.Sprintf("foo%d", i), nil, "")
		require.NoError(t, err)
		require.NotNil(t, checks)
		assert.Equal(t, string(checks[0].CheckID), "db")
	}
}

func TestFSM_Chunking_TermChange(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	require.NoError(t, err)

	req := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tags:    []string{"primary"},
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
	require.NoError(t, err)

	// Only need two chunks to test this
	chunks := [][]byte{
		buf[0:2],
		buf[2:],
	}
	var logs []*raft.Log
	for i, b := range chunks {
		chunkInfo := &raftchunkingtypes.ChunkInfo{
			OpNum:       uint64(32),
			SequenceNum: uint32(i),
			NumChunks:   uint32(len(chunks)),
		}
		chunkBytes, err := proto.Marshal(chunkInfo)
		if err != nil {
			t.Fatal(err)
		}
		logs = append(logs, &raft.Log{
			Term:       uint64(i),
			Data:       b,
			Extensions: chunkBytes,
		})
	}

	// We should see nil for both
	for _, log := range logs {
		resp := fsm.chunker.Apply(log)
		assert.Nil(t, resp)
	}

	// Now verify the other baseline, that when the term doesn't change we see
	// non-nil. First make the chunker have a clean state, then set the terms
	// to be the same.
	err = fsm.chunker.RestoreState(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	logs[1].Term = uint64(0)

	// We should see nil only for the first one
	for i, log := range logs {
		resp := fsm.chunker.Apply(log)
		if i == 0 {
			assert.Nil(t, resp)
		}
		if i == 1 {
			assert.NotNil(t, resp)
		}
	}
}

func newStorageBackend(t *testing.T, handle raftstorage.Handle) *raftstorage.Backend {
	t.Helper()

	backend, err := raftstorage.NewBackend(handle, testutil.Logger(t))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go backend.Run(ctx)

	return backend
}

type testRaftHandle struct {
	apply func(msg []byte) (any, error)
}

func (h *testRaftHandle) Apply(msg []byte) (any, error)              { return h.apply(msg) }
func (testRaftHandle) IsLeader() bool                                { return true }
func (testRaftHandle) EnsureStrongConsistency(context.Context) error { return nil }
func (testRaftHandle) DialLeader() (*grpc.ClientConn, error) {
	return nil, errors.New("DialLeader not implemented")
}
