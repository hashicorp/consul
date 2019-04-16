package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth_ChecksInState(t *testing.T) {
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
			Tags:    []string{"master"},
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

	arg = structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tags:    []string{"slave"},
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    api.HealthWarning,
			ServiceID: "db",
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out2 structs.IndexedCheckServiceNodes
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceTags: []string{"master"},
		TagFilter:   false,
	}
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	nodes := out2.Nodes
	if len(nodes) != 2 {
		t.Fatalf("Bad: %v", nodes)
	}
	if nodes[0].Node.Node != "bar" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[1].Node.Node != "foo" {
		t.Fatalf("Bad: %v", nodes[1])
	}
	if !lib.StrContains(nodes[0].Service.Tags, "slave") {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if !lib.StrContains(nodes[1].Service.Tags, "master") {
		t.Fatalf("Bad: %v", nodes[1])
	}
	if nodes[0].Checks[0].Status != api.HealthWarning {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[1].Checks[0].Status != api.HealthPassing {
		t.Fatalf("Bad: %v", nodes[1])
	}

	// Same should still work for <1.3 RPCs with singular tags
	// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
	// with 1.2.x is not required.
	{
		var out2 structs.IndexedCheckServiceNodes
		req := structs.ServiceSpecificRequest{
			Datacenter:  "dc1",
			ServiceName: "db",
			ServiceTag:  "master",
			TagFilter:   false,
		}
		if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2); err != nil {
			t.Fatalf("err: %v", err)
		}

		nodes := out2.Nodes
		if len(nodes) != 2 {
			t.Fatalf("Bad: %v", nodes)
		}
		if nodes[0].Node.Node != "bar" {
			t.Fatalf("Bad: %v", nodes[0])
		}
		if nodes[1].Node.Node != "foo" {
			t.Fatalf("Bad: %v", nodes[1])
		}
		if !lib.StrContains(nodes[0].Service.Tags, "slave") {
			t.Fatalf("Bad: %v", nodes[0])
		}
		if !lib.StrContains(nodes[1].Service.Tags, "master") {
			t.Fatalf("Bad: %v", nodes[1])
		}
		if nodes[0].Checks[0].Status != api.HealthWarning {
			t.Fatalf("Bad: %v", nodes[0])
		}
		if nodes[1].Checks[0].Status != api.HealthPassing {
			t.Fatalf("Bad: %v", nodes[1])
		}
	}
}

func TestHealth_ServiceNodes_MultipleServiceTags(t *testing.T) {
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
			Tags:    []string{"master", "v2"},
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
			Tags:    []string{"slave", "v2"},
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
		ServiceTags: []string{"master", "v2"},
		TagFilter:   true,
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &out2))

	nodes := out2.Nodes
	require.Len(t, nodes, 1)
	require.Equal(t, nodes[0].Node.Node, "foo")
	require.Contains(t, nodes[0].Service.Tags, "v2")
	require.Contains(t, nodes[0].Service.Tags, "master")
	require.Equal(t, nodes[0].Checks[0].Status, api.HealthPassing)
}

func TestHealth_ServiceNodes_NodeMetaFilter(t *testing.T) {
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
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "write"
}
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var token string
	assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", arg, &token))

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
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

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
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

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
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))
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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &resp))
	assert.Len(resp.Nodes, 0)

	// List w/ token. This should work since we're requesting "foo", but should
	// also only contain the proxies with names that adhere to our ACL.
	req = structs.ServiceSpecificRequest{
		Connect:      true,
		Datacenter:   "dc1",
		ServiceName:  "foo",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &resp))
	assert.Len(resp.Nodes, 1)
}

func TestHealth_NodeChecks_FilterACL(t *testing.T) {
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
	if err := msgpackrpc.CallWithCodec(codec, "Health.NodeChecks", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	found := false
	for _, chk := range reply.HealthChecks {
		switch chk.ServiceName {
		case "foo":
			found = true
		case "bar":
			t.Fatalf("bad: %#v", reply.HealthChecks)
		}
	}
	if !found {
		t.Fatalf("bad: %#v", reply.HealthChecks)
	}

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestHealth_ServiceChecks_FilterACL(t *testing.T) {
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
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	found := false
	for _, chk := range reply.HealthChecks {
		if chk.ServiceName == "foo" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("bad: %#v", reply.HealthChecks)
	}

	opt.ServiceName = "bar"
	reply = structs.IndexedHealthChecks{}
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceChecks", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(reply.HealthChecks) != 0 {
		t.Fatalf("bad: %#v", reply.HealthChecks)
	}

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestHealth_ServiceNodes_FilterACL(t *testing.T) {
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
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(reply.Nodes) != 1 {
		t.Fatalf("bad: %#v", reply.Nodes)
	}

	opt.ServiceName = "bar"
	reply = structs.IndexedCheckServiceNodes{}
	if err := msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(reply.Nodes) != 0 {
		t.Fatalf("bad: %#v", reply.Nodes)
	}

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestHealth_ChecksInState_FilterACL(t *testing.T) {
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
	if err := msgpackrpc.CallWithCodec(codec, "Health.ChecksInState", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}

	found := false
	for _, chk := range reply.HealthChecks {
		switch chk.ServiceName {
		case "foo":
			found = true
		case "bar":
			t.Fatalf("bad service 'bar': %#v", reply.HealthChecks)
		}
	}
	if !found {
		t.Fatalf("missing service 'foo': %#v", reply.HealthChecks)
	}

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestHealth_RPC_Filter(t *testing.T) {
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
