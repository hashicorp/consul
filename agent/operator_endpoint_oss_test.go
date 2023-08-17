// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestOperator_Usage(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	req, err := http.NewRequest("GET", "/v1/operator/usage", nil)
	require.NoError(t, err)

	// Register a few services
	require.NoError(t, upsertTestService(a.RPC, "", "dc1", "web", "test-node", "", func(svc *structs.NodeService) {
		svc.ID = "web1"
	}))
	require.NoError(t, upsertTestService(a.RPC, "", "dc1", "web", "test-node", "", func(svc *structs.NodeService) {
		svc.ID = "web2"
	}))
	require.NoError(t, upsertTestService(a.RPC, "", "dc1", "db", "test-node", ""))
	require.NoError(t, upsertTestService(a.RPC, "", "dc1", "web-proxy", "test-node", "", func(svc *structs.NodeService) {
		svc.Kind = structs.ServiceKindConnectProxy
		svc.Proxy = structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			DestinationServiceID:   "web1",
		}
	}))

	// Add connect-native service to check that we include it in the billable service instances
	require.NoError(t, upsertTestService(a.RPC, "", "dc1", "connect-native-app", "test-node", "", func(svc *structs.NodeService) {
		svc.Connect.Native = true
	}))

	raw, err := a.srv.OperatorUsage(httptest.NewRecorder(), req)
	require.NoError(t, err)

	expected := map[string]structs.ServiceUsage{
		"dc1": {
			Services:         5,
			ServiceInstances: 6,
			ConnectServiceInstances: map[string]int{
				"api-gateway":         0,
				"connect-native":      1,
				"connect-proxy":       1,
				"ingress-gateway":     0,
				"mesh-gateway":        0,
				"terminating-gateway": 0,
			},
			// 4 = 6 total service instances - 1 connect proxy - 1 consul service
			BillableServiceInstances: 4,
			Nodes:                    2,
		},
	}
	require.Equal(t, expected, raw.(structs.Usage).Usage)
}

func upsertTestService(rpc rpcFn, secret, datacenter, name, node, partition string, modifyFuncs ...func(*structs.NodeService)) error {
	req := structs.RegisterRequest{
		Datacenter:     datacenter,
		Node:           node,
		SkipNodeUpdate: true,
		Service: &structs.NodeService{
			ID:      name,
			Service: name,
			Port:    8080,
		},
		WriteRequest: structs.WriteRequest{Token: secret},
	}

	for _, modify := range modifyFuncs {
		modify(req.Service)
	}

	var out struct{}
	return rpc(context.Background(), "Catalog.Register", &req, &out)
}
