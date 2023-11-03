// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestUIIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	// Make a test dir to serve UI files
	uiDir := testutil.TempDir(t, "consul")

	// Make the server
	a := NewTestAgent(t, `
		ui_config {
			dir = "`+uiDir+`"
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Create file
	path := filepath.Join(a.Config.UIConfig.Dir, "my-file")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Request the custom file
	req, _ := http.NewRequest("GET", "/ui/my-file", nil)
	req.URL.Scheme = "http"
	req.URL.Host = a.HTTPAddr()

	// Make the request
	client := cleanhttp.DefaultClient()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response
	if resp.StatusCode != 200 {
		t.Fatalf("bad: %v", resp)
	}

	// Verify the body
	out := bytes.NewBuffer(nil)
	io.Copy(out, resp.Body)
	if out.String() != "test" {
		t.Fatalf("bad: %s", out.Bytes())
	}
}

func TestUINodes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := StartTestAgent(t, TestAgent{HCL: ``, Overrides: `peering = { test_allow_peer_registrations = true }`})
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := []*structs.RegisterRequest{
		{
			Datacenter: "dc1",
			Node:       "test",
			Address:    "127.0.0.1",
		},
		{
			Datacenter: "dc1",
			Node:       "foo-peer",
			Address:    "127.0.0.3",
			PeerName:   "peer1",
		},
	}

	for _, reg := range args {
		var out struct{}
		err := a.RPC(context.Background(), "Catalog.Register", reg, &out)
		require.NoError(t, err)
	}

	// establish "peer1"
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		peerOne := &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				Name:                "peer1",
				State:               pbpeering.PeeringState_ESTABLISHING,
				PeerCAPems:          nil,
				PeerServerName:      "fooservername",
				PeerServerAddresses: []string{"addr1"},
			},
		}
		_, err := a.rpcClientPeering.PeeringWrite(ctx, peerOne)
		require.NoError(t, err)
	}

	req, _ := http.NewRequest("GET", "/v1/internal/ui/nodes/dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UINodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be 3 nodes, and all the empty lists should be non-nil
	nodes := obj.(structs.NodeDump)
	require.Len(t, nodes, 3)

	// check local nodes, services and checks
	require.Equal(t, a.Config.NodeName, nodes[0].Node)
	require.NotNil(t, nodes[0].Services)
	require.Len(t, nodes[0].Services, 1)
	require.NotNil(t, nodes[0].Checks)
	require.Len(t, nodes[0].Checks, 1)
	require.Equal(t, "test", nodes[1].Node)
	require.NotNil(t, nodes[1].Services)
	require.Len(t, nodes[1].Services, 0)
	require.NotNil(t, nodes[1].Checks)
	require.Len(t, nodes[1].Checks, 0)

	// peered node
	require.Equal(t, "foo-peer", nodes[2].Node)
	require.Equal(t, "peer1", nodes[2].PeerName)
	require.NotNil(t, nodes[2].Services)
	require.Len(t, nodes[2].Services, 0)
	require.NotNil(t, nodes[1].Checks)
	require.Len(t, nodes[2].Services, 0)

	// check for consul-version in node meta
	require.Equal(t, nodes[0].Meta[structs.MetaConsulVersion], a.Config.Version)
}

func TestUINodes_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "test",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"os": "linux",
		},
	}

	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "test2",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"os": "macos",
		},
	}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", "/v1/internal/ui/nodes/dc1?filter="+url.QueryEscape("Meta.os == linux"), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UINodes(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	// Should be 2 nodes, and all the empty lists should be non-nil
	nodes := obj.(structs.NodeDump)
	require.Len(t, nodes, 1)
	require.Equal(t, nodes[0].Node, "test")
	require.Empty(t, nodes[0].Services)
	require.Empty(t, nodes[0].Checks)
}

func TestUINodeInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/internal/ui/node/%s", a.Config.NodeName), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UINodeInfo(resp, req)
	require.NoError(t, err)
	require.Equal(t, resp.Code, http.StatusOK)
	assertIndex(t, resp)

	// Should be 1 node for the server
	node := obj.(*structs.NodeInfo)
	if node.Node != a.Config.NodeName {
		t.Fatalf("bad: %v", node)
	}

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "test",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ = http.NewRequest("GET", "/v1/internal/ui/node/test", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.UINodeInfo(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	// Should be non-nil empty lists for services and checks
	node = obj.(*structs.NodeInfo)
	if node.Node != "test" ||
		node.Services == nil || len(node.Services) != 0 ||
		node.Checks == nil || len(node.Checks) != 0 {
		t.Fatalf("bad: %v", node)
	}

	// check for consul-version in node meta
	require.Equal(t, node.Meta[structs.MetaConsulVersion], a.Config.Version)
}

func TestUIServices(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := StartTestAgent(t, TestAgent{HCL: ``, Overrides: `peering = { test_allow_peer_registrations = true }`})
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	requests := []*structs.RegisterRequest{
		// register foo node
		{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:   "foo",
					Name:   "node check",
					Status: api.HealthPassing,
				},
			},
		},
		// register api service on node foo
		{
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				Service: "api",
				ID:      "api-1",
				Tags:    []string{"tag1", "tag2"},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					Name:        "api svc check",
					ServiceName: "api",
					ServiceID:   "api-1",
					Status:      api.HealthWarning,
				},
			},
		},
		// register api-proxy svc on node foo
		{
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "api-proxy",
				ID:      "api-proxy-1",
				Tags:    []string{},
				Meta:    map[string]string{structs.MetaExternalSource: "k8s"},
				Port:    1234,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "api",
				},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					Name:        "api proxy listening",
					ServiceName: "api-proxy",
					ServiceID:   "api-proxy-1",
					Status:      api.HealthPassing,
				},
			},
		},
		// register bar node with service web
		{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				Service: "web",
				ID:      "web-1",
				Tags:    []string{},
				Meta:    map[string]string{structs.MetaExternalSource: "k8s"},
				Port:    1234,
			},
			Checks: []*structs.HealthCheck{
				{
					Node:        "bar",
					Name:        "web svc check",
					Status:      api.HealthCritical,
					ServiceName: "web",
					ServiceID:   "web-1",
				},
			},
		},
		// register zip node with service cache
		{
			Datacenter: "dc1",
			Node:       "zip",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				Service: "cache",
				Tags:    []string{},
			},
		},
		// register peer node foo with peer service
		{
			Datacenter: "dc1",
			Node:       "foo",
			ID:         types.NodeID("e0155642-135d-4739-9853-a1ee6c9f945b"),
			Address:    "127.0.0.2",
			TaggedAddresses: map[string]string{
				"lan": "127.0.0.2",
				"wan": "198.18.0.2",
			},
			NodeMeta: map[string]string{
				"env": "production",
				"os":  "linux",
			},
			PeerName: "peer1",
			Service: &structs.NodeService{
				Kind:     structs.ServiceKindTypical,
				ID:       "serviceID",
				Service:  "service",
				Port:     1235,
				Address:  "198.18.1.2",
				PeerName: "peer1",
			},
		},
	}

	for _, args := range requests {
		var out struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// establish "peer1"
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		peerOne := &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				Name:                "peer1",
				State:               pbpeering.PeeringState_ESTABLISHING,
				PeerCAPems:          nil,
				PeerServerName:      "fooservername",
				PeerServerAddresses: []string{"addr1"},
			},
		}
		_, err := a.rpcClientPeering.PeeringWrite(ctx, peerOne)
		require.NoError(t, err)
	}

	// Register a terminating gateway associated with api and cache
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
			},
		}
		var regOutput struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &arg, &regOutput))

		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{
					Name: "api",
				},
				{
					Name: "cache",
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)

		// Web should not show up as ConnectedWithGateway since this one does not have any instances
		args = &structs.TerminatingGatewayConfigEntry{
			Name: "other-terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{
					Name: "web",
				},
			},
		}

		req = structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	t.Run("No Filter", func(t *testing.T) {
		t.Parallel()
		req, _ := http.NewRequest("GET", "/v1/internal/ui/services/dc1", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.UIServices(resp, req)
		require.NoError(t, err)
		assertIndex(t, resp)

		// Should be 2 nodes, and all the empty lists should be non-nil
		summary := obj.([]*ServiceListingSummary)
		require.Len(t, summary, 7)

		// internal accounting that users don't see can be blown away
		for _, sum := range summary {
			sum.transparentProxySet = false
			sum.externalSourceSet = nil
			sum.checks = nil
		}

		expected := []*ServiceListingSummary{
			{
				ServiceSummary: ServiceSummary{
					Kind:           structs.ServiceKindTypical,
					Name:           "api",
					Datacenter:     "dc1",
					Tags:           []string{"tag1", "tag2"},
					Nodes:          []string{"foo"},
					InstanceCount:  1,
					ChecksPassing:  2,
					ChecksWarning:  1,
					ChecksCritical: 0,
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				ConnectedWithProxy:   true,
				ConnectedWithGateway: true,
			},
			{
				ServiceSummary: ServiceSummary{
					Kind:            structs.ServiceKindConnectProxy,
					Name:            "api-proxy",
					Datacenter:      "dc1",
					Tags:            nil,
					Nodes:           []string{"foo"},
					InstanceCount:   1,
					ChecksPassing:   2,
					ChecksWarning:   0,
					ChecksCritical:  0,
					ExternalSources: []string{"k8s"},
					EnterpriseMeta:  *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			{
				ServiceSummary: ServiceSummary{
					Kind:           structs.ServiceKindTypical,
					Name:           "cache",
					Datacenter:     "dc1",
					Tags:           nil,
					Nodes:          []string{"zip"},
					InstanceCount:  1,
					ChecksPassing:  0,
					ChecksWarning:  0,
					ChecksCritical: 0,
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				ConnectedWithGateway: true,
			},
			{
				ServiceSummary: ServiceSummary{
					Kind:           structs.ServiceKindTypical,
					Name:           "consul",
					Datacenter:     "dc1",
					Tags:           nil,
					Nodes:          []string{a.Config.NodeName},
					InstanceCount:  1,
					ChecksPassing:  1,
					ChecksWarning:  0,
					ChecksCritical: 0,
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			{
				ServiceSummary: ServiceSummary{
					Kind:           structs.ServiceKindTerminatingGateway,
					Name:           "terminating-gateway",
					Datacenter:     "dc1",
					Tags:           nil,
					Nodes:          []string{"foo"},
					InstanceCount:  1,
					ChecksPassing:  1,
					ChecksWarning:  0,
					ChecksCritical: 0,
					GatewayConfig:  GatewayConfig{AssociatedServiceCount: 2},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			{
				ServiceSummary: ServiceSummary{
					Kind:            structs.ServiceKindTypical,
					Name:            "web",
					Datacenter:      "dc1",
					Tags:            nil,
					Nodes:           []string{"bar"},
					InstanceCount:   1,
					ChecksPassing:   0,
					ChecksWarning:   0,
					ChecksCritical:  1,
					ExternalSources: []string{"k8s"},
					EnterpriseMeta:  *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			{
				ServiceSummary: ServiceSummary{
					Kind:           structs.ServiceKindTypical,
					Name:           "service",
					Datacenter:     "dc1",
					Tags:           nil,
					Nodes:          []string{"foo"},
					InstanceCount:  1,
					ChecksPassing:  0,
					ChecksWarning:  0,
					ChecksCritical: 0,
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					PeerName:       "peer1",
				},
			},
		}
		require.ElementsMatch(t, expected, summary)
	})

	t.Run("Filtered", func(t *testing.T) {
		filterQuery := url.QueryEscape("Service.Service == web or Service.Service == api")
		req, _ := http.NewRequest("GET", "/v1/internal/ui/services?filter="+filterQuery, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.UIServices(resp, req)
		require.NoError(t, err)
		assertIndex(t, resp)

		// Should be 2 nodes, and all the empty lists should be non-nil
		summary := obj.([]*ServiceListingSummary)
		require.Len(t, summary, 2)

		// internal accounting that users don't see can be blown away
		for _, sum := range summary {
			sum.externalSourceSet = nil
			sum.checks = nil
		}

		expected := []*ServiceListingSummary{
			{
				ServiceSummary: ServiceSummary{
					Kind:           structs.ServiceKindTypical,
					Name:           "api",
					Datacenter:     "dc1",
					Tags:           []string{"tag1", "tag2"},
					Nodes:          []string{"foo"},
					InstanceCount:  1,
					ChecksPassing:  1,
					ChecksWarning:  1,
					ChecksCritical: 0,
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				ConnectedWithProxy:   false,
				ConnectedWithGateway: false,
			},
			{
				ServiceSummary: ServiceSummary{
					Kind:            structs.ServiceKindTypical,
					Name:            "web",
					Datacenter:      "dc1",
					Tags:            nil,
					Nodes:           []string{"bar"},
					InstanceCount:   1,
					ChecksPassing:   0,
					ChecksWarning:   0,
					ChecksCritical:  1,
					ExternalSources: []string{"k8s"},
					EnterpriseMeta:  *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		}
		require.ElementsMatch(t, expected, summary)
	})
	t.Run("Filtered without results", func(t *testing.T) {
		filterQuery := url.QueryEscape("Service.Service == absent")
		req, _ := http.NewRequest("GET", "/v1/internal/ui/services?filter="+filterQuery, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.UIServices(resp, req)
		require.NoError(t, err)
		assertIndex(t, resp)

		// Ensure the ServiceSummary doesn't output a `null` response when there
		// are no matching summaries
		require.NotNil(t, obj)

		summary := obj.([]*ServiceListingSummary)
		require.Len(t, summary, 0)
	})
}

func TestUIExportedServices(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := StartTestAgent(t, TestAgent{Overrides: `peering = { test_allow_peer_registrations = true }`})
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	requests := []*structs.RegisterRequest{
		// register api service
		{
			Datacenter:     "dc1",
			Node:           "node",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				Service: "api",
				ID:      "api-1",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "node",
					Name:        "api svc check",
					ServiceName: "api",
					ServiceID:   "api-1",
					Status:      api.HealthWarning,
				},
			},
		},
		// register api-proxy svc
		{
			Datacenter:     "dc1",
			Node:           "node",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "api-proxy",
				ID:      "api-proxy-1",
				Tags:    []string{},
				Meta:    map[string]string{structs.MetaExternalSource: "k8s"},
				Port:    1234,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "api",
				},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "node",
					Name:        "api proxy listening",
					ServiceName: "api-proxy",
					ServiceID:   "api-proxy-1",
					Status:      api.HealthPassing,
				},
			},
		},
		// register service web
		{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				Service: "web",
				ID:      "web-1",
				Tags:    []string{},
				Meta:    map[string]string{structs.MetaExternalSource: "k8s"},
				Port:    1234,
			},
			Checks: []*structs.HealthCheck{
				{
					Node:        "bar",
					Name:        "web svc check",
					Status:      api.HealthCritical,
					ServiceName: "web",
					ServiceID:   "web-1",
				},
			},
		},
	}

	for _, args := range requests {
		var out struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// establish "peer1"
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req := &pbpeering.GenerateTokenRequest{
			PeerName: "peer1",
		}
		_, err := a.rpcClientPeering.GenerateToken(ctx, req)
		require.NoError(t, err)
	}

	{
		// Register exported services
		args := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "api",
					Consumers: []structs.ServiceConsumer{
						{
							Peer: "peer1",
						},
					},
				},
			},
		}
		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	t.Run("valid peer", func(t *testing.T) {
		t.Parallel()
		req, _ := http.NewRequest("GET", "/v1/internal/ui/exported-services?peer=peer1", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		decoder := json.NewDecoder(resp.Body)
		var summary []*ServiceListingSummary
		require.NoError(t, decoder.Decode(&summary))
		assertIndex(t, resp)

		require.Len(t, summary, 1)

		// internal accounting that users don't see can be blown away
		for _, sum := range summary {
			sum.transparentProxySet = false
			sum.externalSourceSet = nil
			sum.checks = nil
		}

		expected := []*ServiceListingSummary{
			{
				ServiceSummary: ServiceSummary{
					Kind:           structs.ServiceKindTypical,
					Name:           "api",
					Datacenter:     "dc1",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		}
		require.Equal(t, expected, summary)
	})

	t.Run("invalid peer", func(t *testing.T) {
		t.Parallel()
		req, _ := http.NewRequest("GET", "/v1/internal/ui/exported-services?peer=peer2", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		decoder := json.NewDecoder(resp.Body)
		var summary []*ServiceListingSummary
		require.NoError(t, decoder.Decode(&summary))
		assertIndex(t, resp)

		require.Len(t, summary, 0)
	})
}

func TestUIGatewayServiceNodes_Terminating(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Register terminating gateway and a service that will be associated with it
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "terminating connect",
				Status:    api.HealthPassing,
				ServiceID: "terminating-gateway",
			},
		}
		var regOutput struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &arg, &regOutput))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
				Tags:    []string{"primary"},
			},
			Check: &structs.HealthCheck{
				Name:      "db-warning",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
		}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &arg, &regOutput))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
				Tags:    []string{"backup"},
			},
			Check: &structs.HealthCheck{
				Name:      "db2-passing",
				Status:    api.HealthPassing,
				ServiceID: "db2",
			},
		}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &arg, &regOutput))
	}

	{
		// Request without having registered the config-entry, shouldn't respond with null
		req, _ := http.NewRequest("GET", "/v1/internal/ui/gateway-services-nodes/terminating-gateway", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.UIGatewayServicesNodes(resp, req)
		require.Nil(t, err)
		require.NotNil(t, obj)
	}

	{
		// Register terminating-gateway config entry, linking it to db and redis (does not exist)
		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{
					Name: "db",
				},
				{
					Name:     "redis",
					CAFile:   "/etc/certs/ca.pem",
					CertFile: "/etc/certs/cert.pem",
					KeyFile:  "/etc/certs/key.pem",
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	// Request
	req, _ := http.NewRequest("GET", "/v1/internal/ui/gateway-services-nodes/terminating-gateway", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UIGatewayServicesNodes(resp, req)
	require.Nil(t, err)
	assertIndex(t, resp)

	summary := obj.([]*ServiceSummary)

	// internal accounting that users don't see can be blown away
	for _, sum := range summary {
		sum.externalSourceSet = nil
		sum.checks = nil
	}

	expect := []*ServiceSummary{
		{
			Name:           "redis",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			Name:           "db",
			Datacenter:     "dc1",
			Tags:           []string{"backup", "primary"},
			Nodes:          []string{"bar", "baz"},
			InstanceCount:  2,
			ChecksPassing:  1,
			ChecksWarning:  1,
			ChecksCritical: 0,
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}
	require.ElementsMatch(t, expect, summary)
}

func TestUIGatewayServiceNodes_Ingress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, `alt_domain = "alt.consul."`)
	defer a.Shutdown()

	// Register ingress gateway and a service that will be associated with it
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "ingress-gateway",
				Service: "ingress-gateway",
				Kind:    structs.ServiceKindIngressGateway,
				Port:    8443,
			},
			Check: &structs.HealthCheck{
				Name:      "ingress connect",
				Status:    api.HealthPassing,
				ServiceID: "ingress-gateway",
			},
		}
		var regOutput struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &arg, &regOutput))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
				Tags:    []string{"primary"},
			},
			Check: &structs.HealthCheck{
				Name:      "db-warning",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
		}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &arg, &regOutput))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
				Tags:    []string{"backup"},
			},
			Check: &structs.HealthCheck{
				Name:      "db2-passing",
				Status:    api.HealthPassing,
				ServiceID: "db2",
			},
		}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &arg, &regOutput))

		// Set web protocol to http
		svcDefaultsReq := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Name:     "web",
				Protocol: "http",
			},
		}
		var configOutput bool
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &svcDefaultsReq, &configOutput))
		require.True(t, configOutput)

		// Register ingress-gateway config entry, linking it to db and redis (does not exist)
		args := &structs.IngressGatewayConfigEntry{
			Name: "ingress-gateway",
			Kind: structs.IngressGateway,
			Listeners: []structs.IngressListener{
				{
					Port:     8888,
					Protocol: "tcp",
					Services: []structs.IngressService{
						{
							Name: "db",
						},
					},
				},
				{
					Port:     8080,
					Protocol: "http",
					Services: []structs.IngressService{
						{
							Name: "web",
						},
					},
				},
				{
					Port:     8081,
					Protocol: "http",
					Services: []structs.IngressService{
						{
							Name:  "web",
							Hosts: []string{"*.test.example.com"},
						},
					},
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	// Request
	req, _ := http.NewRequest("GET", "/v1/internal/ui/gateway-services-nodes/ingress-gateway", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UIGatewayServicesNodes(resp, req)
	require.Nil(t, err)
	assertIndex(t, resp)

	// Construct expected addresses so that differences between CE/Ent are
	// handled by code. We specifically don't include the trailing DNS . here as
	// we are constructing what we are expecting, not the actual value
	webDNS := serviceIngressDNSName("web", "dc1", "consul", structs.DefaultEnterpriseMetaInDefaultPartition())
	webDNSAlt := serviceIngressDNSName("web", "dc1", "alt.consul", structs.DefaultEnterpriseMetaInDefaultPartition())
	dbDNS := serviceIngressDNSName("db", "dc1", "consul", structs.DefaultEnterpriseMetaInDefaultPartition())
	dbDNSAlt := serviceIngressDNSName("db", "dc1", "alt.consul", structs.DefaultEnterpriseMetaInDefaultPartition())

	dump := obj.([]*ServiceSummary)
	expect := []*ServiceSummary{
		{
			Name: "web",
			GatewayConfig: GatewayConfig{
				Addresses: []string{
					fmt.Sprintf("%s:8080", webDNS),
					fmt.Sprintf("%s:8080", webDNSAlt),
					"*.test.example.com:8081",
				},
			},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			Name:           "db",
			Datacenter:     "dc1",
			Tags:           []string{"backup", "primary"},
			Nodes:          []string{"bar", "baz"},
			InstanceCount:  2,
			ChecksPassing:  1,
			ChecksWarning:  1,
			ChecksCritical: 0,
			GatewayConfig: GatewayConfig{
				Addresses: []string{
					fmt.Sprintf("%s:8888", dbDNS),
					fmt.Sprintf("%s:8888", dbDNSAlt),
				},
			},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}

	// internal accounting that users don't see can be blown away
	for _, sum := range dump {
		sum.GatewayConfig.addressesSet = nil
		sum.checks = nil
	}
	require.ElementsMatch(t, expect, dump)
}

func TestUIGatewayIntentions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForServiceIntentions(t, a.RPC, "dc1")

	// Register terminating gateway and config entry linking it to postgres + redis
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "terminating connect",
				Status:    api.HealthPassing,
				ServiceID: "terminating-gateway",
			},
		}
		var regOutput struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &arg, &regOutput))

		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{
					Name: "postgres",
				},
				{
					Name:     "redis",
					CAFile:   "/etc/certs/ca.pem",
					CertFile: "/etc/certs/cert.pem",
					KeyFile:  "/etc/certs/key.pem",
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	// create some symmetric intentions to ensure we are only matching on destination
	{
		for _, v := range []string{"*", "mysql", "redis", "postgres"} {
			req := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			req.Intention.SourceName = "api"
			req.Intention.DestinationName = v

			var reply string
			require.NoError(t, a.RPC(context.Background(), "Intention.Apply", &req, &reply))

			req = structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			req.Intention.SourceName = v
			req.Intention.DestinationName = "api"
			require.NoError(t, a.RPC(context.Background(), "Intention.Apply", &req, &reply))
		}
	}

	// Request intentions matching the gateway named "terminating-gateway"
	req, _ := http.NewRequest("GET", "/v1/internal/ui/gateway-intentions/terminating-gateway", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UIGatewayIntentions(resp, req)
	require.Nil(t, err)
	assertIndex(t, resp)

	intentions := obj.(structs.Intentions)
	require.Len(t, intentions, 3)

	// Only intentions with linked services as a destination should be returned, and wildcard matches should be deduped
	expected := []string{"postgres", "*", "redis"}
	actual := []string{
		intentions[0].DestinationName,
		intentions[1].DestinationName,
		intentions[2].DestinationName,
	}
	require.ElementsMatch(t, expected, actual)
}

func TestUIEndpoint_modifySummaryForGatewayService_UseRequestedDCInsteadOfConfigured(t *testing.T) {
	dc := "dc2"
	cfg := config.RuntimeConfig{Datacenter: "dc1", DNSDomain: "consul"}
	sum := ServiceSummary{GatewayConfig: GatewayConfig{}}
	gwsvc := structs.GatewayService{Service: structs.ServiceName{Name: "test"}, Port: 42}
	modifySummaryForGatewayService(&cfg, dc, &sum, &gwsvc)
	expected := serviceCanonicalDNSName("test", "ingress", "dc2", "consul", nil) + ":42"
	require.Equal(t, expected, sum.GatewayConfig.Addresses[0])
}

func TestUIServiceTopology(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Register ingress -> api -> web -> redis
	{
		registrations := map[string]*structs.RegisterRequest{
			"Node edge": {
				Datacenter: "dc1",
				Node:       "edge",
				Address:    "127.0.0.20",
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:    "edge",
						CheckID: "edge:alive",
						Name:    "edge-liveness",
						Status:  api.HealthPassing,
					},
				},
			},
			"Ingress gateway on edge": {
				Datacenter:     "dc1",
				Node:           "edge",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindIngressGateway,
					ID:      "ingress",
					Service: "ingress",
					Port:    443,
					Address: "198.18.1.20",
				},
			},
			"Node foo": {
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.2",
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:    "foo",
						CheckID: "foo:alive",
						Name:    "foo-liveness",
						Status:  api.HealthPassing,
					},
				},
			},
			"Service api on foo": {
				Datacenter:     "dc1",
				Node:           "foo",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "api",
					Service: "api",
					Port:    9090,
					Address: "198.18.1.2",
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "foo",
						CheckID:     "foo:api",
						Name:        "api-liveness",
						Status:      api.HealthPassing,
						ServiceID:   "api",
						ServiceName: "api",
					},
				},
			},
			"Service api-proxy": {
				Datacenter:     "dc1",
				Node:           "foo",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "api-proxy",
					Service: "api-proxy",
					Port:    8443,
					Address: "198.18.1.2",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "api",
						Mode:                   structs.ProxyModeTransparent,
					},
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "foo",
						CheckID:     "foo:api-proxy",
						Name:        "api proxy listening",
						Status:      api.HealthPassing,
						ServiceID:   "api-proxy",
						ServiceName: "api-proxy",
					},
				},
			},
			"Node bar": {
				Datacenter: "dc1",
				Node:       "bar",
				Address:    "127.0.0.3",
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:    "bar",
						CheckID: "bar:alive",
						Name:    "bar-liveness",
						Status:  api.HealthPassing,
					},
				},
			},
			"Service web on bar": {
				Datacenter:     "dc1",
				Node:           "bar",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "web",
					Service: "web",
					Port:    80,
					Address: "198.18.1.20",
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "bar",
						CheckID:     "bar:web",
						Name:        "web-liveness",
						Status:      api.HealthWarning,
						ServiceID:   "web",
						ServiceName: "web",
					},
				},
			},
			"Service web-proxy on bar": {
				Datacenter:     "dc1",
				Node:           "bar",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "web-proxy",
					Service: "web-proxy",
					Port:    8443,
					Address: "198.18.1.20",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "web",
						Upstreams: structs.Upstreams{
							{
								DestinationName: "redis",
								LocalBindPort:   123,
							},
						},
					},
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "bar",
						CheckID:     "bar:web-proxy",
						Name:        "web proxy listening",
						Status:      api.HealthCritical,
						ServiceID:   "web-proxy",
						ServiceName: "web-proxy",
					},
				},
			},
			"Node baz": {
				Datacenter: "dc1",
				Node:       "baz",
				Address:    "127.0.0.4",
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:    "baz",
						CheckID: "baz:alive",
						Name:    "baz-liveness",
						Status:  api.HealthPassing,
					},
				},
			},
			"Service web on baz": {
				Datacenter:     "dc1",
				Node:           "baz",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "web",
					Service: "web",
					Port:    80,
					Address: "198.18.1.40",
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "baz",
						CheckID:     "baz:web",
						Name:        "web-liveness",
						Status:      api.HealthPassing,
						ServiceID:   "web",
						ServiceName: "web",
					},
				},
			},
			"Service web-proxy on baz": {
				Datacenter:     "dc1",
				Node:           "baz",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "web-proxy",
					Service: "web-proxy",
					Port:    8443,
					Address: "198.18.1.40",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "web",
						Mode:                   structs.ProxyModeTransparent,
					},
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "baz",
						CheckID:     "baz:web-proxy",
						Name:        "web proxy listening",
						Status:      api.HealthCritical,
						ServiceID:   "web-proxy",
						ServiceName: "web-proxy",
					},
				},
			},
			"Node zip": {
				Datacenter: "dc1",
				Node:       "zip",
				Address:    "127.0.0.5",
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:    "zip",
						CheckID: "zip:alive",
						Name:    "zip-liveness",
						Status:  api.HealthPassing,
					},
				},
			},
			"Service redis on zip": {
				Datacenter:     "dc1",
				Node:           "zip",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "redis",
					Service: "redis",
					Port:    6379,
					Address: "198.18.1.60",
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "zip",
						CheckID:     "zip:redis",
						Name:        "redis-liveness",
						Status:      api.HealthPassing,
						ServiceID:   "redis",
						ServiceName: "redis",
					},
				},
			},
			"Service redis-proxy on zip": {
				Datacenter:     "dc1",
				Node:           "zip",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "redis-proxy",
					Service: "redis-proxy",
					Port:    8443,
					Address: "198.18.1.60",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "redis",
					},
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "zip",
						CheckID:     "zip:redis-proxy",
						Name:        "redis proxy listening",
						Status:      api.HealthCritical,
						ServiceID:   "redis-proxy",
						ServiceName: "redis-proxy",
					},
				},
			},
			"Node cnative": {
				Datacenter: "dc1",
				Node:       "cnative",
				Address:    "127.0.0.6",
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:    "cnative",
						CheckID: "cnative:alive",
						Name:    "cnative-liveness",
						Status:  api.HealthPassing,
					},
				},
			},
			"Service cbackend on cnative": {
				Datacenter:     "dc1",
				Node:           "cnative",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "cbackend",
					Service: "cbackend",
					Port:    8080,
					Address: "198.18.1.70",
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "cnative",
						CheckID:     "cnative:cbackend",
						Name:        "cbackend-liveness",
						Status:      api.HealthPassing,
						ServiceID:   "cbackend",
						ServiceName: "cbackend",
					},
				},
			},
			"Service cbackend-proxy on cnative": {
				Datacenter:     "dc1",
				Node:           "cnative",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "cbackend-proxy",
					Service: "cbackend-proxy",
					Port:    8443,
					Address: "198.18.1.70",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "cbackend",
					},
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "cnative",
						CheckID:     "cnative:cbackend-proxy",
						Name:        "cbackend proxy listening",
						Status:      api.HealthCritical,
						ServiceID:   "cbackend-proxy",
						ServiceName: "cbackend-proxy",
					},
				},
			},
			"Service cfrontend on cnative": {
				Datacenter:     "dc1",
				Node:           "cnative",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "cfrontend",
					Service: "cfrontend",
					Port:    9080,
					Address: "198.18.1.70",
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "cnative",
						CheckID:     "cnative:cfrontend",
						Name:        "cfrontend-liveness",
						Status:      api.HealthPassing,
						ServiceID:   "cfrontend",
						ServiceName: "cfrontend",
					},
				},
			},
			"Service cfrontend-proxy on cnative": {
				Datacenter:     "dc1",
				Node:           "cnative",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "cfrontend-proxy",
					Service: "cfrontend-proxy",
					Port:    9443,
					Address: "198.18.1.70",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "cfrontend",
						Upstreams: structs.Upstreams{
							{
								DestinationName: "cproxy",
								LocalBindPort:   123,
							},
						},
					},
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "cnative",
						CheckID:     "cnative:cfrontend-proxy",
						Name:        "cfrontend proxy listening",
						Status:      api.HealthCritical,
						ServiceID:   "cfrontend-proxy",
						ServiceName: "cfrontend-proxy",
					},
				},
			},
			"Service cproxy on cnative": {
				Datacenter:     "dc1",
				Node:           "cnative",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "cproxy-https",
					Service: "cproxy",
					Port:    1111,
					Address: "198.18.1.70",
					Tags:    []string{"https"},
					Connect: structs.ServiceConnect{Native: true},
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "cnative",
						CheckID:     "cnative:cproxy-https",
						Name:        "cproxy-liveness",
						Status:      api.HealthPassing,
						ServiceID:   "cproxy-https",
						ServiceName: "cproxy",
					},
				},
			},
			"Service cproxy/http on cnative": {
				Datacenter:     "dc1",
				Node:           "cnative",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "cproxy-http",
					Service: "cproxy",
					Port:    1112,
					Address: "198.18.1.70",
					Tags:    []string{"http"},
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "cnative",
						CheckID:     "cnative:cproxy-http",
						Name:        "cproxy-liveness",
						Status:      api.HealthPassing,
						ServiceID:   "cproxy-http",
						ServiceName: "cproxy",
					},
				},
			},
		}
		for _, args := range registrations {
			var out struct{}
			require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
		}
	}

	// ingress -> api gateway config entry (but no intention)
	// wildcard deny intention
	// api -> web exact intention
	// web -> redis exact intention
	// cfrontend -> cproxy exact intention
	// cproxy -> cbackend exact intention
	{
		entries := []structs.ConfigEntryRequest{
			{
				Datacenter: "dc1",
				Entry: &structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
			},
			{
				Datacenter: "dc1",
				Entry: &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "api",
					Protocol: "tcp",
				},
			},
			{
				Datacenter: "dc1",
				Entry: &structs.ServiceIntentionsConfigEntry{
					Kind: structs.ServiceIntentions,
					Name: "*",
					Meta: map[string]string{structs.MetaExternalSource: "nomad"},
					Sources: []*structs.SourceIntention{
						{
							Name:   "*",
							Action: structs.IntentionActionDeny,
						},
					},
				},
			},
			{
				Datacenter: "dc1",
				Entry: &structs.ServiceIntentionsConfigEntry{
					Kind: structs.ServiceIntentions,
					Name: "redis",
					Sources: []*structs.SourceIntention{
						{
							Name: "web",
							Permissions: []*structs.IntentionPermission{
								{
									Action: structs.IntentionActionAllow,
									HTTP: &structs.IntentionHTTPPermission{
										Methods: []string{"GET"},
									},
								},
							},
						},
					},
				},
			},
			{
				Datacenter: "dc1",
				Entry: &structs.ServiceIntentionsConfigEntry{
					Kind: structs.ServiceIntentions,
					Name: "web",
					Sources: []*structs.SourceIntention{
						{
							Action: structs.IntentionActionAllow,
							Name:   "api",
						},
					},
				},
			},
			{
				Datacenter: "dc1",
				Entry: &structs.ServiceIntentionsConfigEntry{
					Kind: structs.ServiceIntentions,
					Name: "api",
					Sources: []*structs.SourceIntention{
						{
							Name:   "ingress",
							Action: structs.IntentionActionAllow,
						},
					},
				},
			},
			{
				Datacenter: "dc1",
				Entry: &structs.IngressGatewayConfigEntry{
					Kind: "ingress-gateway",
					Name: "ingress",
					Listeners: []structs.IngressListener{
						{
							Port:     1111,
							Protocol: "tcp",
							Services: []structs.IngressService{
								{
									Name:           "api",
									EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
								},
							},
						},
					},
				},
			},
			{
				Datacenter: "dc1",
				Entry: &structs.ServiceIntentionsConfigEntry{
					Kind: structs.ServiceIntentions,
					Name: "cproxy",
					Sources: []*structs.SourceIntention{
						{
							Action: structs.IntentionActionAllow,
							Name:   "cfrontend",
						},
					},
				},
			},
			{
				Datacenter: "dc1",
				Entry: &structs.ServiceIntentionsConfigEntry{
					Kind: structs.ServiceIntentions,
					Name: "cbackend",
					Sources: []*structs.SourceIntention{
						{
							Action: structs.IntentionActionAllow,
							Name:   "cproxy",
						},
					},
				},
			},
		}
		for _, req := range entries {
			out := false
			require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &req, &out))
		}
	}

	type testCase struct {
		name    string
		httpReq *http.Request
		want    *ServiceTopology
		wantErr string
	}

	run := func(t *testing.T, tc testCase) {
		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			obj, err := a.srv.UIServiceTopology(resp, tc.httpReq)

			if tc.wantErr != "" {
				assert.NotNil(r, err)
				assert.Nil(r, tc.want) // should not define a non-nil want
				require.Contains(r, err.Error(), tc.wantErr)
				require.Nil(r, obj)
				return
			}
			assert.Nil(r, err)

			require.NoError(r, checkIndex(resp))
			require.NotNil(r, obj)
			result := obj.(ServiceTopology)
			clearUnexportedFields(result)

			require.Equal(r, *tc.want, result)
		})
	}

	tcs := []testCase{
		{
			name: "request without kind",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/ingress", nil)
				return req
			}(),
			wantErr: "Missing service kind",
		},
		{
			name: "request with unsupported kind",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/ingress?kind=not-a-kind", nil)
				return req
			}(),
			wantErr: `Unsupported service kind "not-a-kind"`,
		},
		{
			name: "ingress",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/ingress?kind=ingress-gateway", nil)
				return req
			}(),
			want: &ServiceTopology{
				Protocol:         "tcp",
				TransparentProxy: false,
				Upstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:             "api",
							Datacenter:       "dc1",
							Nodes:            []string{"foo"},
							InstanceCount:    1,
							ChecksPassing:    3,
							EnterpriseMeta:   *structs.DefaultEnterpriseMetaInDefaultPartition(),
							TransparentProxy: true,
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow:   true,
							Allowed:        true,
							HasPermissions: false,
							HasExact:       true,
						},
						Source: structs.TopologySourceRegistration,
					},
				},
				Downstreams:    []*ServiceTopologySummary{},
				FilteredByACLs: false,
			},
		},
		{
			name: "api",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/api?kind=", nil)
				return req
			}(),
			want: &ServiceTopology{
				Protocol:         "tcp",
				TransparentProxy: true,
				Downstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:             "ingress",
							Kind:             structs.ServiceKindIngressGateway,
							Datacenter:       "dc1",
							Nodes:            []string{"edge"},
							InstanceCount:    1,
							ChecksPassing:    1,
							EnterpriseMeta:   *structs.DefaultEnterpriseMetaInDefaultPartition(),
							TransparentProxy: false,
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow:   true,
							Allowed:        true,
							HasPermissions: false,
							HasExact:       true,
						},
						Source: structs.TopologySourceRegistration,
					},
				},
				Upstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:             "web",
							Datacenter:       "dc1",
							Nodes:            []string{"bar", "baz"},
							InstanceCount:    2,
							ChecksPassing:    3,
							ChecksWarning:    1,
							ChecksCritical:   2,
							EnterpriseMeta:   *structs.DefaultEnterpriseMetaInDefaultPartition(),
							TransparentProxy: false,
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow:   true,
							Allowed:        true,
							HasPermissions: false,
							HasExact:       true,
						},
						Source: structs.TopologySourceSpecificIntention,
					},
				},
				FilteredByACLs: false,
			},
		},
		{
			name: "web",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/web?kind=", nil)
				return req
			}(),
			want: &ServiceTopology{
				Protocol:         "http",
				TransparentProxy: false,
				Upstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:             "redis",
							Datacenter:       "dc1",
							Nodes:            []string{"zip"},
							InstanceCount:    1,
							ChecksPassing:    2,
							ChecksCritical:   1,
							EnterpriseMeta:   *structs.DefaultEnterpriseMetaInDefaultPartition(),
							TransparentProxy: false,
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow:   true,
							Allowed:        false,
							HasPermissions: true,
							HasExact:       true,
						},
						Source: structs.TopologySourceRegistration,
					},
				},
				Downstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:             "api",
							Datacenter:       "dc1",
							Nodes:            []string{"foo"},
							InstanceCount:    1,
							ChecksPassing:    3,
							EnterpriseMeta:   *structs.DefaultEnterpriseMetaInDefaultPartition(),
							TransparentProxy: true,
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow:   true,
							Allowed:        true,
							HasPermissions: false,
							HasExact:       true,
						},
						Source: structs.TopologySourceSpecificIntention,
					},
				},
				FilteredByACLs: false,
			},
		},
		{
			name: "redis",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/redis?kind=", nil)
				return req
			}(),
			want: &ServiceTopology{
				Protocol:         "http",
				TransparentProxy: false,
				Upstreams:        []*ServiceTopologySummary{},
				Downstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:           "web",
							Datacenter:     "dc1",
							Nodes:          []string{"bar", "baz"},
							InstanceCount:  2,
							ChecksPassing:  3,
							ChecksWarning:  1,
							ChecksCritical: 2,
							EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow:   true,
							Allowed:        false,
							HasPermissions: true,
							HasExact:       true,
						},
						Source: structs.TopologySourceRegistration,
					},
				},
				FilteredByACLs: false,
			},
		},
		{
			name: "cproxy",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/cproxy?kind=", nil)
				return req
			}(),
			want: &ServiceTopology{
				Protocol:         "http",
				TransparentProxy: false,
				Upstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:           "cbackend",
							Datacenter:     "dc1",
							Nodes:          []string{"cnative"},
							InstanceCount:  1,
							ChecksPassing:  2,
							ChecksWarning:  0,
							ChecksCritical: 1,
							EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow:   true,
							Allowed:        true,
							HasPermissions: false,
							HasExact:       true,
						},
						Source: structs.TopologySourceSpecificIntention,
					},
				},
				Downstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:           "cfrontend",
							Datacenter:     "dc1",
							Nodes:          []string{"cnative"},
							InstanceCount:  1,
							ChecksPassing:  2,
							ChecksWarning:  0,
							ChecksCritical: 1,
							EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow:   true,
							Allowed:        true,
							HasPermissions: false,
							HasExact:       true,
						},
						Source: structs.TopologySourceRegistration,
					},
				},
				FilteredByACLs: false,
			},
		},
		{
			name: "cbackend",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/cbackend?kind=", nil)
				return req
			}(),
			want: &ServiceTopology{
				Protocol:         "http",
				TransparentProxy: false,
				Upstreams:        []*ServiceTopologySummary{},
				Downstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:           "cproxy",
							Datacenter:     "dc1",
							Tags:           []string{"http", "https"},
							Nodes:          []string{"cnative"},
							InstanceCount:  2,
							ChecksPassing:  3,
							ChecksWarning:  0,
							ChecksCritical: 0,
							ConnectNative:  true,
							EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow:   true,
							Allowed:        true,
							HasPermissions: false,
							HasExact:       true,
						},
						Source: structs.TopologySourceSpecificIntention,
					},
				},
				FilteredByACLs: false,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

// clearUnexportedFields sets unexported members of ServiceTopology to their
// type defaults, since the fields are not marshalled in the JSON response.
func clearUnexportedFields(result ServiceTopology) {
	for _, u := range result.Upstreams {
		u.transparentProxySet = false
		u.externalSourceSet = nil
		u.checks = nil
	}
	for _, d := range result.Downstreams {
		d.transparentProxySet = false
		d.externalSourceSet = nil
		d.checks = nil
	}
}

func TestUIServiceTopology_RoutingConfigs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Register dashboard -> routing-config -> { counting, counting-v2 }
	{
		registrations := map[string]*structs.RegisterRequest{
			"Service dashboard": {
				Datacenter:     "dc1",
				Node:           "foo",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "dashboard",
					Service: "dashboard",
					Port:    9002,
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "foo",
						CheckID:     "foo:dashboard",
						Status:      api.HealthPassing,
						ServiceID:   "dashboard",
						ServiceName: "dashboard",
					},
				},
			},
			"Service dashboard-proxy": {
				Datacenter:     "dc1",
				Node:           "foo",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "dashboard-sidecar-proxy",
					Service: "dashboard-sidecar-proxy",
					Port:    5000,
					Address: "198.18.1.0",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "dashboard",
						DestinationServiceID:   "dashboard",
						LocalServiceAddress:    "127.0.0.1",
						LocalServicePort:       9002,
						Upstreams: []structs.Upstream{
							{
								DestinationType: "service",
								DestinationName: "routing-config",
								LocalBindPort:   5000,
							},
						},
					},
					LocallyRegisteredAsSidecar: true,
				},
			},
			"Service counting": {
				Datacenter:     "dc1",
				Node:           "foo",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "counting",
					Service: "counting",
					Port:    9003,
					Address: "198.18.1.1",
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "foo",
						CheckID:     "foo:api",
						Status:      api.HealthPassing,
						ServiceID:   "counting",
						ServiceName: "counting",
					},
				},
			},
			"Service counting-proxy": {
				Datacenter:     "dc1",
				Node:           "foo",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "counting-proxy",
					Service: "counting-proxy",
					Port:    5001,
					Address: "198.18.1.1",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "counting",
					},
					LocallyRegisteredAsSidecar: true,
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "foo",
						CheckID:     "foo:counting-proxy",
						Status:      api.HealthPassing,
						ServiceID:   "counting-proxy",
						ServiceName: "counting-proxy",
					},
				},
			},
			"Service counting-v2": {
				Datacenter:     "dc1",
				Node:           "foo",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindTypical,
					ID:      "counting-v2",
					Service: "counting-v2",
					Port:    9004,
					Address: "198.18.1.2",
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "foo",
						CheckID:     "foo:api",
						Status:      api.HealthPassing,
						ServiceID:   "counting-v2",
						ServiceName: "counting-v2",
					},
				},
			},
			"Service counting-v2-proxy": {
				Datacenter:     "dc1",
				Node:           "foo",
				SkipNodeUpdate: true,
				Service: &structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "counting-v2-proxy",
					Service: "counting-v2-proxy",
					Port:    5002,
					Address: "198.18.1.2",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "counting-v2",
					},
					LocallyRegisteredAsSidecar: true,
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "foo",
						CheckID:     "foo:counting-v2-proxy",
						Status:      api.HealthPassing,
						ServiceID:   "counting-v2-proxy",
						ServiceName: "counting-v2-proxy",
					},
				},
			},
		}
		for _, args := range registrations {
			var out struct{}
			require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
		}
	}
	{
		entries := []structs.ConfigEntryRequest{
			{
				Datacenter: "dc1",
				Entry: &structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
			},
			{
				Datacenter: "dc1",
				Entry: &structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "routing-config",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathPrefix: "/v2",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "counting-v2",
							},
						},
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathPrefix: "/",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "counting",
							},
						},
					},
				},
			},
		}
		for _, req := range entries {
			out := false
			require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &req, &out))
		}
	}

	type testCase struct {
		name    string
		httpReq *http.Request
		want    *ServiceTopology
		wantErr string
	}

	run := func(t *testing.T, tc testCase) {
		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			obj, err := a.srv.UIServiceTopology(resp, tc.httpReq)
			assert.Nil(r, err)

			if tc.wantErr != "" {
				assert.Nil(r, tc.want) // should not define a non-nil want
				require.Equal(r, tc.wantErr, resp.Body.String())
				require.Nil(r, obj)
				return
			}

			require.NoError(r, checkIndex(resp))
			require.NotNil(r, obj)
			result := obj.(ServiceTopology)
			clearUnexportedFields(result)

			require.Equal(r, *tc.want, result)
		})
	}

	tcs := []testCase{
		{
			name: "dashboard has upstream routing-config",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/dashboard?kind=", nil)
				return req
			}(),
			want: &ServiceTopology{
				Protocol:    "http",
				Downstreams: []*ServiceTopologySummary{},
				Upstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:             "routing-config",
							Datacenter:       "dc1",
							EnterpriseMeta:   *structs.DefaultEnterpriseMetaInDefaultPartition(),
							TransparentProxy: false,
						},
						Intention: structs.IntentionDecisionSummary{
							DefaultAllow: true,
							Allowed:      true,
						},
						Source: structs.TopologySourceRoutingConfig,
					},
				},
			},
		},
		{
			name: "counting has downstream dashboard",
			httpReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/counting?kind=", nil)
				return req
			}(),
			want: &ServiceTopology{
				Protocol:  "http",
				Upstreams: []*ServiceTopologySummary{},
				Downstreams: []*ServiceTopologySummary{
					{
						ServiceSummary: ServiceSummary{
							Name:           "dashboard",
							Datacenter:     "dc1",
							Nodes:          []string{"foo"},
							EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
							InstanceCount:  1,
							ChecksPassing:  1,
						},
						Source: "proxy-registration",
						Intention: structs.IntentionDecisionSummary{
							Allowed:      true,
							DefaultAllow: true,
						},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestUIEndpoint_MetricsProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	var lastHeadersSent atomic.Value

	backendH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastHeadersSent.Store(r.Header)
		if r.URL.Path == "/some/prefix/ok" {
			w.Header().Set("X-Random-Header", "Foo")
			w.Write([]byte("OK"))
			return
		}
		if r.URL.Path == "/some/prefix/query-echo" {
			w.Write([]byte("RawQuery: " + r.URL.RawQuery))
			return
		}
		if r.URL.Path == "/.passwd" {
			w.Write([]byte("SECRETS!"))
			return
		}
		http.Error(w, "not found on backend", http.StatusNotFound)
	})

	backend := httptest.NewServer(backendH)
	defer backend.Close()

	backendURL := backend.URL + "/some/prefix"

	// Share one agent for all these test cases. This has a few nice side-effects:
	//  1. it's cheaper
	//  2. it implicitly tests that config reloading works between cases
	//
	// Note we can't test the case where UI is disabled though as that's not
	// reloadable so we'll do that in a separate test below rather than have many
	// new tests all with a new agent. response headers also aren't reloadable
	// currently due to the way we wrap API endpoints at startup.
	a := NewTestAgent(t, `
		ui_config {
			enabled = true
		}
		http_config {
			response_headers {
				"Access-Control-Allow-Origin" = "*"
			}
		}
	`)
	defer a.Shutdown()

	endpointPath := "/v1/internal/ui/metrics-proxy"

	cases := []struct {
		name            string
		config          config.UIMetricsProxy
		path            string
		wantCode        int
		wantContains    string
		wantHeaders     map[string]string
		wantHeadersSent map[string]string
	}{
		{
			name:     "disabled",
			config:   config.UIMetricsProxy{},
			path:     endpointPath + "/ok",
			wantCode: http.StatusNotFound,
		},
		{
			name: "blocked path",
			config: config.UIMetricsProxy{
				BaseURL:       backendURL,
				PathAllowlist: []string{"/some/other-prefix/ok"},
			},
			path:     endpointPath + "/ok",
			wantCode: http.StatusForbidden,
		},
		{
			name: "allowed path",
			config: config.UIMetricsProxy{
				BaseURL:       backendURL,
				PathAllowlist: []string{"/some/prefix/ok"},
			},
			path:         endpointPath + "/ok",
			wantCode:     http.StatusOK,
			wantContains: "OK",
		},
		{
			name: "basic proxying",
			config: config.UIMetricsProxy{
				BaseURL: backendURL,
			},
			path:         endpointPath + "/ok",
			wantCode:     http.StatusOK,
			wantContains: "OK",
			wantHeaders: map[string]string{
				"X-Random-Header": "Foo",
			},
		},
		{
			name: "404 on backend",
			config: config.UIMetricsProxy{
				BaseURL: backendURL,
			},
			path:         endpointPath + "/random-path",
			wantCode:     http.StatusNotFound,
			wantContains: "not found on backend",
		},
		{
			// Note that this case actually doesn't exercise our validation logic at
			// all since the top level API mux resolves this to /v1/internal/.passwd
			// and it never hits our handler at all. I left it in though as this
			// wasn't obvious and it's worth knowing if we change something in our mux
			// that might affect path traversal opportunity here. In fact this makes
			// our path traversal handling somewhat redundant because any traversal
			// that goes "back" far enough to traverse up from the BaseURL of the
			// proxy target will in fact miss our handler entirely. It's still better
			// to be safe than sorry though.
			name: "path traversal should fail - api mux",
			config: config.UIMetricsProxy{
				BaseURL: backendURL,
			},
			path:         endpointPath + "/../../.passwd",
			wantCode:     http.StatusMovedPermanently,
			wantContains: "Moved Permanently",
		},
		{
			name: "adding auth header",
			config: config.UIMetricsProxy{
				BaseURL: backendURL,
				AddHeaders: []config.UIMetricsProxyAddHeader{
					{
						Name:  "Authorization",
						Value: "SECRET_KEY",
					},
					{
						Name:  "X-Some-Other-Header",
						Value: "foo",
					},
				},
			},
			path:         endpointPath + "/ok",
			wantCode:     http.StatusOK,
			wantContains: "OK",
			wantHeaders: map[string]string{
				"X-Random-Header": "Foo",
			},
			wantHeadersSent: map[string]string{
				"X-Some-Other-Header": "foo",
				"Authorization":       "SECRET_KEY",
			},
		},
		{
			name: "passes through query params",
			config: config.UIMetricsProxy{
				BaseURL: backendURL,
			},
			// encoded=test[0]&&test[1]==!@£$%^
			path:         endpointPath + "/query-echo?foo=bar&encoded=test%5B0%5D%26%26test%5B1%5D%3D%3D%21%40%C2%A3%24%25%5E",
			wantCode:     http.StatusOK,
			wantContains: "RawQuery: foo=bar&encoded=test%5B0%5D%26%26test%5B1%5D%3D%3D%21%40%C2%A3%24%25%5E",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Reload the agent config with the desired UI config by making a copy and
			// using internal reload.
			cfg := *a.Agent.config

			// Modify the UIConfig part (this is a copy remember and that struct is
			// not a pointer)
			cfg.UIConfig.MetricsProxy = tc.config

			require.NoError(t, a.Agent.reloadConfigInternal(&cfg))

			// Now fetch the API handler to run requests against
			a.enableDebug.Store(true)

			h := a.srv.handler()

			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			require.Equal(t, tc.wantCode, rec.Code,
				"Wrong status code. Body = %s", rec.Body.String())
			require.Contains(t, rec.Body.String(), tc.wantContains)
			for k, v := range tc.wantHeaders {
				// Headers are a slice of values, just assert that one of the values is
				// the one we want.
				require.Contains(t, rec.Result().Header[k], v)
			}
			if len(tc.wantHeadersSent) > 0 {
				headersSent, ok := lastHeadersSent.Load().(http.Header)
				require.True(t, ok, "backend not called")
				for k, v := range tc.wantHeadersSent {
					require.Contains(t, headersSent[k], v,
						"header %s doesn't have the right value set", k)
				}
			}
		})
	}
}
