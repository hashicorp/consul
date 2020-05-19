package agent

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

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
	defer os.RemoveAll(uiDir)

	// Make the server
	a := NewTestAgent(t, `
		ui_dir = "`+uiDir+`"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Create file
	path := filepath.Join(a.Config.UIDir, "my-file")
	if err := ioutil.WriteFile(path, []byte("test"), 0777); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register node
	req, _ := http.NewRequest("GET", "/ui/my-file", nil)
	req.URL.Scheme = "http"
	req.URL.Host = a.srv.Addr

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
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
		&structs.RegisterRequest{
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
		&structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				Service: "api",
				Tags:    []string{"tag1", "tag2"},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					Name:        "api svc check",
					ServiceName: "api",
					Status:      api.HealthWarning,
				},
			},
		},
		// register web svc on node foo
		&structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "web",
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
					Name:        "web svc check",
					ServiceName: "web",
					Status:      api.HealthPassing,
				},
			},
		},
		// register bar node with service web
		&structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: "web",
				Tags:    []string{},
				Meta:    map[string]string{metaExternalSource: "k8s"},
				Port:    1234,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "api",
				},
			},
			Checks: []*structs.HealthCheck{
				&structs.HealthCheck{
					Node:        "bar",
					Name:        "web svc check",
					Status:      api.HealthCritical,
					ServiceName: "web",
				},
			},
		},
		// register zip node with service cache
		&structs.RegisterRequest{
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

	t.Run("No Filter", func(t *testing.T) {
		t.Parallel()
		req, _ := http.NewRequest("GET", "/v1/internal/ui/services/dc1", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.UIServices(resp, req)
		require.NoError(t, err)
		assertIndex(t, resp)

		// Should be 2 nodes, and all the empty lists should be non-nil
		summary := obj.([]*ServiceSummary)
		require.Len(t, summary, 4)

		// internal accounting that users don't see can be blown away
		for _, sum := range summary {
			sum.externalSourceSet = nil
			sum.proxyForSet = nil
		}

		expected := []*ServiceSummary{
			&ServiceSummary{
				Kind:           structs.ServiceKindTypical,
				Name:           "api",
				Tags:           []string{"tag1", "tag2"},
				Nodes:          []string{"foo"},
				InstanceCount:  1,
				ChecksPassing:  2,
				ChecksWarning:  1,
				ChecksCritical: 0,
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
			},
			&ServiceSummary{
				Kind:           structs.ServiceKindTypical,
				Name:           "cache",
				Tags:           nil,
				Nodes:          []string{"zip"},
				InstanceCount:  1,
				ChecksPassing:  0,
				ChecksWarning:  0,
				ChecksCritical: 0,
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
			},
			&ServiceSummary{
				Kind:            structs.ServiceKindConnectProxy,
				Name:            "web",
				Tags:            nil,
				Nodes:           []string{"bar", "foo"},
				InstanceCount:   2,
				ProxyFor:        []string{"api"},
				ChecksPassing:   2,
				ChecksWarning:   1,
				ChecksCritical:  1,
				ExternalSources: []string{"k8s"},
				EnterpriseMeta:  *structs.DefaultEnterpriseMeta(),
			},
			&ServiceSummary{
				Kind:           structs.ServiceKindTypical,
				Name:           "consul",
				Tags:           nil,
				Nodes:          []string{a.Config.NodeName},
				InstanceCount:  1,
				ChecksPassing:  1,
				ChecksWarning:  0,
				ChecksCritical: 0,
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
		summary := obj.([]*ServiceSummary)
		require.Len(t, summary, 2)

		// internal accounting that users don't see can be blown away
		for _, sum := range summary {
			sum.externalSourceSet = nil
			sum.proxyForSet = nil
		}

		expected := []*ServiceSummary{
			&ServiceSummary{
				Kind:           structs.ServiceKindTypical,
				Name:           "api",
				Tags:           []string{"tag1", "tag2"},
				Nodes:          []string{"foo"},
				InstanceCount:  1,
				ChecksPassing:  2,
				ChecksWarning:  1,
				ChecksCritical: 0,
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
			},
			&ServiceSummary{
				Kind:            structs.ServiceKindConnectProxy,
				Name:            "web",
				Tags:            nil,
				Nodes:           []string{"bar", "foo"},
				InstanceCount:   2,
				ProxyFor:        []string{"api"},
				ChecksPassing:   2,
				ChecksWarning:   1,
				ChecksCritical:  1,
				ExternalSources: []string{"k8s"},
				EnterpriseMeta:  *structs.DefaultEnterpriseMeta(),
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

	dump := obj.([]*ServiceSummary)
	expect := []*ServiceSummary{
		{
			Name:           "redis",
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
		},
		{
			Name:           "db",
			Tags:           []string{"backup", "primary"},
			Nodes:          []string{"bar", "baz"},
			InstanceCount:  2,
			ChecksPassing:  1,
			ChecksWarning:  1,
			ChecksCritical: 0,
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
		},
	}
	assert.ElementsMatch(t, expect, dump)
}

func TestUIGatewayServiceNodes_Ingress(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, "")
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
					Protocol: "tcp",
					Services: []structs.IngressService{
						{
							Name: "web",
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
		require.NoError(t, a.RPC("ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	// Request
	req, _ := http.NewRequest("GET", "/v1/internal/ui/gateway-services-nodes/ingress-gateway", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.UIGatewayServicesNodes(resp, req)
	assert.Nil(t, err)
	assertIndex(t, resp)

	dump := obj.([]*ServiceSummary)
	expect := []*ServiceSummary{
		{
			Name:           "web",
			GatewayConfig:  GatewayConfig{ListenerPort: 8080},
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
		},
		{
			Name:           "db",
			Tags:           []string{"backup", "primary"},
			Nodes:          []string{"bar", "baz"},
			InstanceCount:  2,
			ChecksPassing:  1,
			ChecksWarning:  1,
			ChecksCritical: 0,
			GatewayConfig:  GatewayConfig{ListenerPort: 8888},
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
		},
	}
	assert.ElementsMatch(t, expect, dump)
}
