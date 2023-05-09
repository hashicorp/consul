package consul

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestHealth_ChecksInState(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Name:   "memory utilization",
			Status: api.HealthPassing,
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out2 structs.IndexedHealthChecks
	inState := structs.ChecksInStateRequest{
		Datacenter: "dc1",
		State:      api.HealthPassing,
	}
	if err := msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &inState, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks := out2.HealthChecks
	if len(checks) != 2 {
		t.Fatalf("Bad: %v", checks)
	}

	// Serf check is automatically added
	if checks[0].Name != "memory utilization" {
		t.Fatalf("Bad: %v", checks[0])
	}
	if checks[1].CheckID != structs.SerfCheckID {
		t.Fatalf("Bad: %v", checks[1])
	}
}

func TestHealth_ChecksInState_NodeMetaFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"somekey": "somevalue",
			"common":  "1",
		},
		Check: &structs.HealthCheck{
			Name:   "memory utilization",
			Status: api.HealthPassing,
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	arg = structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		NodeMeta: map[string]string{
			"common": "1",
		},
		Check: &structs.HealthCheck{
			Name:   "disk space",
			Status: api.HealthPassing,
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	cases := []struct {
		filters    map[string]string
		checkNames []string
	}{
		// Get foo's check by its unique meta value
		{
			filters:    map[string]string{"somekey": "somevalue"},
			checkNames: []string{"memory utilization"},
		},
		// Get both foo/bar's checks by their common meta value
		{
			filters:    map[string]string{"common": "1"},
			checkNames: []string{"disk space", "memory utilization"},
		},
		// Use an invalid meta value, should get empty result
		{
			filters:    map[string]string{"invalid": "nope"},
			checkNames: []string{},
		},
		// Use multiple filters to get foo's check
		{
			filters: map[string]string{
				"somekey": "somevalue",
				"common":  "1",
			},
			checkNames: []string{"memory utilization"},
		},
	}

	for _, tc := range cases {
		var out structs.IndexedHealthChecks
		inState := structs.ChecksInStateRequest{
			Datacenter:      "dc1",
			NodeMetaFilters: tc.filters,
			State:           api.HealthPassing,
		}
		if err := msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &inState, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		checks := out.HealthChecks
		if len(checks) != len(tc.checkNames) {
			t.Fatalf("Bad: %v, %v", checks, tc.checkNames)
		}

		for i, check := range checks {
			if tc.checkNames[i] != check.Name {
				t.Fatalf("Bad: %v %v", checks, tc.checkNames)
			}
		}
	}
}

func TestHealth_ChecksInState_DistanceSort(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.3"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	updates := structs.Coordinates{
		{Node: "foo", Coord: lib.GenerateCoordinate(1 * time.Millisecond)},
		{Node: "bar", Coord: lib.GenerateCoordinate(2 * time.Millisecond)},
	}
	if err := s1.fsm.State().CoordinateBatchUpdate(3, updates); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Name:   "memory utilization",
			Status: api.HealthPassing,
		},
	}

	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg.Node = "bar"
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Query relative to foo to make sure it shows up first in the list.
	var out2 structs.IndexedHealthChecks
	inState := structs.ChecksInStateRequest{
		Datacenter: "dc1",
		State:      api.HealthPassing,
		Source: structs.QuerySource{
			Datacenter: "dc1",
			Node:       "foo",
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &inState, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}
	checks := out2.HealthChecks
	if len(checks) != 3 {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[0].Node != "foo" {
		t.Fatalf("Bad: %v", checks[1])
	}

	// Now query relative to bar to make sure it shows up first.
	inState.Source.Node = "bar"
	if err := msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &inState, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}
	checks = out2.HealthChecks
	if len(checks) != 3 {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[0].Node != "bar" {
		t.Fatalf("Bad: %v", checks[1])
	}
}

func TestHealth_NodeChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Name:   "memory utilization",
			Status: api.HealthPassing,
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out2 structs.IndexedHealthChecks
	node := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	if err := msgpackrpc.CallWithCodec(codec, "Health.NodeChecks", &node, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks := out2.HealthChecks
	if len(checks) != 1 {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[0].Name != "memory utilization" {
		t.Fatalf("Bad: %v", checks)
	}
}

func TestHealth_ServiceChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    api.HealthPassing,
			ServiceID: "db",
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out2 structs.IndexedHealthChecks
	node := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
	}
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &node, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks := out2.HealthChecks
	if len(checks) != 1 {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[0].Name != "db connect" {
		t.Fatalf("Bad: %v", checks)
	}
}

func TestHealth_ServiceChecks_NodeMetaFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"somekey": "somevalue",
			"common":  "1",
		},
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
		},
		Check: &structs.HealthCheck{
			Name:      "memory utilization",
			Status:    api.HealthPassing,
			ServiceID: "db",
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	arg = structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		NodeMeta: map[string]string{
			"common": "1",
		},
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
		},
		Check: &structs.HealthCheck{
			Name:      "disk space",
			Status:    api.HealthPassing,
			ServiceID: "db",
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	cases := []struct {
		filters    map[string]string
		checkNames []string
	}{
		// Get foo's check by its unique meta value
		{
			filters:    map[string]string{"somekey": "somevalue"},
			checkNames: []string{"memory utilization"},
		},
		// Get both foo/bar's checks by their common meta value
		{
			filters:    map[string]string{"common": "1"},
			checkNames: []string{"disk space", "memory utilization"},
		},
		// Use an invalid meta value, should get empty result
		{
			filters:    map[string]string{"invalid": "nope"},
			checkNames: []string{},
		},
		// Use multiple filters to get foo's check
		{
			filters: map[string]string{
				"somekey": "somevalue",
				"common":  "1",
			},
			checkNames: []string{"memory utilization"},
		},
	}

	for _, tc := range cases {
		var out structs.IndexedHealthChecks
		args := structs.ServiceSpecificRequest{
			Datacenter:      "dc1",
			NodeMetaFilters: tc.filters,
			ServiceName:     "db",
		}
		if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		checks := out.HealthChecks
		if len(checks) != len(tc.checkNames) {
			t.Fatalf("Bad: %v, %v", checks, tc.checkNames)
		}

		for i, check := range checks {
			if tc.checkNames[i] != check.Name {
				t.Fatalf("Bad: %v %v", checks, tc.checkNames)
			}
		}
	}
}

