package agent

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUiIndex(t *testing.T) {
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
	if err := ioutil.WriteFile(path, []byte("test"), 0644); err != nil {
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

func TestUiNodes(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "test",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/internal/ui/nodes/dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UINodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be 2 nodes, and all the empty lists should be non-nil
	nodes := obj.(structs.NodeDump)
	if len(nodes) != 2 ||
		nodes[0].Node != a.Config.NodeName ||
		nodes[0].Services == nil || len(nodes[0].Services) != 1 ||
		nodes[0].Checks == nil || len(nodes[0].Checks) != 1 ||
		nodes[1].Node != "test" ||
		nodes[1].Services == nil || len(nodes[1].Services) != 0 ||
		nodes[1].Checks == nil || len(nodes[1].Checks) != 0 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestUiNodes_Filter(t *testing.T) {
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
	require.NoError(t, a.RPC("Catalog.Register", args, &out))

	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "test2",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"os": "macos",
		},
	}
	require.NoError(t, a.RPC("Catalog.Register", args, &out))

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

func TestUiNodeInfo(t *testing.T) {
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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
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
}

func TestUiServices(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
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
		//register api service on node foo
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
				Meta:    map[string]string{metaExternalSource: "k8s"},
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
				Meta:    map[string]string{metaExternalSource: "k8s"},
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
	}

	for _, args := range requests {
		var out struct{}
		require.NoError(t, a.RPC("Catalog.Register", args, &out))
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
		require.NoError(t, a.RPC("Catalog.Register", &arg, &regOutput))

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
		require.NoError(t, a.RPC("ConfigEntry.Apply", &req, &configOutput))
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
		require.NoError(t, a.RPC("ConfigEntry.Apply", &req, &configOutput))
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
		require.Len(t, summary, 6)

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
					ChecksPassing:  2,
					ChecksWarning:  1,
					ChecksCritical: 0,
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
					EnterpriseMeta:  *structs.DefaultEnterpriseMeta(),
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
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
					EnterpriseMeta:  *structs.DefaultEnterpriseMeta(),
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
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
					EnterpriseMeta:  *structs.DefaultEnterpriseMeta(),
				},
			},
		}
		require.ElementsMatch(t, expected, summary)
	})
}

func TestUIGatewayServiceNodes_Terminating(t *testing.T) {
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
		require.NoError(t, a.RPC("Catalog.Register", &arg, &regOutput))

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
		require.NoError(t, a.RPC("Catalog.Register", &arg, &regOutput))

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
		require.NoError(t, a.RPC("Catalog.Register", &arg, &regOutput))

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
		require.NoError(t, a.RPC("ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	// Request
	req, _ := http.NewRequest("GET", "/v1/internal/ui/gateway-services-nodes/terminating-gateway", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UIGatewayServicesNodes(resp, req)
	assert.Nil(t, err)
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
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
		},
	}
	assert.ElementsMatch(t, expect, summary)
}

func TestUIGatewayServiceNodes_Ingress(t *testing.T) {
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
		require.NoError(t, a.RPC("Catalog.Register", &arg, &regOutput))

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
		require.NoError(t, a.RPC("Catalog.Register", &arg, &regOutput))

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
		require.NoError(t, a.RPC("Catalog.Register", &arg, &regOutput))

		// Set web protocol to http
		svcDefaultsReq := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Name:     "web",
				Protocol: "http",
			},
		}
		var configOutput bool
		require.NoError(t, a.RPC("ConfigEntry.Apply", &svcDefaultsReq, &configOutput))
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
		require.NoError(t, a.RPC("ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	// Request
	req, _ := http.NewRequest("GET", "/v1/internal/ui/gateway-services-nodes/ingress-gateway", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UIGatewayServicesNodes(resp, req)
	assert.Nil(t, err)
	assertIndex(t, resp)

	// Construct expected addresses so that differences between OSS/Ent are handled by code
	webDNS := serviceIngressDNSName("web", "dc1", "consul.", structs.DefaultEnterpriseMeta())
	webDNSAlt := serviceIngressDNSName("web", "dc1", "alt.consul.", structs.DefaultEnterpriseMeta())
	dbDNS := serviceIngressDNSName("db", "dc1", "consul.", structs.DefaultEnterpriseMeta())
	dbDNSAlt := serviceIngressDNSName("db", "dc1", "alt.consul.", structs.DefaultEnterpriseMeta())

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
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
		},
	}

	// internal accounting that users don't see can be blown away
	for _, sum := range dump {
		sum.GatewayConfig.addressesSet = nil
		sum.checks = nil
	}
	assert.ElementsMatch(t, expect, dump)
}

