package fsm

import (
	"bytes"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSM_SnapshotRestore_OSS(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Add some state
	node1 := &structs.Node{
		ID:         "610918a6-464f-fa9b-1a95-03bd6e88ed92",
		Node:       "foo",
		Datacenter: "dc1",
		Address:    "127.0.0.1",
	}
	node2 := &structs.Node{
		ID:         "40e4a748-2192-161a-0510-9bf59fe950b5",
		Node:       "baz",
		Datacenter: "dc1",
		Address:    "127.0.0.2",
		TaggedAddresses: map[string]string{
			"hello": "1.2.3.4",
		},
		Meta: map[string]string{
			"testMeta": "testing123",
		},
	}
	assert.NoError(fsm.state.EnsureNode(1, node1))
	assert.NoError(fsm.state.EnsureNode(2, node2))

	// Add a service instance with Connect config.
	connectConf := structs.ServiceConnect{
		Native: true,
		Proxy: &structs.ServiceDefinitionConnectProxy{
			Command:  []string{"foo", "bar"},
			ExecMode: "a",
			Config: map[string]interface{}{
				"a": "qwer",
				"b": 4.3,
			},
		},
	}
	fsm.state.EnsureService(3, "foo", &structs.NodeService{
		ID:      "web",
		Service: "web",
		Tags:    nil,
		Address: "127.0.0.1",
		Port:    80,
		Connect: connectConf,
	})
	fsm.state.EnsureService(4, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5000})
	fsm.state.EnsureService(5, "baz", &structs.NodeService{ID: "web", Service: "web", Tags: nil, Address: "127.0.0.2", Port: 80})
	fsm.state.EnsureService(6, "baz", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"secondary"}, Address: "127.0.0.2", Port: 5000})
	fsm.state.EnsureCheck(7, &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "web",
		Name:      "web connectivity",
		Status:    api.HealthPassing,
		ServiceID: "web",
	})
	fsm.state.KVSSet(8, &structs.DirEntry{
		Key:   "/test",
		Value: []byte("foo"),
	})
	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	fsm.state.SessionCreate(9, session)
	policy := structs.ACLPolicy{
		ID:          structs.ACLPolicyGlobalManagementID,
		Name:        "global-management",
		Description: "Builtin Policy that grants unlimited access",
		Rules:       structs.ACLPolicyGlobalManagement,
		Syntax:      acl.SyntaxCurrent,
	}
	policy.SetHash(true)
	require.NoError(t, fsm.state.ACLPolicySet(1, &policy))

	token := &structs.ACLToken{
		AccessorID:  "30fca056-9fbb-4455-b94a-bf0e2bc575d6",
		SecretID:    "cbe1c6fd-d865-4034-9d6d-64fef7fb46a9",
		Description: "Bootstrap Token (Global Management)",
		Policies: []structs.ACLTokenPolicyLink{
			{
				ID: structs.ACLPolicyGlobalManagementID,
			},
		},
		CreateTime: time.Now(),
		Local:      false,
		// DEPRECATED (ACL-Legacy-Compat) - This is used so that the bootstrap token is still visible via the v1 acl APIs
		Type: structs.ACLTokenTypeManagement,
	}
	require.NoError(t, fsm.state.ACLBootstrap(10, 0, token, false))

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

	updates := structs.Coordinates{
		&structs.Coordinate{
			Node:  "baz",
			Coord: generateRandomCoordinate(),
		},
		&structs.Coordinate{
			Node:  "foo",
			Coord: generateRandomCoordinate(),
		},
	}
	if err := fsm.state.CoordinateBatchUpdate(13, updates); err != nil {
		t.Fatalf("err: %s", err)
	}

	query := structs.PreparedQuery{
		ID: generateUUID(),
		Service: structs.ServiceQuery{
			Service: "web",
		},
		RaftIndex: structs.RaftIndex{
			CreateIndex: 14,
			ModifyIndex: 14,
		},
	}
	if err := fsm.state.PreparedQuerySet(14, &query); err != nil {
		t.Fatalf("err: %s", err)
	}

	autopilotConf := &autopilot.Config{
		CleanupDeadServers:   true,
		LastContactThreshold: 100 * time.Millisecond,
		MaxTrailingLogs:      222,
	}
	if err := fsm.state.AutopilotSetConfig(15, autopilotConf); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Intentions
	ixn := structs.TestIntention(t)
	ixn.ID = generateUUID()
	ixn.RaftIndex = structs.RaftIndex{
		CreateIndex: 14,
		ModifyIndex: 14,
	}
	assert.Nil(fsm.state.IntentionSet(14, ixn))

	// CA Roots
	roots := []*structs.CARoot{
		connect.TestCA(t, nil),
		connect.TestCA(t, nil),
	}
	for _, r := range roots[1:] {
		r.Active = false
	}
	ok, err := fsm.state.CARootSetCAS(15, 0, roots)
	assert.Nil(err)
	assert.True(ok)

	ok, err = fsm.state.CASetProviderState(16, &structs.CAConsulProviderState{
		ID:         "asdf",
		PrivateKey: "foo",
		RootCert:   "bar",
	})
	assert.Nil(err)
	assert.True(ok)

	// CA Config
	caConfig := &structs.CAConfiguration{
		ClusterID: "foo",
		Provider:  "consul",
		Config: map[string]interface{}{
			"foo": "asdf",
			"bar": 6.5,
		},
	}
	err = fsm.state.CASetConfig(17, caConfig)
	assert.Nil(err)

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
	fsm2, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Do a restore
	if err := fsm2.Restore(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the contents
	_, nodes, err := fsm2.state.Nodes(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ID != node2.ID ||
		nodes[0].Node != "baz" ||
		nodes[0].Datacenter != "dc1" ||
		nodes[0].Address != "127.0.0.2" ||
		len(nodes[0].Meta) != 1 ||
		nodes[0].Meta["testMeta"] != "testing123" ||
		len(nodes[0].TaggedAddresses) != 1 ||
		nodes[0].TaggedAddresses["hello"] != "1.2.3.4" {
		t.Fatalf("bad: %v", nodes[0])
	}
	if nodes[1].ID != node1.ID ||
		nodes[1].Node != "foo" ||
		nodes[1].Datacenter != "dc1" ||
		nodes[1].Address != "127.0.0.1" ||
		len(nodes[1].TaggedAddresses) != 0 {
		t.Fatalf("bad: %v", nodes[1])
	}

	_, fooSrv, err := fsm2.state.NodeServices(nil, "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(fooSrv.Services) != 2 {
		t.Fatalf("Bad: %v", fooSrv)
	}
	if !lib.StrContains(fooSrv.Services["db"].Tags, "primary") {
		t.Fatalf("Bad: %v", fooSrv)
	}
	if fooSrv.Services["db"].Port != 5000 {
		t.Fatalf("Bad: %v", fooSrv)
	}
	connectSrv := fooSrv.Services["web"]
	if !reflect.DeepEqual(connectSrv.Connect, connectConf) {
		t.Fatalf("got: %v, want: %v", connectSrv.Connect, connectConf)
	}

	_, checks, err := fsm2.state.NodeChecks(nil, "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 1 {
		t.Fatalf("Bad: %v", checks)
	}

	// Verify key is set
	_, d, err := fsm2.state.KVSGet(nil, "/test")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(d.Value) != "foo" {
		t.Fatalf("bad: %v", d)
	}

	// Verify session is restored
	idx, s, err := fsm2.state.SessionGet(nil, session.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Node != "foo" {
		t.Fatalf("bad: %v", s)
	}
	if idx <= 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Verify ACL Token is restored
	_, a, err := fsm2.state.ACLTokenGetByAccessor(nil, token.AccessorID)
	require.NoError(t, err)
	require.Equal(t, token.AccessorID, a.AccessorID)
	require.Equal(t, token.ModifyIndex, a.ModifyIndex)

	// Verify the acl-token-bootstrap index was restored
	canBootstrap, index, err := fsm2.state.CanBootstrapACLToken()
	require.False(t, canBootstrap)
	require.True(t, index > 0)

	// Verify ACL Policy is restored
	_, policy2, err := fsm2.state.ACLPolicyGetByID(nil, structs.ACLPolicyGlobalManagementID)
	require.NoError(t, err)
	require.Equal(t, policy.Name, policy2.Name)

	// Verify tombstones are restored
	func() {
		snap := fsm2.state.Snapshot()
		defer snap.Close()
		stones, err := snap.Tombstones()
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		stone := stones.Next().(*state.Tombstone)
		if stone == nil {
			t.Fatalf("missing tombstone")
		}
		if stone.Key != "/remove" || stone.Index != 12 {
			t.Fatalf("bad: %v", stone)
		}
		if stones.Next() != nil {
			t.Fatalf("unexpected extra tombstones")
		}
	}()

	// Verify coordinates are restored
	_, coords, err := fsm2.state.Coordinates(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !reflect.DeepEqual(coords, updates) {
		t.Fatalf("bad: %#v", coords)
	}

	// Verify queries are restored.
	_, queries, err := fsm2.state.PreparedQueryList(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(queries) != 1 {
		t.Fatalf("bad: %#v", queries)
	}
	if !reflect.DeepEqual(queries[0], &query) {
		t.Fatalf("bad: %#v", queries[0])
	}

	// Verify autopilot config is restored.
	_, restoredConf, err := fsm2.state.AutopilotConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !reflect.DeepEqual(restoredConf, autopilotConf) {
		t.Fatalf("bad: %#v, %#v", restoredConf, autopilotConf)
	}

	// Verify intentions are restored.
	_, ixns, err := fsm2.state.Intentions(nil)
	assert.Nil(err)
	assert.Len(ixns, 1)
	assert.Equal(ixn, ixns[0])

	// Verify CA roots are restored.
	_, roots, err = fsm2.state.CARoots(nil)
	assert.Nil(err)
	assert.Len(roots, 2)

	// Verify provider state is restored.
	_, state, err := fsm2.state.CAProviderState("asdf")
	assert.Nil(err)
	assert.Equal("foo", state.PrivateKey)
	assert.Equal("bar", state.RootCert)

	// Verify CA configuration is restored.
	_, caConf, err := fsm2.state.CAConfig()
	assert.Nil(err)
	assert.Equal(caConfig, caConf)

	// Snapshot
	snap, err = fsm2.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Release()

	// Persist
	buf = bytes.NewBuffer(nil)
	sink = &MockSink{buf, false}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try to restore on the old FSM and make sure it abandons the old state
	// store.
	abandonCh := fsm.state.AbandonCh()
	if err := fsm.Restore(sink); err != nil {
		t.Fatalf("err: %v", err)
	}
	select {
	case <-abandonCh:
	default:
		t.Fatalf("bad")
	}
}

func TestFSM_BadRestore_OSS(t *testing.T) {
	t.Parallel()
	// Create an FSM with some state.
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	abandonCh := fsm.state.AbandonCh()

	// Do a bad restore.
	buf := bytes.NewBuffer([]byte("bad snapshot"))
	sink := &MockSink{buf, false}
	if err := fsm.Restore(sink); err == nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the contents didn't get corrupted.
	_, nodes, err := fsm.state.Nodes(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Node != "foo" ||
		nodes[0].Address != "127.0.0.1" ||
		len(nodes[0].TaggedAddresses) != 0 {
		t.Fatalf("bad: %v", nodes[0])
	}

	// Verify the old state store didn't get abandoned.
	select {
	case <-abandonCh:
		t.Fatalf("bad")
	default:
	}
}

func TestFSM_BadSnapshot_NilCAConfig(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	// Create an FSM with no config entry.
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
	fsm2, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Do a restore
	if err := fsm2.Restore(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure there's no entry in the CA config table.
	state := fsm2.State()
	idx, config, err := state.CAConfig()
	require.NoError(err)
	require.Equal(uint64(0), idx)
	if config != nil {
		t.Fatalf("config should be nil")
	}
}
