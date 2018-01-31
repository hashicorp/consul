package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/go-cleanhttp"
	"golang.org/x/net/http2"
)

func TestHTTPServer_UnixSocket(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tempDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(tempDir)
	socket := filepath.Join(tempDir, "test.sock")

	// Only testing mode, since uid/gid might not be settable
	// from test environment.
	a := NewTestAgent(t.Name(), `
		addresses {
			http = "unix://`+socket+`"
		}
		unix_sockets {
			mode = "0777"
		}
	`)
	defer a.Shutdown()

	// Ensure the socket was created
	if _, err := os.Stat(socket); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the mode was set properly
	fi, err := os.Stat(socket)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if fi.Mode().String() != "Srwxrwxrwx" {
		t.Fatalf("bad permissions: %s", fi.Mode())
	}

	// Ensure we can get a response from the socket.
	trans := cleanhttp.DefaultTransport()
	trans.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
		return net.Dial("unix", socket)
	}
	client := &http.Client{
		Transport: trans,
	}

	// This URL doesn't look like it makes sense, but the scheme (http://) and
	// the host (127.0.0.1) are required by the HTTP client library. In reality
	// this will just use the custom dialer and talk to the socket.
	resp, err := client.Get("http://127.0.0.1/v1/agent/self")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer resp.Body.Close()

	if body, err := ioutil.ReadAll(resp.Body); err != nil || len(body) == 0 {
		t.Fatalf("bad: %s %v", body, err)
	}
}

func TestHTTPServer_UnixSocket_FileExists(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tempDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(tempDir)
	socket := filepath.Join(tempDir, "test.sock")

	// Create a regular file at the socket path
	if err := ioutil.WriteFile(socket, []byte("hello world"), 0644); err != nil {
		t.Fatalf("err: %s", err)
	}
	fi, err := os.Stat(socket)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !fi.Mode().IsRegular() {
		t.Fatalf("not a regular file: %s", socket)
	}

	a := NewTestAgent(t.Name(), `
		addresses {
			http = "unix://`+socket+`"
		}
	`)
	defer a.Shutdown()

	// Ensure the file was replaced by the socket
	fi, err = os.Stat(socket)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("expected socket to replace file")
	}
}

