package fsm

import (
	"bytes"
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
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-raftchunking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSM_SnapshotRestore_OSS(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
	require.NoError(fsm.state.EnsureNode(1, node1))
	require.NoError(fsm.state.EnsureNode(2, node2))

	// Add a service instance with Connect config.
	connectConf := structs.ServiceConnect{
		Native: true,
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

	policy := &structs.ACLPolicy{
		ID:          structs.ACLPolicyGlobalManagementID,
		Name:        "global-management",
		Description: "Builtin Policy that grants unlimited access",
		Rules:       structs.ACLPolicyGlobalManagement,
		Syntax:      acl.SyntaxCurrent,
	}
	policy.SetHash(true)
	require.NoError(fsm.state.ACLPolicySet(1, policy))

	role := &structs.ACLRole{
		ID:          "86dedd19-8fae-4594-8294-4e6948a81f9a",
		Name:        "some-role",
		Description: "test snapshot role",
		ServiceIdentities: []*structs.ACLServiceIdentity{
			&structs.ACLServiceIdentity{
				ServiceName: "example",
			},
		},
	}
	role.SetHash(true)
	require.NoError(fsm.state.ACLRoleSet(1, role))

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
	require.NoError(fsm.state.ACLBootstrap(10, 0, token, false))

	method := &structs.ACLAuthMethod{
		Name:        "some-method",
		Type:        "testing",
		Description: "test snapshot auth method",
		Config: map[string]interface{}{
			"SessionID": "952ebfa8-2a42-46f0-bcd3-fd98a842000e",
		},
	}
	require.NoError(fsm.state.ACLAuthMethodSet(1, method))

	bindingRule := &structs.ACLBindingRule{
		ID:          "85184c52-5997-4a84-9817-5945f2632a17",
		Description: "test snapshot binding rule",
		AuthMethod:  "some-method",
		Selector:    "serviceaccount.namespace==default",
		BindType:    structs.BindingRuleBindTypeService,
		BindName:    "${serviceaccount.name}",
	}
	require.NoError(fsm.state.ACLBindingRuleSet(1, bindingRule))

	fsm.state.KVSSet(11, &structs.DirEntry{
		Key:   "/remove",
		Value: []byte("foo"),
	})
	fsm.state.KVSDelete(12, "/remove", nil)
	idx, _, err := fsm.state.KVSList(nil, "/remove", nil)
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
	require.NoError(fsm.state.IntentionSet(14, ixn))

	// CA Roots
	roots := []*structs.CARoot{
		connect.TestCA(t, nil),
		connect.TestCA(t, nil),
	}
	for _, r := range roots[1:] {
		r.Active = false
	}
	ok, err := fsm.state.CARootSetCAS(15, 0, roots)
	require.NoError(err)
	assert.True(ok)

	ok, err = fsm.state.CASetProviderState(16, &structs.CAConsulProviderState{
		ID:         "asdf",
		PrivateKey: "foo",
		RootCert:   "bar",
	})
	require.NoError(err)
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
	require.NoError(err)

	// Config entries
	serviceConfig := &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "foo",
		Protocol: "http",
	}
	proxyConfig := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
	}
	require.NoError(fsm.state.EnsureConfigEntry(18, serviceConfig, structs.DefaultEnterpriseMeta()))
	require.NoError(fsm.state.EnsureConfigEntry(19, proxyConfig, structs.DefaultEnterpriseMeta()))

	// Raft Chunking
	chunkState := &raftchunking.State{
		ChunkMap: make(raftchunking.ChunkMap),
	}
	chunkState.ChunkMap[0] = []*raftchunking.ChunkInfo{
		{
			OpNum:       0,
			SequenceNum: 0,
			NumChunks:   3,
			Data:        []byte("foo"),
		},
		nil,
		{
			OpNum:       0,
			SequenceNum: 2,
			NumChunks:   3,
			Data:        []byte("bar"),
		},
	}
	chunkState.ChunkMap[20] = []*raftchunking.ChunkInfo{
		nil,
		{
			OpNum:       20,
			SequenceNum: 1,
			NumChunks:   2,
			Data:        []byte("bar"),
		},
	}
	err = fsm.chunker.RestoreState(chunkState)
	require.NoError(err)

	// Federation states
	fedState1 := &structs.FederationState{
		Datacenter: "dc1",
		MeshGateways: []structs.CheckServiceNode{
			{
				Node: &structs.Node{
					ID:         "664bac9f-4de7-4f1b-ad35-0e5365e8f329",
					Node:       "gateway1",
					Datacenter: "dc1",
					Address:    "1.2.3.4",
				},
				Service: &structs.NodeService{
					ID:      "mesh-gateway",
					Service: "mesh-gateway",
					Kind:    structs.ServiceKindMeshGateway,
					Port:    1111,
					Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
				},
				Checks: []*structs.HealthCheck{
					{
						Name:      "web connectivity",
						Status:    api.HealthPassing,
						ServiceID: "mesh-gateway",
					},
				},
			},
			{
				Node: &structs.Node{
					ID:         "3fb9a696-8209-4eee-a1f7-48600deb9716",
					Node:       "gateway2",
					Datacenter: "dc1",
					Address:    "9.8.7.6",
				},
				Service: &structs.NodeService{
					ID:      "mesh-gateway",
					Service: "mesh-gateway",
					Kind:    structs.ServiceKindMeshGateway,
					Port:    2222,
					Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
				},
				Checks: []*structs.HealthCheck{
					{
						Name:      "web connectivity",
						Status:    api.HealthPassing,
						ServiceID: "mesh-gateway",
					},
				},
			},
		},
		UpdatedAt: time.Now().UTC(),
	}
	fedState2 := &structs.FederationState{
		Datacenter: "dc2",
		MeshGateways: []structs.CheckServiceNode{
			{
				Node: &structs.Node{
					ID:         "0f92b02e-9f51-4aa2-861b-4ddbc3492724",
					Node:       "gateway1",
					Datacenter: "dc2",
					Address:    "8.8.8.8",
				},
				Service: &structs.NodeService{
					ID:      "mesh-gateway",
					Service: "mesh-gateway",
					Kind:    structs.ServiceKindMeshGateway,
					Port:    3333,
					Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
				},
				Checks: []*structs.HealthCheck{
					{
						Name:      "web connectivity",
						Status:    api.HealthPassing,
						ServiceID: "mesh-gateway",
					},
				},
			},
			{
				Node: &structs.Node{
					ID:         "99a76121-1c3f-4023-88ef-805248beb10b",
					Node:       "gateway2",
					Datacenter: "dc2",
					Address:    "5.5.5.5",
				},
				Service: &structs.NodeService{
					ID:      "mesh-gateway",
					Service: "mesh-gateway",
					Kind:    structs.ServiceKindMeshGateway,
					Port:    4444,
					Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
				},
				Checks: []*structs.HealthCheck{
					{
						Name:      "web connectivity",
						Status:    api.HealthPassing,
						ServiceID: "mesh-gateway",
					},
				},
			},
		},
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(fsm.state.FederationStateSet(21, fedState1))
	require.NoError(fsm.state.FederationStateSet(22, fedState2))

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
	fsm2, err := New(nil, logger)
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

	_, fooSrv, err := fsm2.state.NodeServices(nil, "foo", nil)
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

	_, checks, err := fsm2.state.NodeChecks(nil, "foo", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 1 {
		t.Fatalf("Bad: %v", checks)
	}

	// Verify key is set
	_, d, err := fsm2.state.KVSGet(nil, "/test", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(d.Value) != "foo" {
		t.Fatalf("bad: %v", d)
	}

	// Verify session is restored
	idx, s, err := fsm2.state.SessionGet(nil, session.ID, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Node != "foo" {
		t.Fatalf("bad: %v", s)
	}
	if idx <= 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Verify ACL Binding Rule is restored
	_, bindingRule2, err := fsm2.state.ACLBindingRuleGetByID(nil, bindingRule.ID, nil)
	require.NoError(err)
	require.Equal(bindingRule, bindingRule2)

	// Verify ACL Auth Method is restored
	_, method2, err := fsm2.state.ACLAuthMethodGetByName(nil, method.Name, nil)
	require.NoError(err)
	require.Equal(method, method2)

	// Verify ACL Token is restored
	_, token2, err := fsm2.state.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	require.NoError(err)
	{
		// time.Time is tricky to compare generically when it takes a ser/deserialization round trip.
		require.True(token.CreateTime.Equal(token2.CreateTime))
		token2.CreateTime = token.CreateTime
	}
	require.Equal(token, token2)

	// Verify the acl-token-bootstrap index was restored
	canBootstrap, index, err := fsm2.state.CanBootstrapACLToken()
	require.False(canBootstrap)
	require.True(index > 0)

	// Verify ACL Role is restored
	_, role2, err := fsm2.state.ACLRoleGetByID(nil, role.ID, nil)
	require.NoError(err)
	require.Equal(role, role2)

	// Verify ACL Policy is restored
	_, policy2, err := fsm2.state.ACLPolicyGetByID(nil, structs.ACLPolicyGlobalManagementID, nil)
	require.NoError(err)
	require.Equal(policy, policy2)

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
	require.NoError(err)
	assert.Len(ixns, 1)
	assert.Equal(ixn, ixns[0])

	// Verify CA roots are restored.
	_, roots, err = fsm2.state.CARoots(nil)
	require.NoError(err)
	assert.Len(roots, 2)

	// Verify provider state is restored.
	_, state, err := fsm2.state.CAProviderState("asdf")
	require.NoError(err)
	assert.Equal("foo", state.PrivateKey)
	assert.Equal("bar", state.RootCert)

	// Verify CA configuration is restored.
	_, caConf, err := fsm2.state.CAConfig(nil)
	require.NoError(err)
	assert.Equal(caConfig, caConf)

	// Verify config entries are restored
	_, serviceConfEntry, err := fsm2.state.ConfigEntry(nil, structs.ServiceDefaults, "foo", structs.DefaultEnterpriseMeta())
	require.NoError(err)
	assert.Equal(serviceConfig, serviceConfEntry)

	_, proxyConfEntry, err := fsm2.state.ConfigEntry(nil, structs.ProxyDefaults, "global", structs.DefaultEnterpriseMeta())
	require.NoError(err)
	assert.Equal(proxyConfig, proxyConfEntry)

	newChunkState, err := fsm2.chunker.CurrentState()
	require.NoError(err)
	assert.Equal(newChunkState, chunkState)

	// Verify federation states are restored.
	_, fedStateLoaded1, err := fsm2.state.FederationStateGet(nil, "dc1")
	require.NoError(err)
	assert.Equal(fedState1, fedStateLoaded1)
	_, fedStateLoaded2, err := fsm2.state.FederationStateGet(nil, "dc2")
	require.NoError(err)
	assert.Equal(fedState2, fedStateLoaded2)

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
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
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
	fsm2, err := New(nil, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Do a restore
	if err := fsm2.Restore(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure there's no entry in the CA config table.
	state := fsm2.State()
	idx, config, err := state.CAConfig(nil)
	require.NoError(err)
	require.Equal(uint64(0), idx)
	if config != nil {
		t.Fatalf("config should be nil")
	}
}