func TestHealth_ServiceChecks_DistanceSort(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.3"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	updates := structs.Coordinates{
		{Node: "foo", Coord: lib.GenerateCoordinate(1 * time.Millisecond)},
		{Node: "bar", Coord: lib.GenerateCoordinate(2 * time.Millisecond)},
	}
	if err := s1.fsm.State().CoordinateBatchUpdate(3, updates); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    api.HealthPassing,
			ServiceID: "db",
		},
	}

	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg.Node = "bar"
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Query relative to foo to make sure it shows up first in the list.
	var out2 structs.IndexedHealthChecks
	node := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		Source: structs.QuerySource{
			Datacenter: "dc1",
			Node:       "foo",
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &node, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}
	checks := out2.HealthChecks
	if len(checks) != 2 {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[0].Node != "foo" {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[1].Node != "bar" {
		t.Fatalf("Bad: %v", checks)
	}

	// Now query relative to bar to make sure it shows up first.
	node.Source.Node = "bar"
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &node, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}
	checks = out2.HealthChecks
	if len(checks) != 2 {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[0].Node != "bar" {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[1].Node != "foo" {
		t.Fatalf("Bad: %v", checks)
	}
}

func TestHealth_ServiceNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerWithConfig(t, func(config *Config) {
		config.PeeringTestAllowPeerRegistrations = true
	})
	codec := rpcClient(t, s1)

	waitForLeaderEstablishment(t, s1)

	testingPeerNames := []string{"", "my-peer"}

	// TODO(peering): will have to seed this data differently in the future
	for _, peerName := range testingPeerNames {
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			PeerName:   peerName,
			Service: &structs.NodeService{
				ID:       "db",
				Service:  "db",
				Tags:     []string{"primary"},
				PeerName: peerName,
			},
			Check: &structs.HealthCheck{
				Name:      "db connect",
				Status:    api.HealthPassing,
				ServiceID: "db",
				PeerName:  peerName,
			},
		}
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			PeerName:   peerName,
			Service: &structs.NodeService{
				ID:       "db",
				Service:  "db",
				Tags:     []string{"replica"},
				PeerName: peerName,
			},
			Check: &structs.HealthCheck{
				Name:      "db connect",
				Status:    api.HealthWarning,
				ServiceID: "db",
				PeerName:  peerName,
			},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
	}

	verify := func(t *testing.T, out2 structs.IndexedCheckServiceNodes, peerName string) {
		nodes := out2.Nodes
		require.Len(t, nodes, 2)
		require.Equal(t, peerName, nodes[0].Node.PeerName)
		require.Equal(t, peerName, nodes[1].Node.PeerName)
		require.Equal(t, "bar", nodes[0].Node.Node)
		require.Equal(t, "foo", nodes[1].Node.Node)
		require.Equal(t, peerName, nodes[0].Service.PeerName)
		require.Equal(t, peerName, nodes[1].Service.PeerName)
		require.Contains(t, nodes[0].Service.Tags, "replica")
		require.Contains(t, nodes[1].Service.Tags, "primary")
		require.Equal(t, peerName, nodes[0].Checks[0].PeerName)
		require.Equal(t, peerName, nodes[1].Checks[0].PeerName)
		require.Equal(t, api.HealthWarning, nodes[0].Checks[0].Status)
		require.Equal(t, api.HealthPassing, nodes[1].Checks[0].Status)
	}

	for _, peerName := range testingPeerNames {
		testName := "peer named " + peerName
		if peerName == "" {
			testName = "local peer"
		}
		t.Run(testName, func(t *testing.T) {
			t.Run("with service tags", func(t *testing.T) {
				var out2 structs.IndexedCheckServiceNodes
				req := structs.ServiceSpecificRequest{
					Datacenter:  "dc1",
					ServiceName: "db",
					ServiceTags: []string{"primary"},
					TagFilter:   false,
					PeerName:    peerName,
				}
				require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2))
				verify(t, out2, peerName)
			})

			// Same should still work for <1.3 RPCs with singular tags
			// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
			// with 1.2.x is not required.
			t.Run("with legacy service tag", func(t *testing.T) {
				var out2 structs.IndexedCheckServiceNodes
				req := structs.ServiceSpecificRequest{
					Datacenter:  "dc1",
					ServiceName: "db",
					ServiceTag:  "primary",
					TagFilter:   false,
					PeerName:    peerName,
				}
				require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2))
				verify(t, out2, peerName)
			})
		})
	}
}