func TestHTTPServer_H2(t *testing.T) {
	t.Parallel()

	// Fire up an agent with TLS enabled.
	a := &TestAgent{
		Name:   t.Name(),
		UseTLS: true,
		HCL: `
			key_file = "../test/client_certs/server.key"
			cert_file = "../test/client_certs/server.crt"
			ca_file = "../test/client_certs/rootca.crt"
		`,
	}
	a.Start()
	defer a.Shutdown()

	// Make an HTTP/2-enabled client, using the API helpers to set
	// up TLS to be as normal as possible for Consul.
	tlscfg := &api.TLSConfig{
		Address:  "consul.test",
		KeyFile:  "../test/client_certs/client.key",
		CertFile: "../test/client_certs/client.crt",
		CAFile:   "../test/client_certs/rootca.crt",
	}
	tlsccfg, err := api.SetupTLSConfig(tlscfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	transport := api.DefaultConfig().Transport
	transport.TLSClientConfig = tlsccfg
	if err := http2.ConfigureTransport(transport); err != nil {
		t.Fatalf("err: %v", err)
	}
	hc := &http.Client{
		Transport: transport,
	}

	// Hook a handler that echoes back the protocol.
	handler := func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusOK)
		fmt.Fprint(resp, req.Proto)
	}
	w, ok := a.srv.Handler.(*wrappedMux)
	if !ok {
		t.Fatalf("handler is not expected type")
	}
	w.mux.HandleFunc("/echo", handler)

	// Call it and make sure we see HTTP/2.
	url := fmt.Sprintf("https://%s/echo", a.srv.ln.Addr().String())
	resp, err := hc.Get(url)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !bytes.Equal(body, []byte("HTTP/2.0")) {
		t.Fatalf("bad: %v", body)
	}

	// This doesn't have a closed-loop way to verify HTTP/2 support for
	// some other endpoint, but configure an API client and make a call
	// just as a sanity check.
	cfg := &api.Config{
		Address:    a.srv.ln.Addr().String(),
		Scheme:     "https",
		HttpClient: hc,
	}
	client, err := api.NewClient(cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := client.Agent().Self(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSetIndex(t *testing.T) {
	t.Parallel()
	resp := httptest.NewRecorder()
	setIndex(resp, 1000)
	header := resp.Header().Get("X-Consul-Index")
	if header != "1000" {
		t.Fatalf("Bad: %v", header)
	}
	setIndex(resp, 2000)
	if v := resp.Header()["X-Consul-Index"]; len(v) != 1 {
		t.Fatalf("bad: %#v", v)
	}
}

func TestSetKnownLeader(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	tests := []struct {
		desc string
		d    time.Duration
		h    string
	}{
		{"neg", -1, "0"},
		{"zero", 0, "0"},
		{"pos", 123 * time.Millisecond, "123"},
		{"pos ms only", 123456 * time.Microsecond, "123"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			resp := httptest.NewRecorder()
			setLastContact(resp, tt.d)
			header := resp.Header().Get("X-Consul-LastContact")
			if got, want := header, tt.h; got != want {
				t.Fatalf("got X-Consul-LastContact header %q want %q", got, want)
			}
		})
	}
}

func TestSetMeta(t *testing.T) {
	t.Parallel()
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

func TestHTTPAPI_BlockEndpoints(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), `
		http_config {
			block_endpoints = ["/v1/agent/self"]
		}
	`)
	defer a.Shutdown()

	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		return nil, nil
	}

	// Try a blocked endpoint, which should get a 403.
	{
		req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
		resp := httptest.NewRecorder()
		a.srv.wrap(handler)(resp, req)
		if got, want := resp.Code, http.StatusForbidden; got != want {
			t.Fatalf("bad response code got %d want %d", got, want)
		}
	}

	// Make sure some other endpoint still works.
	{
		req, _ := http.NewRequest("GET", "/v1/agent/checks", nil)
		resp := httptest.NewRecorder()
		a.srv.wrap(handler)(resp, req)
		if got, want := resp.Code, http.StatusOK; got != want {
			t.Fatalf("bad response code got %d want %d", got, want)
		}
	}
}

func TestHTTPAPI_Ban_Nonprintable_Characters(t *testing.T) {
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/kv/bad\x00ness", nil)
	resp := httptest.NewRecorder()
	a.srv.Handler.ServeHTTP(resp, req)
	if got, want := resp.Code, http.StatusBadRequest; got != want {
		t.Fatalf("bad response code got %d want %d", got, want)
	}
}

func TestHTTPAPI_TranslateAddrHeader(t *testing.T) {
	t.Parallel()
	// Header should not be present if address translation is off.
	{
		a := NewTestAgent(t.Name(), "")
		defer a.Shutdown()

		resp := httptest.NewRecorder()
		handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
			return nil, nil
		}

		req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
		a.srv.wrap(handler)(resp, req)

		translate := resp.Header().Get("X-Consul-Translate-Addresses")
		if translate != "" {
			t.Fatalf("bad: expected %q, got %q", "", translate)
		}
	}

	// Header should be set to true if it's turned on.
	{
		a := NewTestAgent(t.Name(), `
			translate_wan_addrs = true
		`)
		defer a.Shutdown()

		resp := httptest.NewRecorder()
		handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
			return nil, nil
		}

		req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
		a.srv.wrap(handler)(resp, req)

		translate := resp.Header().Get("X-Consul-Translate-Addresses")
		if translate != "true" {
			t.Fatalf("bad: expected %q, got %q", "true", translate)
		}
	}
}