func TestUIGatewayIntentions(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

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
		require.NoError(t, a.RPC("Catalog.Register", &arg, &regOutput))

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
		require.NoError(t, a.RPC("ConfigEntry.Apply", &req, &configOutput))
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
			assert.NoError(t, a.RPC("Intention.Apply", &req, &reply))

			req = structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			req.Intention.SourceName = v
			req.Intention.DestinationName = "api"
			assert.NoError(t, a.RPC("Intention.Apply", &req, &reply))
		}
	}

	// Request intentions matching the gateway named "terminating-gateway"
	req, _ := http.NewRequest("GET", "/v1/internal/ui/gateway-intentions/terminating-gateway", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UIGatewayIntentions(resp, req)
	assert.Nil(t, err)
	assertIndex(t, resp)

	intentions := obj.(structs.Intentions)
	assert.Len(t, intentions, 3)

	// Only intentions with linked services as a destination should be returned, and wildcard matches should be deduped
	expected := []string{"postgres", "*", "redis"}
	actual := []string{
		intentions[0].DestinationName,
		intentions[1].DestinationName,
		intentions[2].DestinationName,
	}
	assert.ElementsMatch(t, expected, actual)
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
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Register terminating gateway and config entry linking it to postgres + redis
	{
		registrations := map[string]*structs.RegisterRequest{
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
						Upstreams: structs.Upstreams{
							{
								DestinationName: "web",
								LocalBindPort:   8080,
							},
						},
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
		}
		for _, args := range registrations {
			var out struct{}
			require.NoError(t, a.RPC("Catalog.Register", args, &out))
		}
	}

	t.Run("api", func(t *testing.T) {
		// Request topology for api
		req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/api", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.UIServiceTopology(resp, req)
		assert.Nil(t, err)
		assertIndex(t, resp)

		expect := ServiceTopology{
			Upstreams: []*ServiceSummary{
				{
					Name:           "web",
					Datacenter:     "dc1",
					Nodes:          []string{"bar", "baz"},
					InstanceCount:  2,
					ChecksPassing:  3,
					ChecksWarning:  1,
					ChecksCritical: 2,
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				},
			},
			FilteredByACLs: false,
		}
		result := obj.(ServiceTopology)

		// Internal accounting that is not returned in JSON response
		for _, u := range result.Upstreams {
			u.externalSourceSet = nil
			u.checks = nil
		}
		require.Equal(t, expect, result)
	})

	t.Run("web", func(t *testing.T) {
		// Request topology for web
		req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/web", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.UIServiceTopology(resp, req)
		assert.Nil(t, err)
		assertIndex(t, resp)

		expect := ServiceTopology{
			Upstreams: []*ServiceSummary{
				{
					Name:           "redis",
					Datacenter:     "dc1",
					Nodes:          []string{"zip"},
					InstanceCount:  1,
					ChecksPassing:  2,
					ChecksCritical: 1,
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				},
			},
			Downstreams: []*ServiceSummary{
				{
					Name:           "api",
					Datacenter:     "dc1",
					Nodes:          []string{"foo"},
					InstanceCount:  1,
					ChecksPassing:  3,
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				},
			},
			FilteredByACLs: false,
		}
		result := obj.(ServiceTopology)

		// Internal accounting that is not returned in JSON response
		for _, u := range result.Upstreams {
			u.externalSourceSet = nil
			u.checks = nil
		}
		for _, d := range result.Downstreams {
			d.externalSourceSet = nil
			d.checks = nil
		}
		require.Equal(t, expect, result)
	})

	t.Run("redis", func(t *testing.T) {
		// Request topology for redis
		req, _ := http.NewRequest("GET", "/v1/internal/ui/service-topology/redis", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.UIServiceTopology(resp, req)
		assert.Nil(t, err)
		assertIndex(t, resp)

		expect := ServiceTopology{
			Downstreams: []*ServiceSummary{
				{
					Name:           "web",
					Datacenter:     "dc1",
					Nodes:          []string{"bar", "baz"},
					InstanceCount:  2,
					ChecksPassing:  3,
					ChecksWarning:  1,
					ChecksCritical: 2,
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				},
			},
			FilteredByACLs: false,
		}
		result := obj.(ServiceTopology)

		// Internal accounting that is not returned in JSON response
		for _, d := range result.Downstreams {
			d.externalSourceSet = nil
			d.checks = nil
		}
		require.Equal(t, expect, result)
	})
}
