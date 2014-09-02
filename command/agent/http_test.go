package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
)

func makeHTTPServer(t *testing.T) (string, *HTTPServer) {
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	uiDir := filepath.Join(dir, "ui")
	if err := os.Mkdir(uiDir, 755); err != nil {
		t.Fatalf("err: %v", err)
	}
	addr, _ := agent.config.ClientListener("", agent.config.Ports.HTTP)
	server, err := NewHTTPServer(agent, uiDir, true, agent.logOutput, addr.String())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, server
}

func encodeReq(obj interface{}) io.ReadCloser {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(obj)
	return ioutil.NopCloser(buf)
}

func TestSetIndex(t *testing.T) {
	resp := httptest.NewRecorder()
	setIndex(resp, 1000)
	header := resp.Header().Get("X-Consul-Index")
	if header != "1000" {
		t.Fatalf("Bad: %v", header)
	}
}

func TestSetKnownLeader(t *testing.T) {
	resp := httptest.NewRecorder()
	setKnownLeader(resp, true)
	header := resp.Header().Get("X-Consul-KnownLeader")
	if header != "true" {
		t.Fatalf("Bad: %v", header)
	}
	resp = httptest.NewRecorder()
	setKnownLeader(resp, false)
	header = resp.Header().Get("X-Consul-KnownLeader")
	if header != "false" {
		t.Fatalf("Bad: %v", header)
	}
}

func TestSetLastContact(t *testing.T) {
	resp := httptest.NewRecorder()
	setLastContact(resp, 123456*time.Microsecond)
	header := resp.Header().Get("X-Consul-LastContact")
	if header != "123" {
		t.Fatalf("Bad: %v", header)
	}
}

func TestSetMeta(t *testing.T) {
	meta := structs.QueryMeta{
		Index:       1000,
		KnownLeader: true,
		LastContact: 123456 * time.Microsecond,
	}
	resp := httptest.NewRecorder()
	setMeta(resp, &meta)
	header := resp.Header().Get("X-Consul-Index")
	if header != "1000" {
		t.Fatalf("Bad: %v", header)
	}
	header = resp.Header().Get("X-Consul-KnownLeader")
	if header != "true" {
		t.Fatalf("Bad: %v", header)
	}
	header = resp.Header().Get("X-Consul-LastContact")
	if header != "123" {
		t.Fatalf("Bad: %v", header)
	}
}

func TestContentTypeIsJSON(t *testing.T) {
	dir, srv := makeHTTPServer(t)

	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	resp := httptest.NewRecorder()

	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		// stub out a DirEntry so that it will be encoded as JSON
		return &structs.DirEntry{Key: "key"}, nil
	}

	req, _ := http.NewRequest("GET", "/v1/kv/key", nil)
	srv.wrap(handler)(resp, req)

	contentType := resp.Header().Get("Content-Type")

	if contentType != "application/json" {
		t.Fatalf("Content-Type header was not 'application/json'")
	}
}

func TestParseWait(t *testing.T) {
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, err := http.NewRequest("GET",
		"/v1/catalog/nodes?wait=60s&index=1000", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if d := parseWait(resp, req, &b); d {
		t.Fatalf("unexpected done")
	}

	if b.MinQueryIndex != 1000 {
		t.Fatalf("Bad: %v", b)
	}
	if b.MaxQueryTime != 60*time.Second {
		t.Fatalf("Bad: %v", b)
	}
}

func TestParseWait_InvalidTime(t *testing.T) {
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, err := http.NewRequest("GET",
		"/v1/catalog/nodes?wait=60foo&index=1000", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if d := parseWait(resp, req, &b); !d {
		t.Fatalf("expected done")
	}

	if resp.Code != 400 {
		t.Fatalf("bad code: %v", resp.Code)
	}
}

func TestParseWait_InvalidIndex(t *testing.T) {
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, err := http.NewRequest("GET",
		"/v1/catalog/nodes?wait=60s&index=foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if d := parseWait(resp, req, &b); !d {
		t.Fatalf("expected done")
	}

	if resp.Code != 400 {
		t.Fatalf("bad code: %v", resp.Code)
	}
}

func TestParseConsistency(t *testing.T) {
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, err := http.NewRequest("GET",
		"/v1/catalog/nodes?stale", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if d := parseConsistency(resp, req, &b); d {
		t.Fatalf("unexpected done")
	}

	if !b.AllowStale {
		t.Fatalf("Bad: %v", b)
	}
	if b.RequireConsistent {
		t.Fatalf("Bad: %v", b)
	}

	b = structs.QueryOptions{}
	req, err = http.NewRequest("GET",
		"/v1/catalog/nodes?consistent", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if d := parseConsistency(resp, req, &b); d {
		t.Fatalf("unexpected done")
	}

	if b.AllowStale {
		t.Fatalf("Bad: %v", b)
	}
	if !b.RequireConsistent {
		t.Fatalf("Bad: %v", b)
	}
}

func TestParseConsistency_Invalid(t *testing.T) {
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, err := http.NewRequest("GET",
		"/v1/catalog/nodes?stale&consistent", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if d := parseConsistency(resp, req, &b); !d {
		t.Fatalf("expected done")
	}

	if resp.Code != 400 {
		t.Fatalf("bad code: %v", resp.Code)
	}
}

// assertIndex tests that X-Consul-Index is set and non-zero
func assertIndex(t *testing.T, resp *httptest.ResponseRecorder) {
	header := resp.Header().Get("X-Consul-Index")
	if header == "" || header == "0" {
		t.Fatalf("Bad: %v", header)
	}
}

// checkIndex is like assertIndex but returns an error
func checkIndex(resp *httptest.ResponseRecorder) error {
	header := resp.Header().Get("X-Consul-Index")
	if header == "" || header == "0" {
		return fmt.Errorf("Bad: %v", header)
	}
	return nil
}

// getIndex parses X-Consul-Index
func getIndex(t *testing.T, resp *httptest.ResponseRecorder) uint64 {
	header := resp.Header().Get("X-Consul-Index")
	if header == "" {
		t.Fatalf("Bad: %v", header)
	}
	val, err := strconv.Atoi(header)
	if err != nil {
		t.Fatalf("Bad: %v", header)
	}
	return uint64(val)
}

func httpTest(t *testing.T, f func(srv *HTTPServer)) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()
	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")
	f(srv)
}