func TestHTTPAPIResponseHeaders(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		http_config {
			response_headers = {
				"Access-Control-Allow-Origin" = "*"
				"X-XSS-Protection" = "1; mode=block"
			}
		}
	`)
	defer a.Shutdown()

	resp := httptest.NewRecorder()
	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		return nil, nil
	}

	req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
	a.srv.wrap(handler)(resp, req)

	origin := resp.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Fatalf("bad Access-Control-Allow-Origin: expected %q, got %q", "*", origin)
	}

	xss := resp.Header().Get("X-XSS-Protection")
	if xss != "1; mode=block" {
		t.Fatalf("bad X-XSS-Protection header: expected %q, got %q", "1; mode=block", xss)
	}
}

func TestContentTypeIsJSON(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	resp := httptest.NewRecorder()
	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		// stub out a DirEntry so that it will be encoded as JSON
		return &structs.DirEntry{Key: "key"}, nil
	}

	req, _ := http.NewRequest("GET", "/v1/kv/key", nil)
	a.srv.wrap(handler)(resp, req)

	contentType := resp.Header().Get("Content-Type")

	if contentType != "application/json" {
		t.Fatalf("Content-Type header was not 'application/json'")
	}
}

func TestHTTP_wrap_obfuscateLog(t *testing.T) {
	t.Parallel()
	buf := new(bytes.Buffer)
	a := &TestAgent{Name: t.Name(), LogOutput: buf}
	a.Start()
	defer a.Shutdown()

	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		return nil, nil
	}

	for _, pair := range [][]string{
		{
			"/some/url?token=secret1&token=secret2",
			"/some/url?token=<hidden>&token=<hidden>",
		},
		{
			"/v1/acl/clone/secret1",
			"/v1/acl/clone/<hidden>",
		},
		{
			"/v1/acl/clone/secret1?token=secret2",
			"/v1/acl/clone/<hidden>?token=<hidden>",
		},
		{
			"/v1/acl/destroy/secret1",
			"/v1/acl/destroy/<hidden>",
		},
		{
			"/v1/acl/destroy/secret1?token=secret2",
			"/v1/acl/destroy/<hidden>?token=<hidden>",
		},
		{
			"/v1/acl/info/secret1",
			"/v1/acl/info/<hidden>",
		},
		{
			"/v1/acl/info/secret1?token=secret2",
			"/v1/acl/info/<hidden>?token=<hidden>",
		},
	} {
		url, want := pair[0], pair[1]
		t.Run(url, func(t *testing.T) {
			resp := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", url, nil)
			a.srv.wrap(handler)(resp, req)

			if got := buf.String(); !strings.Contains(got, want) {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
}

func TestPrettyPrint(t *testing.T) {
	t.Parallel()
	testPrettyPrint("pretty=1", t)
}

func TestPrettyPrintBare(t *testing.T) {
	t.Parallel()
	testPrettyPrint("pretty", t)
}

func testPrettyPrint(pretty string, t *testing.T) {
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	r := &structs.DirEntry{Key: "key"}

	resp := httptest.NewRecorder()
	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		return r, nil
	}

	urlStr := "/v1/kv/key?" + pretty
	req, _ := http.NewRequest("GET", urlStr, nil)
	a.srv.wrap(handler)(resp, req)

	expected, _ := json.MarshalIndent(r, "", "    ")
	expected = append(expected, "\n"...)
	actual, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !bytes.Equal(expected, actual) {
		t.Fatalf("bad: %q", string(actual))
	}
}

func TestParseSource(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Default is agent's DC and no node (since the user didn't care, then
	// just give them the cheapest possible query).
	req, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	source := structs.QuerySource{}
	a.srv.parseSource(req, &source)
	if source.Datacenter != "dc1" || source.Node != "" {
		t.Fatalf("bad: %v", source)
	}

	// Adding the source parameter should set that node.
	req, _ = http.NewRequest("GET", "/v1/catalog/nodes?near=bob", nil)
	source = structs.QuerySource{}
	a.srv.parseSource(req, &source)
	if source.Datacenter != "dc1" || source.Node != "bob" {
		t.Fatalf("bad: %v", source)
	}

	// We should follow whatever dc parameter was given so that the node is
	// looked up correctly on the receiving end.
	req, _ = http.NewRequest("GET", "/v1/catalog/nodes?near=bob&dc=foo", nil)
	source = structs.QuerySource{}
	a.srv.parseSource(req, &source)
	if source.Datacenter != "foo" || source.Node != "bob" {
		t.Fatalf("bad: %v", source)
	}

	// The magic "_agent" node name will use the agent's local node name.
	req, _ = http.NewRequest("GET", "/v1/catalog/nodes?near=_agent", nil)
	source = structs.QuerySource{}
	a.srv.parseSource(req, &source)
	if source.Datacenter != "dc1" || source.Node != a.Config.NodeName {
		t.Fatalf("bad: %v", source)
	}
}

func TestParseWait(t *testing.T) {
	t.Parallel()
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?wait=60s&index=1000", nil)
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
	t.Parallel()
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?wait=60foo&index=1000", nil)
	if d := parseWait(resp, req, &b); !d {
		t.Fatalf("expected done")
	}

	if resp.Code != 400 {
		t.Fatalf("bad code: %v", resp.Code)
	}
}

func TestParseWait_InvalidIndex(t *testing.T) {
	t.Parallel()
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?wait=60s&index=foo", nil)
	if d := parseWait(resp, req, &b); !d {
		t.Fatalf("expected done")
	}

	if resp.Code != 400 {
		t.Fatalf("bad code: %v", resp.Code)
	}
}

func TestParseConsistency(t *testing.T) {
	t.Parallel()
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?stale", nil)
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
	req, _ = http.NewRequest("GET", "/v1/catalog/nodes?consistent", nil)
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
	t.Parallel()
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?stale&consistent", nil)
	if d := parseConsistency(resp, req, &b); !d {
		t.Fatalf("expected done")
	}

	if resp.Code != 400 {
		t.Fatalf("bad code: %v", resp.Code)
	}
}

// Test ACL token is resolved in correct order
func TestACLResolution(t *testing.T) {
	t.Parallel()
	var token string
	// Request without token
	req, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	// Request with explicit token
	reqToken, _ := http.NewRequest("GET", "/v1/catalog/nodes?token=foo", nil)
	// Request with header token only
	reqHeaderToken, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	reqHeaderToken.Header.Add("X-Consul-Token", "bar")

	// Request with header and querystring tokens
	reqBothTokens, _ := http.NewRequest("GET", "/v1/catalog/nodes?token=baz", nil)
	reqBothTokens.Header.Add("X-Consul-Token", "zap")

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Check when no token is set
	a.tokens.UpdateUserToken("")
	a.srv.parseToken(req, &token)
	if token != "" {
		t.Fatalf("bad: %s", token)
	}

	// Check when ACLToken set
	a.tokens.UpdateUserToken("agent")
	a.srv.parseToken(req, &token)
	if token != "agent" {
		t.Fatalf("bad: %s", token)
	}

	// Explicit token has highest precedence
	a.srv.parseToken(reqToken, &token)
	if token != "foo" {
		t.Fatalf("bad: %s", token)
	}

	// Header token has precedence over agent token
	a.srv.parseToken(reqHeaderToken, &token)
	if token != "bar" {
		t.Fatalf("bad: %s", token)
	}

	// Querystring token has precedence over header and agent tokens
	a.srv.parseToken(reqBothTokens, &token)
	if token != "baz" {
		t.Fatalf("bad: %s", token)
	}
}

func TestEnableWebUI(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		ui = true
	`)
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/ui/", nil)
	resp := httptest.NewRecorder()
	a.srv.Handler.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("should handle ui")
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

func jsonReader(v interface{}) io.Reader {
	if v == nil {
		return nil
	}
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(v); err != nil {
		panic(err)
	}
	return b
}