func TestHealth_ServiceNodes_BlockingQuery_withFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServer(t)
	codec := rpcClient(t, s1)

	waitForLeaderEstablishment(t, s1)

	register := func(t *testing.T, name, tag string) {
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			ID:         types.NodeID("43d419c0-433b-42c3-bf8a-193eba0b41a3"),
			Node:       "node1",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      name,
				Service: name,
				Tags:    []string{tag},
			},
		}
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
	}

	register(t, "web", "foo")

	var lastIndex uint64
	testutil.RunStep(t, "read original", func(t *testing.T) {
		var out structs.IndexedCheckServiceNodes
		req := structs.ServiceSpecificRequest{
			Datacenter:  "dc1",
			ServiceName: "web",
			QueryOptions: structs.QueryOptions{
				Filter: "foo in Service.Tags",
			},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out))

		require.Len(t, out.Nodes, 1)
		node := out.Nodes[0]
		require.Equal(t, "node1", node.Node.Node)
		require.Equal(t, "web", node.Service.Service)
		require.Equal(t, []string{"foo"}, node.Service.Tags)

		require.Equal(t, structs.QueryBackendBlocking, out.Backend)
		lastIndex = out.Index
	})

	testutil.RunStep(t, "read blocking query result", func(t *testing.T) {
		req := structs.ServiceSpecificRequest{
			Datacenter:  "dc1",
			ServiceName: "web",
			QueryOptions: structs.QueryOptions{
				Filter: "foo in Service.Tags",
			},
		}
		req.MinQueryIndex = lastIndex

		var out structs.IndexedCheckServiceNodes
		errCh := channelCallRPC(s1, "Health.ServiceNodes", &req, &out, nil)

		time.Sleep(200 * time.Millisecond)

		// Change the tags
		register(t, "web", "bar")

		if err := <-errCh; err != nil {
			require.NoError(t, err)
		}

		require.Equal(t, structs.QueryBackendBlocking, out.Backend)
		require.Len(t, out.Nodes, 0)
	})
}

