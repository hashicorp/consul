package agent

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	cleanhttp "github.com/hashicorp/go-cleanhttp"
)

func TestUiIndex(t *testing.T) {
	t.Parallel()
	// Make a test dir to serve UI files
	uiDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(uiDir)

	// Make the server
	a := NewTestAgent(t.Name(), `
		ui_dir = "`+uiDir+`"
	`)
	defer a.Shutdown()

	// Create file
	path := filepath.Join(a.Config.UIDir, "my-file")
	if err := ioutil.WriteFile(path, []byte("test"), 777); err != nil {
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

	// Verify the response
	if resp.StatusCode != 200 {
		t.Fatalf("bad: %v", resp)
	}

	// Verify the body
	out := bytes.NewBuffer(nil)
	io.Copy(out, resp.Body)
	if string(out.Bytes()) != "test" {
		t.Fatalf("bad: %s", out.Bytes())
	}
}

func TestUiNodes(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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

func TestUiNodeInfo(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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

func TestSummarizeServices(t *testing.T) {
	t.Parallel()
	dump := structs.NodeDump{
		&structs.NodeInfo{
			Node:    "foo",
			Address: "127.0.0.1",
			Services: []*structs.NodeService{
				&structs.NodeService{
					Service: "api",
				},
				&structs.NodeService{
					Service: "web",
				},
			},
			Checks: []*structs.HealthCheck{
				&structs.HealthCheck{
					Status:      api.HealthPassing,
					ServiceName: "",
				},
				&structs.HealthCheck{
					Status:      api.HealthPassing,
					ServiceName: "web",
				},
				&structs.HealthCheck{
					Status:      api.HealthWarning,
					ServiceName: "api",
				},
			},
		},
		&structs.NodeInfo{
			Node:    "bar",
			Address: "127.0.0.2",
			Services: []*structs.NodeService{
				&structs.NodeService{
					Service: "web",
				},
			},
			Checks: []*structs.HealthCheck{
				&structs.HealthCheck{
					Status:      api.HealthCritical,
					ServiceName: "web",
				},
			},
		},
		&structs.NodeInfo{
			Node:    "zip",
			Address: "127.0.0.3",
			Services: []*structs.NodeService{
				&structs.NodeService{
					Service: "cache",
				},
			},
		},
	}

	summary := summarizeServices(dump)
	if len(summary) != 3 {
		t.Fatalf("bad: %v", summary)
	}

	expectAPI := &ServiceSummary{
		Name:           "api",
		Nodes:          []string{"foo"},
		ChecksPassing:  1,
		ChecksWarning:  1,
		ChecksCritical: 0,
	}
	if !reflect.DeepEqual(summary[0], expectAPI) {
		t.Fatalf("bad: %v", summary[0])
	}

	expectCache := &ServiceSummary{
		Name:           "cache",
		Nodes:          []string{"zip"},
		ChecksPassing:  0,
		ChecksWarning:  0,
		ChecksCritical: 0,
	}
	if !reflect.DeepEqual(summary[1], expectCache) {
		t.Fatalf("bad: %v", summary[1])
	}

	expectWeb := &ServiceSummary{
		Name:           "web",
		Nodes:          []string{"bar", "foo"},
		ChecksPassing:  2,
		ChecksWarning:  0,
		ChecksCritical: 1,
	}
	if !reflect.DeepEqual(summary[2], expectWeb) {
		t.Fatalf("bad: %v", summary[2])
	}
}
