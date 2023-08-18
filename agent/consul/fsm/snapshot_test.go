// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fsm

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/go-raftchunking"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestFSM_SnapshotRestore_CE(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)

	handle := &testRaftHandle{}
	storageBackend := newStorageBackend(t, handle)
	handle.apply = func(buf []byte) (any, error) { return storageBackend.Apply(buf, 123), nil }

	fsm := NewFromDeps(Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: storageBackend,
	})

	fsm.state.SystemMetadataSet(10, &structs.SystemMetadataEntry{Key: structs.SystemMetadataVirtualIPsEnabled, Value: "true"})

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
	require.NoError(t, fsm.state.EnsureNode(1, node1))
	require.NoError(t, fsm.state.EnsureNode(2, node2))

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

	psn := structs.PeeredServiceName{ServiceName: structs.NewServiceName("web", nil)}
	vip, err := fsm.state.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.1")

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
		Rules:       structs.ACLPolicyGlobalManagementRules,
	}
	policy.SetHash(true)
	require.NoError(t, fsm.state.ACLPolicySet(1, policy))

	role := &structs.ACLRole{
		ID:          "86dedd19-8fae-4594-8294-4e6948a81f9a",
		Name:        "some-role",
		Description: "test snapshot role",
		ServiceIdentities: []*structs.ACLServiceIdentity{
			{
				ServiceName: "example",
			},
		},
	}
	role.SetHash(true)
	require.NoError(t, fsm.state.ACLRoleSet(1, role))

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
	}
	require.NoError(t, fsm.state.ACLBootstrap(10, 0, token))

	method := &structs.ACLAuthMethod{
		Name:        "some-method",
		Type:        "testing",
		Description: "test snapshot auth method",
		Config: map[string]interface{}{
			"SessionID": "952ebfa8-2a42-46f0-bcd3-fd98a842000e",
		},
	}
	require.NoError(t, fsm.state.ACLAuthMethodSet(1, method))

	method = &structs.ACLAuthMethod{
		Name:        "some-method2",
		Type:        "testing",
		Description: "test snapshot auth method",
	}
	require.NoError(t, fsm.state.ACLAuthMethodSet(1, method))

	bindingRule := &structs.ACLBindingRule{
		ID:          "85184c52-5997-4a84-9817-5945f2632a17",
		Description: "test snapshot binding rule",
		AuthMethod:  "some-method",
		Selector:    "serviceaccount.namespace==default",
		BindType:    structs.BindingRuleBindTypeService,
		BindName:    "${serviceaccount.name}",
	}
	require.NoError(t, fsm.state.ACLBindingRuleSet(1, bindingRule))

	fsm.state.KVSSet(11, &structs.DirEntry{
		Key:   "/remove",
		Value: []byte("foo"),
	})
	fsm.state.KVSDelete(12, "/remove", nil)
	idx, _, err := fsm.state.KVSList(nil, "/remove", nil)
	require.NoError(t, err)
	require.EqualValues(t, 12, idx, "bad index")

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
	require.NoError(t, fsm.state.CoordinateBatchUpdate(13, updates))

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
	require.NoError(t, fsm.state.PreparedQuerySet(14, &query))

	autopilotConf := &structs.AutopilotConfig{
		CleanupDeadServers:   true,
		LastContactThreshold: 100 * time.Millisecond,
		MaxTrailingLogs:      222,
	}
	require.NoError(t, fsm.state.AutopilotSetConfig(15, autopilotConf))

	// Legacy Intentions
	ixn := structs.TestIntention(t)
	ixn.ID = generateUUID()
	ixn.RaftIndex = structs.RaftIndex{
		CreateIndex: 14,
		ModifyIndex: 14,
	}
	//nolint:staticcheck
	require.NoError(t, fsm.state.LegacyIntentionSet(14, ixn))

	// CA Roots
	roots := []*structs.CARoot{
		connect.TestCA(t, nil),
		connect.TestCA(t, nil),
	}
	for _, r := range roots[1:] {
		r.Active = false
	}
	ok, err := fsm.state.CARootSetCAS(15, 0, roots)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = fsm.state.CASetProviderState(16, &structs.CAConsulProviderState{
		ID:         "asdf",
		PrivateKey: "foo",
		RootCert:   "bar",
	})
	require.NoError(t, err)
	require.True(t, ok)

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
	require.NoError(t, err)

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
	require.NoError(t, fsm.state.EnsureConfigEntry(18, serviceConfig))
	require.NoError(t, fsm.state.EnsureConfigEntry(19, proxyConfig))

	ingress := &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "ingress",
		Listeners: []structs.IngressListener{
			{
				Port:     8080,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name: "foo",
					},
				},
			},
		},
	}
	require.NoError(t, fsm.state.EnsureConfigEntry(20, ingress))
	_, gatewayServices, err := fsm.state.GatewayServices(nil, "ingress", structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)

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
	require.NoError(t, err)

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
	require.NoError(t, fsm.state.FederationStateSet(21, fedState1))
	require.NoError(t, fsm.state.FederationStateSet(22, fedState2))

	// Update a node, service and health check to make sure the ModifyIndexes are preserved correctly after restore.
	require.NoError(t, fsm.state.EnsureNode(23, &structs.Node{
		ID:         "610918a6-464f-fa9b-1a95-03bd6e88ed92",
		Node:       "foo",
		Datacenter: "dc1",
		Address:    "127.0.0.3",
	}))
	require.NoError(t, fsm.state.EnsureService(24, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5001}))
	require.NoError(t, fsm.state.EnsureCheck(25, &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "web",
		Name:      "web connectivity",
		Status:    api.HealthCritical,
		ServiceID: "web",
	}))

	// system metadata
	systemMetadataEntry := &structs.SystemMetadataEntry{
		Key: "key1", Value: "val1",
	}
	require.NoError(t, fsm.state.SystemMetadataSet(25, systemMetadataEntry))

	// service-intentions
	serviceIxn := &structs.ServiceIntentionsConfigEntry{
		Kind: structs.ServiceIntentions,
		Name: "foo",
		Sources: []*structs.SourceIntention{
			{
				Name:   "bar",
				Action: structs.IntentionActionAllow,
			},
		},
	}
	require.NoError(t, fsm.state.EnsureConfigEntry(26, serviceIxn))
	psn = structs.PeeredServiceName{ServiceName: structs.NewServiceName("foo", nil)}
	vip, err = fsm.state.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.2")

	// mesh config entry
	meshConfig := &structs.MeshConfigEntry{
		TransparentProxy: structs.TransparentProxyMeshConfig{
			MeshDestinationsOnly: true,
		},
	}
	require.NoError(t, fsm.state.EnsureConfigEntry(27, meshConfig))

	// Connect-native services for virtual IP generation
	systemMetadataEntry = &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataVirtualIPsEnabled,
		Value: "true",
	}
	require.NoError(t, fsm.state.SystemMetadataSet(28, systemMetadataEntry))

	fsm.state.EnsureService(29, "foo", &structs.NodeService{
		ID:      "frontend",
		Service: "frontend",
		Address: "127.0.0.1",
		Port:    8000,
		Connect: connectConf,
	})
	psn = structs.PeeredServiceName{ServiceName: structs.NewServiceName("frontend", nil)}
	vip, err = fsm.state.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.3")

	fsm.state.EnsureService(30, "foo", &structs.NodeService{
		ID:      "backend",
		Service: "backend",
		Address: "127.0.0.1",
		Port:    9000,
		Connect: connectConf,
	})
	psn = structs.PeeredServiceName{ServiceName: structs.NewServiceName("backend", nil)}
	vip, err = fsm.state.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.4")

	_, serviceNames, err := fsm.state.ServiceNamesOfKind(nil, structs.ServiceKindTypical)
	require.NoError(t, err)

	expect := []string{"backend", "db", "frontend", "web"}
	for i, sn := range serviceNames {
		require.Equal(t, expect[i], sn.Service.Name)
	}

	// Peerings
	require.NoError(t, fsm.state.PeeringWrite(31, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   "1fabcd52-1d46-49b0-b1d8-71559aee47f5",
			Name: "qux",
		},
		SecretsRequest: &pbpeering.SecretsWriteRequest{
			PeerID: "1fabcd52-1d46-49b0-b1d8-71559aee47f5",
			Request: &pbpeering.SecretsWriteRequest_GenerateToken{
				GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
					EstablishmentSecret: "baaeea83-8419-4aa8-ac89-14e7246a3d2f",
				},
			},
		},
	}))

	// Peering Trust Bundles
	require.NoError(t, fsm.state.PeeringTrustBundleWrite(32, &pbpeering.PeeringTrustBundle{
		TrustDomain: "qux.com",
		PeerName:    "qux",
		RootPEMs:    []string{"qux certificate bundle"},
	}))

	// Issue two more secrets writes so that there are three secrets associated with the peering:
	// - Establishment: "389bbcdf-1c31-47d6-ae96-f2a3f4c45f84"
	// - Pending: "0b7812d4-32d9-4e54-b1b3-4d97084982a0"
	require.NoError(t, fsm.state.PeeringSecretsWrite(34, &pbpeering.SecretsWriteRequest{
		PeerID: "1fabcd52-1d46-49b0-b1d8-71559aee47f5",
		Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
			ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
				EstablishmentSecret: "baaeea83-8419-4aa8-ac89-14e7246a3d2f",
				PendingStreamSecret: "0b7812d4-32d9-4e54-b1b3-4d97084982a0",
			},
		},
	}))
	require.NoError(t, fsm.state.PeeringSecretsWrite(33, &pbpeering.SecretsWriteRequest{
		PeerID: "1fabcd52-1d46-49b0-b1d8-71559aee47f5",
		Request: &pbpeering.SecretsWriteRequest_GenerateToken{
			GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
				EstablishmentSecret: "389bbcdf-1c31-47d6-ae96-f2a3f4c45f84",
			},
		},
	}))

	// Add a service-resolver entry to get a virtual IP for service goo
	resolverEntry := &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "goo",
	}
	require.NoError(t, fsm.state.EnsureConfigEntry(34, resolverEntry))
	vip, err = fsm.state.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.NewServiceName("goo", nil)})
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.5")

	// Resources
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

	// Snapshot
	snap, err := fsm.Snapshot()
	require.NoError(t, err)
	defer snap.Release()

	// Persist
	buf := bytes.NewBuffer(nil)
	sink := &MockSink{buf, false}
	require.NoError(t, snap.Persist(sink))

	// create an encoder to handle some custom persisted data
	// this is mainly to inject data that would no longer ever
	// be persisted but that we still need to be able to restore
	encoder := codec.NewEncoder(sink, structs.MsgpackHandle)

	// Persist a ACLToken without a Hash - the state store will
	// now tack these on but we want to ensure we can restore
	// tokens without a hash and have the hash be set.
	token2 := &structs.ACLToken{
		AccessorID:  "4464e4c2-1c55-4c37-978a-66cb3abe6587",
		SecretID:    "fc8708dc-c5ae-4bb2-a9af-a1ca456548fb",
		Description: "Test No Hash",
		CreateTime:  time.Now(),
		Local:       false,
		Policies: []structs.ACLTokenPolicyLink{
			{
				Name: "global-management",
				ID:   structs.ACLPolicyGlobalManagementID,
			},
		},
		RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
	}

	_, err = sink.Write([]byte{byte(structs.ACLTokenSetRequestType)})
	require.NoError(t, err)
	require.NoError(t, encoder.Encode(&token2))

	// Try to restore on a new FSM
	storageBackend2 := newStorageBackend(t, nil)

	fsm2 := NewFromDeps(Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: storageBackend2,
	})

	// Do a restore
	require.NoError(t, fsm2.Restore(sink))

	// Verify the contents
	_, nodes, err := fsm2.state.Nodes(nil, nil, "")
	require.NoError(t, err)
	require.Len(t, nodes, 2, "incorect number of nodes: %v", nodes)

	// validate the first node. Note that this test relies on stable
	// iteration through the memdb index and the fact that node2 has
	// a name of "baz" so it should be indexed before node1 with a
	// name of "foo". If memdb our our indexing changes this is likely
	// to break.
	require.Equal(t, node2.ID, nodes[0].ID)
	require.Equal(t, "baz", nodes[0].Node)
	require.Equal(t, "dc1", nodes[0].Datacenter)
	require.Equal(t, "127.0.0.2", nodes[0].Address)
	require.Len(t, nodes[0].Meta, 1)
	require.Equal(t, "testing123", nodes[0].Meta["testMeta"])
	require.Len(t, nodes[0].TaggedAddresses, 1)
	require.Equal(t, "1.2.3.4", nodes[0].TaggedAddresses["hello"])
	require.Equal(t, uint64(2), nodes[0].CreateIndex)
	require.Equal(t, uint64(2), nodes[0].ModifyIndex)

	require.Equal(t, node1.ID, nodes[1].ID)
	require.Equal(t, "foo", nodes[1].Node)
	require.Equal(t, "dc1", nodes[1].Datacenter)
	require.Equal(t, "127.0.0.3", nodes[1].Address)
	require.Empty(t, nodes[1].TaggedAddresses)
	require.Equal(t, uint64(1), nodes[1].CreateIndex)
	require.Equal(t, uint64(23), nodes[1].ModifyIndex)

	_, fooSrv, err := fsm2.state.NodeServices(nil, "foo", nil, "")
	require.NoError(t, err)
	require.Len(t, fooSrv.Services, 4)
	require.Contains(t, fooSrv.Services["db"].Tags, "primary")
	require.True(t, stringslice.Contains(fooSrv.Services["db"].Tags, "primary"))
	require.Equal(t, 5001, fooSrv.Services["db"].Port)
	require.Equal(t, uint64(4), fooSrv.Services["db"].CreateIndex)
	require.Equal(t, uint64(24), fooSrv.Services["db"].ModifyIndex)
	connectSrv := fooSrv.Services["web"]
	require.Equal(t, connectConf, connectSrv.Connect)
	require.Equal(t, uint64(3), fooSrv.Services["web"].CreateIndex)
	require.Equal(t, uint64(3), fooSrv.Services["web"].ModifyIndex)

	_, checks, err := fsm2.state.NodeChecks(nil, "foo", nil, "")
	require.NoError(t, err)
	require.Len(t, checks, 1)
	require.Equal(t, "foo", checks[0].Node)
	require.Equal(t, "web", checks[0].ServiceName)
	require.Equal(t, uint64(7), checks[0].CreateIndex)
	require.Equal(t, uint64(25), checks[0].ModifyIndex)

	// Verify virtual IPs are consistent.
	psn = structs.PeeredServiceName{ServiceName: structs.NewServiceName("web", nil)}
	vip, err = fsm2.state.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.1")
	psn = structs.PeeredServiceName{ServiceName: structs.NewServiceName("foo", nil)}
	vip, err = fsm2.state.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.2")
	psn = structs.PeeredServiceName{ServiceName: structs.NewServiceName("frontend", nil)}
	vip, err = fsm2.state.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.3")
	psn = structs.PeeredServiceName{ServiceName: structs.NewServiceName("backend", nil)}
	vip, err = fsm2.state.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.4")
	psn = structs.PeeredServiceName{ServiceName: structs.NewServiceName("goo", nil)}
	vip, err = fsm2.state.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, vip, "240.0.0.5")

	// Verify key is set
	_, d, err := fsm2.state.KVSGet(nil, "/test", nil)
	require.NoError(t, err)
	require.EqualValues(t, "foo", d.Value)

	// Verify session is restored
	idx, s, err := fsm2.state.SessionGet(nil, session.ID, nil)
	require.NoError(t, err)
	require.Equal(t, "foo", s.Node)
	require.EqualValues(t, 9, idx)

	// Verify ACL Binding Rule is restored
	_, bindingRule2, err := fsm2.state.ACLBindingRuleGetByID(nil, bindingRule.ID, nil)
	require.NoError(t, err)
	require.Equal(t, bindingRule, bindingRule2)

	// Verify ACL Auth Methods are restored
	_, authMethods, err := fsm2.state.ACLAuthMethodList(nil, nil)
	require.NoError(t, err)
	require.Len(t, authMethods, 2)
	require.Equal(t, "some-method", authMethods[0].Name)
	require.Equal(t, "some-method2", authMethods[1].Name)

	// Verify ACL Token is restored
	_, rtoken, err := fsm2.state.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	require.NoError(t, err)
	require.NotNil(t, rtoken)
	// the state store function will add on the Hash if its empty
	require.NotEmpty(t, rtoken.Hash)
	token.CreateTime = token.CreateTime.Round(0)
	rtoken.CreateTime = rtoken.CreateTime.Round(0)

	// note that this can work because the state store will add the Hash to the token before
	// storing. That token just happens to be a pointer to the one in this function so it
	// adds the Hash to our local var.
	require.Equal(t, token, rtoken)

	// Verify ACLToken without hash computes the Hash during restoration
	_, rtoken, err = fsm2.state.ACLTokenGetByAccessor(nil, token2.AccessorID, nil)
	require.NoError(t, err)
	require.NotNil(t, rtoken)
	require.NotEmpty(t, rtoken.Hash)
	// nil the Hash so we can compare them
	rtoken.Hash = nil
	token2.CreateTime = token2.CreateTime.Round(0)
	rtoken.CreateTime = rtoken.CreateTime.Round(0)
	require.Equal(t, token2, rtoken)

	// Verify the acl-token-bootstrap index was restored
	canBootstrap, index, err := fsm2.state.CanBootstrapACLToken()
	require.NoError(t, err)
	require.False(t, canBootstrap)
	require.True(t, index > 0)

	// Verify ACL Role is restored
	_, role2, err := fsm2.state.ACLRoleGetByID(nil, role.ID, nil)
	require.NoError(t, err)
	require.Equal(t, role, role2)

	// Verify ACL Policy is restored
	_, policy2, err := fsm2.state.ACLPolicyGetByID(nil, structs.ACLPolicyGlobalManagementID, nil)
	require.NoError(t, err)
	require.Equal(t, policy, policy2)

	// Verify tombstones are restored
	func() {
		snap := fsm2.state.Snapshot()
		defer snap.Close()
		stones, err := snap.Tombstones()
		require.NoError(t, err)
		stone := stones.Next().(*state.Tombstone)
		require.NotNil(t, stone)
		require.Equal(t, "/remove", stone.Key)
		require.Nil(t, stones.Next())
	}()

	// Verify coordinates are restored
	_, coords, err := fsm2.state.Coordinates(nil, nil)
	require.NoError(t, err)
	require.Equal(t, updates, coords)

	// Verify queries are restored.
	_, queries, err := fsm2.state.PreparedQueryList(nil)
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.Equal(t, &query, queries[0])

	// Verify autopilot config is restored.
	_, restoredConf, err := fsm2.state.AutopilotConfig()
	require.NoError(t, err)
	require.Equal(t, autopilotConf, restoredConf)

	// Verify legacy intentions are restored.
	_, ixns, err := fsm2.state.LegacyIntentions(nil, structs.WildcardEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	require.Len(t, ixns, 1)
	require.Equal(t, ixn, ixns[0])

	// Verify CA roots are restored.
	_, roots, err = fsm2.state.CARoots(nil)
	require.NoError(t, err)
	require.Len(t, roots, 2)

	// Verify provider state is restored.
	_, provider, err := fsm2.state.CAProviderState("asdf")
	require.NoError(t, err)
	require.Equal(t, "foo", provider.PrivateKey)
	require.Equal(t, "bar", provider.RootCert)

	// Verify CA configuration is restored.
	_, caConf, err := fsm2.state.CAConfig(nil)
	require.NoError(t, err)
	require.Equal(t, caConfig, caConf)

	// Verify config entries are restored
	_, serviceConfEntry, err := fsm2.state.ConfigEntry(nil, structs.ServiceDefaults, "foo", structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	require.Equal(t, serviceConfig, serviceConfEntry)

	_, proxyConfEntry, err := fsm2.state.ConfigEntry(nil, structs.ProxyDefaults, "global", structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	require.Equal(t, proxyConfig, proxyConfEntry)

	_, ingressRestored, err := fsm2.state.ConfigEntry(nil, structs.IngressGateway, "ingress", structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	require.Equal(t, ingress, ingressRestored)

	_, restoredGatewayServices, err := fsm2.state.GatewayServices(nil, "ingress", structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	require.Equal(t, gatewayServices, restoredGatewayServices)

	newChunkState, err := fsm2.chunker.CurrentState()
	require.NoError(t, err)
	require.Equal(t, newChunkState, chunkState)

	// Verify federation states are restored.
	_, fedStateLoaded1, err := fsm2.state.FederationStateGet(nil, "dc1")
	require.NoError(t, err)
	require.Equal(t, fedState1, fedStateLoaded1)
	_, fedStateLoaded2, err := fsm2.state.FederationStateGet(nil, "dc2")
	require.NoError(t, err)
	require.Equal(t, fedState2, fedStateLoaded2)

	// Verify usage data is correctly updated
	idx, nodeUsage, err := fsm2.state.NodeUsage()
	require.NoError(t, err)
	require.Equal(t, len(nodes), nodeUsage.Nodes)
	require.NotZero(t, idx)

	// Verify system metadata is restored.
	_, systemMetadataLoaded, err := fsm2.state.SystemMetadataList(nil)
	require.NoError(t, err)
	require.Len(t, systemMetadataLoaded, 2)
	require.Equal(t, systemMetadataEntry, systemMetadataLoaded[1])

	// Verify service-intentions is restored
	_, serviceIxnEntry, err := fsm2.state.ConfigEntry(nil, structs.ServiceIntentions, "foo", structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	require.Equal(t, serviceIxn, serviceIxnEntry)

	// Verify mesh config entry is restored
	_, meshConfigEntry, err := fsm2.state.ConfigEntry(nil, structs.MeshConfig, structs.MeshConfigMesh, structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	require.Equal(t, meshConfig, meshConfigEntry)

	_, restoredServiceNames, err := fsm2.state.ServiceNamesOfKind(nil, structs.ServiceKindTypical)
	require.NoError(t, err)

	expect = []string{"backend", "db", "frontend", "web"}
	for i, sn := range restoredServiceNames {
		require.Equal(t, expect[i], sn.Service.Name)
	}

	// Verify peering is restored
	idx, prngRestored, err := fsm2.state.PeeringRead(nil, state.Query{
		Value: "qux",
	})
	require.NoError(t, err)
	require.Equal(t, uint64(32), idx) // This is the index of the PTB write, which updates the peering
	require.NotNil(t, prngRestored)
	require.Equal(t, "qux", prngRestored.Name)

	// Verify peering secrets are restored
	secretsRestored, err := fsm2.state.PeeringSecretsRead(nil, "1fabcd52-1d46-49b0-b1d8-71559aee47f5")
	require.NoError(t, err)
	expectSecrets := &pbpeering.PeeringSecrets{
		PeerID: "1fabcd52-1d46-49b0-b1d8-71559aee47f5",
		Establishment: &pbpeering.PeeringSecrets_Establishment{
			SecretID: "389bbcdf-1c31-47d6-ae96-f2a3f4c45f84",
		},
		Stream: &pbpeering.PeeringSecrets_Stream{
			PendingSecretID: "0b7812d4-32d9-4e54-b1b3-4d97084982a0",
		},
	}
	prototest.AssertDeepEqual(t, expectSecrets, secretsRestored)

	uuids := []string{"389bbcdf-1c31-47d6-ae96-f2a3f4c45f84", "0b7812d4-32d9-4e54-b1b3-4d97084982a0"}
	for _, id := range uuids {
		free, err := fsm2.state.ValidateProposedPeeringSecretUUID(id)
		require.NoError(t, err)

		// The UUIDs in the peering secret should be tracked as in use.
		require.False(t, free)
	}

	// Verify peering trust bundle is restored
	idx, ptbRestored, err := fsm2.state.PeeringTrustBundleRead(nil, state.Query{
		Value: "qux",
	})
	require.NoError(t, err)
	require.Equal(t, uint64(32), idx)
	require.NotNil(t, ptbRestored)
	require.Equal(t, "qux.com", ptbRestored.TrustDomain)
	require.Equal(t, "qux", ptbRestored.PeerName)
	require.Len(t, ptbRestored.RootPEMs, 1)
	require.Equal(t, "qux certificate bundle", ptbRestored.RootPEMs[0])

	// Verify resources are restored.
	resourceRestored, err := storageBackend2.Read(context.Background(), storage.EventualConsistency, resource.Id)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, resource, resourceRestored)

	// Snapshot
	snap, err = fsm2.Snapshot()
	require.NoError(t, err)
	defer snap.Release()

	// Persist
	buf = bytes.NewBuffer(nil)
	sink = &MockSink{buf, false}
	require.NoError(t, snap.Persist(sink))

	// Try to restore on the old FSM and make sure it abandons the old state
	// store.
	abandonCh := fsm.state.AbandonCh()
	require.NoError(t, fsm.Restore(sink))
	select {
	case <-abandonCh:
	default:
		require.Fail(t, "Old state not abandoned")
	}
}

func TestFSM_BadRestore_CE(t *testing.T) {
	t.Parallel()
	// Create an FSM with some state.
	logger := testutil.Logger(t)

	fsm := NewFromDeps(Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: newStorageBackend(t, nil),
	})
	fsm.state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	abandonCh := fsm.state.AbandonCh()

	// Do a bad restore.
	buf := bytes.NewBuffer([]byte("bad snapshot"))
	sink := &MockSink{buf, false}
	require.Error(t, fsm.Restore(sink))

	// Verify the contents didn't get corrupted.
	_, nodes, err := fsm.state.Nodes(nil, nil, "")
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "foo", nodes[0].Node)
	require.Equal(t, "127.0.0.1", nodes[0].Address)
	require.Empty(t, nodes[0].TaggedAddresses)

	// Verify the old state store didn't get abandoned.
	select {
	case <-abandonCh:
		require.FailNow(t, "FSM state was abandoned when it should not have been")
	default:
	}
}

func TestFSM_BadSnapshot_NilCAConfig(t *testing.T) {
	t.Parallel()

	// Create an FSM with no config entry.
	logger := testutil.Logger(t)

	fsm := NewFromDeps(Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: newStorageBackend(t, nil),
	})

	// Snapshot
	snap, err := fsm.Snapshot()
	require.NoError(t, err)
	defer snap.Release()

	// Persist
	buf := bytes.NewBuffer(nil)
	sink := &MockSink{buf, false}
	require.NoError(t, snap.Persist(sink))

	// Try to restore on a new FSM
	fsm2 := NewFromDeps(Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: newStorageBackend(t, nil),
	})

	// Do a restore
	require.NoError(t, fsm2.Restore(sink))

	// Make sure there's no entry in the CA config table.
	state := fsm2.State()
	idx, config, err := state.CAConfig(nil)
	require.NoError(t, err)
	require.EqualValues(t, 0, idx)
	require.Nil(t, config)
}

// This test asserts that ServiceVirtualIP, which made a breaking change
// in 1.13.0, can still restore from older snapshots which use the old
// state.ServiceVirtualIP type.
func Test_restoreServiceVirtualIP(t *testing.T) {
	psn := structs.PeeredServiceName{
		ServiceName: structs.ServiceName{
			Name: "foo",
		},
	}

	run := func(t *testing.T, input interface{}) {
		t.Helper()

		var b []byte
		buf := bytes.NewBuffer(b)
		// Encode input
		encoder := codec.NewEncoder(buf, structs.MsgpackHandle)
		require.NoError(t, encoder.Encode(input))

		// Create a decoder
		dec := codec.NewDecoder(buf, structs.MsgpackHandle)

		logger := testutil.Logger(t)

		fsm := NewFromDeps(Deps{
			Logger: logger,
			NewStateStore: func() *state.Store {
				return state.NewStateStore(nil)
			},
			StorageBackend: newStorageBackend(t, nil),
		})

		restore := fsm.State().Restore()

		// Call restore
		require.NoError(t, restoreServiceVirtualIP(nil, restore, dec))
		require.NoError(t, restore.Commit())

		ip, err := fsm.State().VirtualIPForService(psn)
		require.NoError(t, err)

		// 240->224 due to addIPOffset
		require.Equal(t, "224.0.0.2", ip)
	}

	t.Run("new ServiceVirtualIP with PeeredServiceName", func(t *testing.T) {
		run(t, state.ServiceVirtualIP{
			Service:   psn,
			IP:        net.ParseIP("240.0.0.2"),
			RaftIndex: structs.RaftIndex{},
		})
	})
	t.Run("pre-1.13.0 ServiceVirtualIP with ServiceName", func(t *testing.T) {
		type compatServiceVirtualIP struct {
			Service   structs.ServiceName
			IP        net.IP
			RaftIndex structs.RaftIndex
		}

		run(t, compatServiceVirtualIP{
			Service: structs.ServiceName{
				Name: "foo",
			},
			IP:        net.ParseIP("240.0.0.2"),
			RaftIndex: structs.RaftIndex{},
		})
	})
}