func TestHealth_ServiceNodes_MultipleServiceTags(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tags:    []string{"primary", "v2"},
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    api.HealthPassing,
			ServiceID: "db",
		},
	}
	var out struct{}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

	arg = structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tags:    []string{"replica", "v2"},
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    api.HealthWarning,
			ServiceID: "db",
		},
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

	var out2 structs.IndexedCheckServiceNodes
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceTags: []string{"primary", "v2"},
		TagFilter:   true,
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2))

	nodes := out2.Nodes
	require.Len(t, nodes, 1)
	require.Equal(t, nodes[0].Node.Node, "foo")
	require.Contains(t, nodes[0].Service.Tags, "v2")
	require.Contains(t, nodes[0].Service.Tags, "primary")
	require.Equal(t, nodes[0].Checks[0].Status, api.HealthPassing)
}

func TestHealth_ServiceNodes_NodeMetaFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"somekey": "somevalue",
			"common":  "1",
		},
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
		},
		Check: &structs.HealthCheck{
			Name:      "memory utilization",
			Status:    api.HealthPassing,
			ServiceID: "db",
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg = structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		NodeMeta: map[string]string{
			"common": "1",
		},
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
		},
		Check: &structs.HealthCheck{
			Name:      "disk space",
			Status:    api.HealthWarning,
			ServiceID: "db",
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	cases := []struct {
		filters map[string]string
		nodes   structs.CheckServiceNodes
	}{
		// Get foo's check by its unique meta value
		{
			filters: map[string]string{"somekey": "somevalue"},
			nodes: structs.CheckServiceNodes{
				structs.CheckServiceNode{
					Node:   &structs.Node{Node: "foo"},
					Checks: structs.HealthChecks{&structs.HealthCheck{Name: "memory utilization"}},
				},
			},
		},
		// Get both foo/bar's checks by their common meta value
		{
			filters: map[string]string{"common": "1"},
			nodes: structs.CheckServiceNodes{
				structs.CheckServiceNode{
					Node:   &structs.Node{Node: "bar"},
					Checks: structs.HealthChecks{&structs.HealthCheck{Name: "disk space"}},
				},
				structs.CheckServiceNode{
					Node:   &structs.Node{Node: "foo"},
					Checks: structs.HealthChecks{&structs.HealthCheck{Name: "memory utilization"}},
				},
			},
		},
		// Use an invalid meta value, should get empty result
		{
			filters: map[string]string{"invalid": "nope"},
			nodes:   structs.CheckServiceNodes{},
		},
		// Use multiple filters to get foo's check
		{
			filters: map[string]string{
				"somekey": "somevalue",
				"common":  "1",
			},
			nodes: structs.CheckServiceNodes{
				structs.CheckServiceNode{
					Node:   &structs.Node{Node: "foo"},
					Checks: structs.HealthChecks{&structs.HealthCheck{Name: "memory utilization"}},
				},
			},
		},
	}

	for _, tc := range cases {
		var out structs.IndexedCheckServiceNodes
		req := structs.ServiceSpecificRequest{
			Datacenter:      "dc1",
			NodeMetaFilters: tc.filters,
			ServiceName:     "db",
		}
		if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(out.Nodes) != len(tc.nodes) {
			t.Fatalf("bad: %v, %v, filters: %v", out.Nodes, tc.nodes, tc.filters)
		}

		for i, node := range out.Nodes {
			checks := tc.nodes[i].Checks
			if len(node.Checks) != len(checks) {
				t.Fatalf("bad: %v, %v, filters: %v", node.Checks, checks, tc.filters)
			}
			for j, check := range node.Checks {
				if check.Name != checks[j].Name {
					t.Fatalf("bad: %v, %v, filters: %v", check, checks[j], tc.filters)
				}
			}
		}
	}
}

