// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestMultiportAdapter_RPCBackwardCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir, server := testServer(t)
	defer os.RemoveAll(dir)
	defer server.Shutdown()
	codec := rpcClient(t, server)
	defer codec.Close()

	testrpc.WaitForLeader(t, server.RPC, "dc1")

	ports := structs.ServicePorts{
		{Name: "http", Port: 8080, Default: true},
		{Name: "admin", Port: 9090},
	}
	registerReq := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "node-1",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "web-1",
			Service: "web",
			Ports:   ports,
		},
	}
	var registerResp struct{}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &registerReq, &registerResp))

	t.Run("Catalog.ServiceNodes", func(t *testing.T) {
		req := structs.ServiceSpecificRequest{Datacenter: "dc1", ServiceName: "web"}
		var resp structs.IndexedServiceNodes
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
		require.Len(t, resp.ServiceNodes, 1)
		require.Equal(t, 8080, resp.ServiceNodes[0].ServicePort)
		require.Equal(t, ports, resp.ServiceNodes[0].ServicePorts)
	})

	t.Run("Catalog.NodeServices", func(t *testing.T) {
		req := structs.NodeSpecificRequest{Datacenter: "dc1", Node: "node-1"}
		var resp structs.IndexedNodeServices
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &req, &resp))
		require.NotNil(t, resp.NodeServices)
		require.Equal(t, 8080, resp.NodeServices.Services["web-1"].Port)
		require.Equal(t, ports, resp.NodeServices.Services["web-1"].Ports)
	})

	t.Run("Catalog.NodeServiceList", func(t *testing.T) {
		req := structs.NodeSpecificRequest{Datacenter: "dc1", Node: "node-1"}
		var resp structs.IndexedNodeServiceList
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.NodeServiceList", &req, &resp))
		require.Len(t, resp.NodeServices.Services, 1)
		require.Equal(t, 8080, resp.NodeServices.Services[0].Port)
		require.Equal(t, ports, resp.NodeServices.Services[0].Ports)
	})

	t.Run("Health.ServiceNodes", func(t *testing.T) {
		req := structs.ServiceSpecificRequest{Datacenter: "dc1", ServiceName: "web"}
		var resp structs.IndexedCheckServiceNodes
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Health.ServiceNodes", &req, &resp))
		require.Len(t, resp.Nodes, 1)
		require.Equal(t, 8080, resp.Nodes[0].Service.Port)
		require.Equal(t, ports, resp.Nodes[0].Service.Ports)
	})

	query := structs.PreparedQuery{
		Name: "web-query",
		Service: structs.ServiceQuery{
			Service: "web",
		},
	}
	applyReq := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query:      &query,
	}
	var queryID string
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &applyReq, &queryID))

	t.Run("PreparedQuery.Execute", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{Datacenter: "dc1", QueryIDOrName: queryID}
		var resp structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "PreparedQuery.Execute", &req, &resp))
		require.Len(t, resp.Nodes, 1)
		require.Equal(t, 8080, resp.Nodes[0].Service.Port)
		require.Equal(t, ports, resp.Nodes[0].Service.Ports)
	})

	t.Run("PreparedQuery.ExecuteRemote", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRemoteRequest{Datacenter: "dc1", Query: query}
		var resp structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "PreparedQuery.ExecuteRemote", &req, &resp))
		require.Len(t, resp.Nodes, 1)
		require.Equal(t, 8080, resp.Nodes[0].Service.Port)
		require.Equal(t, ports, resp.Nodes[0].Service.Ports)
	})
}
