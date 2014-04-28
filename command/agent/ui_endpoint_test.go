package agent

import (
	"bytes"
	"github.com/hashicorp/consul/consul/structs"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUiIndex(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Create file
	path := filepath.Join(srv.uiDir, "my-file")
	if err := ioutil.WriteFile(path, []byte("test"), 777); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register node
	req, err := http.NewRequest("GET", "/ui/my-file", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	req.URL.Scheme = "http"
	req.URL.Host = srv.listener.Addr().String()

	// Make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify teh response
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
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest("GET", "/v1/internal/ui/nodes/dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.UINodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be 1 node for the server
	nodes := obj.(structs.NodeDump)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}