func TestHealth_ServiceNodes_DistanceSort(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.3"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	updates := structs.Coordinates{
		{Node: "foo", Coord: lib.GenerateCoordinate(1 * time.Millisecond)},
		{Node: "bar", Coord: lib.GenerateCoordinate(2 * time.Millisecond)},
	}
	if err := s1.fsm.State().CoordinateBatchUpdate(3, updates); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    api.HealthPassing,
			ServiceID: "db",
		},
	}

	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg.Node = "bar"
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Query relative to foo to make sure it shows up first in the list.
	var out2 structs.IndexedCheckServiceNodes
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		Source: structs.QuerySource{
			Datacenter: "dc1",
			Node:       "foo",
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}
	nodes := out2.Nodes
	if len(nodes) != 2 {
		t.Fatalf("Bad: %v", nodes)
	}
	if nodes[0].Node.Node != "foo" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[1].Node.Node != "bar" {
		t.Fatalf("Bad: %v", nodes[1])
	}

	// Now query relative to bar to make sure it shows up first.
	req.Source.Node = "bar"
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}
	nodes = out2.Nodes
	if len(nodes) != 2 {
		t.Fatalf("Bad: %v", nodes)
	}
	if nodes[0].Node.Node != "bar" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[1].Node.Node != "foo" {
		t.Fatalf("Bad: %v", nodes[1])
	}
}

func TestHealth_ServiceNodes_ConnectProxy_ACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	rules := `
service "foo" {
	policy = "write"
}
service "foo-proxy" {
	policy = "write"
}
node "foo" {
	policy = "write"
}
`
	token := createToken(t, codec, rules)

	{
		var out struct{}

		// Register a service
		args := structs.TestRegisterRequestProxy(t)
		args.WriteRequest.Token = "root"
		args.Service.ID = "foo-proxy-0"
		args.Service.Service = "foo-proxy"
		args.Service.Proxy.DestinationServiceName = "bar"
		args.Check = &structs.HealthCheck{
			Name:      "proxy",
			Status:    api.HealthPassing,
			ServiceID: args.Service.ID,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a service
		args = structs.TestRegisterRequestProxy(t)
		args.WriteRequest.Token = "root"
		args.Service.Service = "foo-proxy"
		args.Service.Proxy.DestinationServiceName = "foo"
		args.Check = &structs.HealthCheck{
			Name:      "proxy",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a service
		args = structs.TestRegisterRequestProxy(t)
		args.WriteRequest.Token = "root"
		args.Service.Service = "another-proxy"
		args.Service.Proxy.DestinationServiceName = "foo"
		args.Check = &structs.HealthCheck{
			Name:      "proxy",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))
	}

	// List w/ token. This should disallow because we don't have permission
	// to read "bar"
	req := structs.ServiceSpecificRequest{
		Connect:      true,
		Datacenter:   "dc1",
		ServiceName:  "bar",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	var resp structs.IndexedCheckServiceNodes
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &resp))
	assert.Len(t, resp.Nodes, 0)
	assert.Greater(t, resp.Index, uint64(0))

	// List w/ token. This should work since we're requesting "foo", but should
	// also only contain the proxies with names that adhere to our ACL.
	req = structs.ServiceSpecificRequest{
		Connect:      true,
		Datacenter:   "dc1",
		ServiceName:  "foo",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &resp))
	assert.Len(t, resp.Nodes, 1)
}

func TestHealth_ServiceNodes_Gateway(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	{
		var out struct{}

		// Register a service "api"
		args := structs.TestRegisterRequest(t)
		args.Service.Service = "api"
		args.Check = &structs.HealthCheck{
			Name:      "api",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a proxy for api
		args = structs.TestRegisterRequestProxy(t)
		args.Service.Service = "api-proxy"
		args.Service.Proxy.DestinationServiceName = "api"
		args.Check = &structs.HealthCheck{
			Name:      "api-proxy",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a service "web"
		args = structs.TestRegisterRequest(t)
		args.Check = &structs.HealthCheck{
			Name:      "web",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a proxy for web
		args = structs.TestRegisterRequestProxy(t)
		args.Check = &structs.HealthCheck{
			Name:      "proxy",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a gateway for web
		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				Service: "gateway",
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "gateway",
				Status:    api.HealthPassing,
				ServiceID: args.Service.Service,
			},
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		entryArgs := &structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "gateway",
				Services: []structs.LinkedService{
					{
						Name: "web",
					},
				},
			},
		}
		var entryResp bool
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &entryArgs, &entryResp))
	}

	retry.Run(t, func(r *retry.R) {
		// List should return both the terminating-gateway and the connect-proxy associated with web
		req := structs.ServiceSpecificRequest{
			Connect:     true,
			Datacenter:  "dc1",
			ServiceName: "web",
		}
		var resp structs.IndexedCheckServiceNodes
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &resp))
		assert.Len(r, resp.Nodes, 2)

		// Check sidecar
		assert.Equal(r, structs.ServiceKindConnectProxy, resp.Nodes[0].Service.Kind)
		assert.Equal(r, "foo", resp.Nodes[0].Node.Node)
		assert.Equal(r, "web-proxy", resp.Nodes[0].Service.Service)
		assert.Equal(r, "web-proxy", resp.Nodes[0].Service.ID)
		assert.Equal(r, "web", resp.Nodes[0].Service.Proxy.DestinationServiceName)
		assert.Equal(r, 2222, resp.Nodes[0].Service.Port)

		// Check gateway
		assert.Equal(r, structs.ServiceKindTerminatingGateway, resp.Nodes[1].Service.Kind)
		assert.Equal(r, "foo", resp.Nodes[1].Node.Node)
		assert.Equal(r, "gateway", resp.Nodes[1].Service.Service)
		assert.Equal(r, "gateway", resp.Nodes[1].Service.ID)
		assert.Equal(r, 443, resp.Nodes[1].Service.Port)
	})
}
func TestHealth_ServiceNodes_Ingress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "ingress-gateway",
			Service: "ingress-gateway",
			Kind:    structs.ServiceKindIngressGateway,
		},
		Check: &structs.HealthCheck{
			Name:      "ingress connect",
			Status:    api.HealthPassing,
			ServiceID: "ingress-gateway",
		},
	}
	var out struct{}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

	arg = structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			ID:      "ingress-gateway",
			Service: "ingress-gateway",
			Kind:    structs.ServiceKindIngressGateway,
		},
		Check: &structs.HealthCheck{
			Name:      "ingress connect",
			Status:    api.HealthWarning,
			ServiceID: "ingress-gateway",
		},
	}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

	// Register ingress-gateway config entry
	{
		args := &structs.IngressGatewayConfigEntry{
			Name: "ingress-gateway",
			Kind: structs.IngressGateway,
			Listeners: []structs.IngressListener{
				{
					Port: 8888,
					Services: []structs.IngressService{
						{Name: "db"},
					},
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var out bool
		require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
		require.True(t, out)
	}

	var out2 structs.IndexedCheckServiceNodes
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		Ingress:     true,
	}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2))

	nodes := out2.Nodes
	require.Len(t, nodes, 2)
	require.Equal(t, nodes[0].Node.Node, "bar")
	require.Equal(t, nodes[0].Checks[0].Status, api.HealthWarning)
	require.Equal(t, nodes[1].Node.Node, "foo")
	require.Equal(t, nodes[1].Checks[0].Status, api.HealthPassing)
}

func TestHealth_ServiceNodes_Ingress_ACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	// Create the ACL.
	token, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
  service "db" { policy = "read" }
	service "ingress-gateway" { policy = "read" }
	node_prefix "" { policy = "read" }`)
	require.NoError(t, err)

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "ingress-gateway",
			Service: "ingress-gateway",
		},
		Check: &structs.HealthCheck{
			Name:      "ingress connect",
			Status:    api.HealthPassing,
			ServiceID: "ingress-gateway",
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out struct{}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

	arg = structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			ID:      "ingress-gateway",
			Service: "ingress-gateway",
		},
		Check: &structs.HealthCheck{
			Name:      "ingress connect",
			Status:    api.HealthWarning,
			ServiceID: "ingress-gateway",
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

	// Register proxy-defaults with 'http' protocol
	{
		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
		require.True(t, out)
	}

	// Register ingress-gateway config entry
	{
		args := &structs.IngressGatewayConfigEntry{
			Name: "ingress-gateway",
			Kind: structs.IngressGateway,
			Listeners: []structs.IngressListener{
				{
					Port:     8888,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "db"},
						{Name: "another"},
					},
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:           structs.ConfigEntryUpsert,
			Datacenter:   "dc1",
			Entry:        args,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
		require.True(t, out)
	}

	// No token used
	var out2 structs.IndexedCheckServiceNodes
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		Ingress:     true,
	}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2))
	require.Len(t, out2.Nodes, 0)

	// Requesting a service that is not covered by the token's policy
	req = structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "another",
		Ingress:      true,
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2))
	require.Len(t, out2.Nodes, 0)

	// Requesting service covered by the token's policy
	req = structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "db",
		Ingress:      true,
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2))

	nodes := out2.Nodes
	require.Len(t, nodes, 2)
	require.Equal(t, nodes[0].Node.Node, "bar")
	require.Equal(t, nodes[0].Checks[0].Status, api.HealthWarning)
	require.Equal(t, nodes[1].Node.Node, "foo")
	require.Equal(t, nodes[1].Checks[0].Status, api.HealthPassing)
}

func TestHealth_NodeChecks_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()

	opt := structs.NodeSpecificRequest{
		Datacenter:   "dc1",
		Node:         srv.config.NodeName,
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply := structs.IndexedHealthChecks{}
	err := msgpackrpc.CallWithCodec(codec, "Health.NodeChecks", &opt, &reply)
	require.NoError(t, err)

	found := false
	for _, chk := range reply.HealthChecks {
		switch chk.ServiceName {
		case "foo":
			found = true
		case "bar":
			t.Fatalf("bad: %#v", reply.HealthChecks)
		}
	}
	require.True(t, found, "bad: %#v", reply.HealthChecks)
	require.True(t, reply.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestHealth_ServiceChecks_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()

	opt := structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "foo",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply := structs.IndexedHealthChecks{}
	err := msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &opt, &reply)
	require.NoError(t, err)

	found := false
	for _, chk := range reply.HealthChecks {
		if chk.ServiceName == "foo" {
			found = true
			break
		}
	}
	require.True(t, found, "bad: %#v", reply.HealthChecks)

	opt.ServiceName = "bar"
	reply = structs.IndexedHealthChecks{}
	err = msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &opt, &reply)
	require.NoError(t, err)
	require.Empty(t, reply.HealthChecks)
	require.True(t, reply.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestHealth_ServiceNodes_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()

	opt := structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "foo",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply := structs.IndexedCheckServiceNodes{}
	err := msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &opt, &reply)
	require.NoError(t, err)
	require.Len(t, reply.Nodes, 1)

	opt.ServiceName = "bar"
	reply = structs.IndexedCheckServiceNodes{}
	err = msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &opt, &reply)
	require.NoError(t, err)
	require.Empty(t, reply.Nodes)
	require.True(t, reply.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestHealth_ChecksInState_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()

	opt := structs.ChecksInStateRequest{
		Datacenter:   "dc1",
		State:        api.HealthPassing,
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply := structs.IndexedHealthChecks{}
	err := msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &opt, &reply)
	require.NoError(t, err)

	found := false
	for _, chk := range reply.HealthChecks {
		switch chk.ServiceName {
		case "foo":
			found = true
		case "bar":
			t.Fatalf("bad service 'bar': %#v", reply.HealthChecks)
		}
	}
	require.True(t, found, "missing service 'foo': %#v", reply.HealthChecks)
	require.True(t, reply.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestHealth_RPC_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// prep the cluster with some data we can use in our filters
	registerTestCatalogEntries(t, codec)

	// Run the tests against the test server

	t.Run("NodeChecks", func(t *testing.T) {
		args := structs.NodeSpecificRequest{
			Datacenter:   "dc1",
			Node:         "foo",
			QueryOptions: structs.QueryOptions{Filter: "ServiceName == redis and v1 in ServiceTags"},
		}

		out := new(structs.IndexedHealthChecks)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.NodeChecks", &args, out))
		require.Len(t, out.HealthChecks, 1)
		require.Equal(t, types.CheckID("foo:redisV1"), out.HealthChecks[0].CheckID)

		args.Filter = "ServiceID == ``"
		out = new(structs.IndexedHealthChecks)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.NodeChecks", &args, out))
		require.Len(t, out.HealthChecks, 2)
	})

	t.Run("ServiceChecks", func(t *testing.T) {
		args := structs.ServiceSpecificRequest{
			Datacenter:   "dc1",
			ServiceName:  "redis",
			QueryOptions: structs.QueryOptions{Filter: "Node == foo"},
		}

		out := new(structs.IndexedHealthChecks)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &args, out))
		// 1 service check for each instance
		require.Len(t, out.HealthChecks, 2)

		args.Filter = "Node == bar"
		out = new(structs.IndexedHealthChecks)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &args, out))
		// 1 service check for each instance
		require.Len(t, out.HealthChecks, 1)

		args.Filter = "Node == foo and v1 in ServiceTags"
		out = new(structs.IndexedHealthChecks)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &args, out))
		// 1 service check for the matching instance
		require.Len(t, out.HealthChecks, 1)
	})

	t.Run("ServiceNodes", func(t *testing.T) {
		args := structs.ServiceSpecificRequest{
			Datacenter:   "dc1",
			ServiceName:  "redis",
			QueryOptions: structs.QueryOptions{Filter: "Service.Meta.version == 2"},
		}

		out := new(structs.IndexedCheckServiceNodes)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &args, out))
		require.Len(t, out.Nodes, 1)

		args.ServiceName = "web"
		args.Filter = "Node.Meta.os == linux"
		out = new(structs.IndexedCheckServiceNodes)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &args, out))
		require.Len(t, out.Nodes, 2)
		require.Equal(t, "baz", out.Nodes[0].Node.Node)
		require.Equal(t, "baz", out.Nodes[1].Node.Node)

		args.Filter = "Node.Meta.os == linux and Service.Meta.version == 1"
		out = new(structs.IndexedCheckServiceNodes)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &args, out))
		require.Len(t, out.Nodes, 1)
	})

	t.Run("ChecksInState", func(t *testing.T) {
		args := structs.ChecksInStateRequest{
			Datacenter:   "dc1",
			State:        api.HealthAny,
			QueryOptions: structs.QueryOptions{Filter: "Node == baz"},
		}

		out := new(structs.IndexedHealthChecks)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &args, out))
		require.Len(t, out.HealthChecks, 6)

		args.Filter = "Status == warning or Status == critical"
		out = new(structs.IndexedHealthChecks)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &args, out))
		require.Len(t, out.HealthChecks, 2)

		args.State = api.HealthCritical
		args.Filter = "Node == baz"
		out = new(structs.IndexedHealthChecks)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &args, out))
		require.Len(t, out.HealthChecks, 1)

		args.State = api.HealthWarning
		out = new(structs.IndexedHealthChecks)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &args, out))
		require.Len(t, out.HealthChecks, 1)
	})
}
