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
	"github.com/stretchr/testify/require"
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
		a.srv.wrap(handler, []string{"GET"})(resp, req)
		if got, want := resp.Code, http.StatusForbidden; got != want {
			t.Fatalf("bad response code got %d want %d", got, want)
		}
	}

	// Make sure some other endpoint still works.
	{
		req, _ := http.NewRequest("GET", "/v1/agent/checks", nil)
		resp := httptest.NewRecorder()
		a.srv.wrap(handler, []string{"GET"})(resp, req)
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

func TestHTTPAPI_Allow_Nonprintable_Characters_With_Flag(t *testing.T) {
	a := NewTestAgent(t.Name(), "disable_http_unprintable_char_filter = true")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/kv/bad\x00ness", nil)
	resp := httptest.NewRecorder()
	a.srv.Handler.ServeHTTP(resp, req)
	// Key doesn't actually exist so we should get 404
	if got, want := resp.Code, http.StatusNotFound; got != want {
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
		a.srv.wrap(handler, []string{"GET"})(resp, req)

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
		a.srv.wrap(handler, []string{"GET"})(resp, req)

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
	a.srv.wrap(handler, []string{"GET"})(resp, req)

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
	a.srv.wrap(handler, []string{"GET"})(resp, req)

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
			a.srv.wrap(handler, []string{"GET"})(resp, req)

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
	a.srv.wrap(handler, []string{"GET"})(resp, req)

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
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()
	if d := a.srv.parseConsistency(resp, req, &b); d {
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
	if d := a.srv.parseConsistency(resp, req, &b); d {
		t.Fatalf("unexpected done")
	}

	if b.AllowStale {
		t.Fatalf("Bad: %v", b)
	}
	if !b.RequireConsistent {
		t.Fatalf("Bad: %v", b)
	}
}

// ensureConsistency check if consistency modes are correctly applied
// if maxStale < 0 => stale, without MaxStaleDuration
// if maxStale == 0 => no stale
// if maxStale > 0 => stale + check duration
func ensureConsistency(t *testing.T, a *TestAgent, path string, maxStale time.Duration, requireConsistent bool) {
	t.Helper()
	req, _ := http.NewRequest("GET", path, nil)
	var b structs.QueryOptions
	resp := httptest.NewRecorder()
	if d := a.srv.parseConsistency(resp, req, &b); d {
		t.Fatalf("unexpected done")
	}
	allowStale := maxStale.Nanoseconds() != 0
	if b.AllowStale != allowStale {
		t.Fatalf("Bad Allow Stale")
	}
	if maxStale > 0 && b.MaxStaleDuration != maxStale {
		t.Fatalf("Bad MaxStaleDuration: %d VS expected %d", b.MaxStaleDuration, maxStale)
	}
	if b.RequireConsistent != requireConsistent {
		t.Fatal("Bad Consistent")
	}
}

func TestParseConsistencyAndMaxStale(t *testing.T) {
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Default => Consistent
	a.config.DiscoveryMaxStale = time.Duration(0)
	ensureConsistency(t, a, "/v1/catalog/nodes", 0, false)
	// Stale, without MaxStale
	ensureConsistency(t, a, "/v1/catalog/nodes?stale", -1, false)
	// Override explicitly
	ensureConsistency(t, a, "/v1/catalog/nodes?max_stale=3s", 3*time.Second, false)
	ensureConsistency(t, a, "/v1/catalog/nodes?stale&max_stale=3s", 3*time.Second, false)

	// stale by defaul on discovery
	a.config.DiscoveryMaxStale = time.Duration(7 * time.Second)
	ensureConsistency(t, a, "/v1/catalog/nodes", a.config.DiscoveryMaxStale, false)
	// Not in KV
	ensureConsistency(t, a, "/v1/kv/my/path", 0, false)

	// DiscoveryConsistencyLevel should apply
	ensureConsistency(t, a, "/v1/health/service/one", a.config.DiscoveryMaxStale, false)
	ensureConsistency(t, a, "/v1/catalog/service/one", a.config.DiscoveryMaxStale, false)
	ensureConsistency(t, a, "/v1/catalog/services", a.config.DiscoveryMaxStale, false)

	// Query path should be taken into account
	ensureConsistency(t, a, "/v1/catalog/services?consistent", 0, true)
	// Since stale is added, no MaxStale should be applied
	ensureConsistency(t, a, "/v1/catalog/services?stale", -1, false)
	ensureConsistency(t, a, "/v1/catalog/services?leader", 0, false)
}

func TestParseConsistency_Invalid(t *testing.T) {
	t.Parallel()
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?stale&consistent", nil)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()
	if d := a.srv.parseConsistency(resp, req, &b); !d {
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

	// Request with Authorization Bearer token
	reqAuthBearerToken, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	reqAuthBearerToken.Header.Add("Authorization", "Bearer bearer-token")

	// Request with invalid Authorization scheme
	reqAuthBearerInvalidScheme, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	reqAuthBearerInvalidScheme.Header.Add("Authorization", "Beer")

	// Request with empty Authorization Bearer token
	reqAuthBearerTokenEmpty, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	reqAuthBearerTokenEmpty.Header.Add("Authorization", "Bearer")

	// Request with empty Authorization Bearer token
	reqAuthBearerTokenInvalid, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	reqAuthBearerTokenInvalid.Header.Add("Authorization", "Bearertoken")

	// Request with more than one space between Bearer and token
	reqAuthBearerTokenMultiSpaces, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	reqAuthBearerTokenMultiSpaces.Header.Add("Authorization", "Bearer     bearer-token")

	// Request with Authorization Bearer token containing spaces
	reqAuthBearerTokenSpaces, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	reqAuthBearerTokenSpaces.Header.Add("Authorization", "Bearer bearer-token "+
		" the rest is discarded   ")

	// Request with Authorization Bearer and querystring token
	reqAuthBearerAndQsToken, _ := http.NewRequest("GET", "/v1/catalog/nodes?token=qstoken", nil)
	reqAuthBearerAndQsToken.Header.Add("Authorization", "Bearer bearer-token")

	// Request with Authorization Bearer and X-Consul-Token header token
	reqAuthBearerAndXToken, _ := http.NewRequest("GET", "/v1/catalog/nodes", nil)
	reqAuthBearerAndXToken.Header.Add("X-Consul-Token", "xtoken")
	reqAuthBearerAndXToken.Header.Add("Authorization", "Bearer bearer-token")

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

	//
	// Authorization Bearer token tests
	//

	// Check if Authorization bearer token header is parsed correctly
	a.srv.parseToken(reqAuthBearerToken, &token)
	if token != "bearer-token" {
		t.Fatalf("bad: %s", token)
	}

	// Check Authorization Bearer scheme invalid
	a.srv.parseToken(reqAuthBearerInvalidScheme, &token)
	if token != "agent" {
		t.Fatalf("bad: %s", token)
	}

	// Check if Authorization Bearer token is empty
	a.srv.parseToken(reqAuthBearerTokenEmpty, &token)
	if token != "agent" {
		t.Fatalf("bad: %s", token)
	}

	// Check if the Authorization Bearer token is invalid
	a.srv.parseToken(reqAuthBearerTokenInvalid, &token)
	if token != "agent" {
		t.Fatalf("bad: %s", token)
	}

	// Check multi spaces between Authorization Bearer and token value
	a.srv.parseToken(reqAuthBearerTokenMultiSpaces, &token)
	if token != "bearer-token" {
		t.Fatalf("bad: %s", token)
	}

	// Check if Authorization Bearer token with spaces is parsed correctly
	a.srv.parseToken(reqAuthBearerTokenSpaces, &token)
	if token != "bearer-token" {
		t.Fatalf("bad: %s", token)
	}

	// Check if explicit token has precedence over Authorization bearer token
	a.srv.parseToken(reqAuthBearerAndQsToken, &token)
	if token != "qstoken" {
		t.Fatalf("bad: %s", token)
	}

	// Check if X-Consul-Token has precedence over Authorization bearer token
	a.srv.parseToken(reqAuthBearerAndXToken, &token)
	if token != "xtoken" {
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

func TestParseToken_ProxyTokenResolve(t *testing.T) {
	t.Parallel()

	type endpointCheck struct {
		endpoint string
		handler  func(s *HTTPServer, resp http.ResponseWriter, req *http.Request) (interface{}, error)
	}

	// This is not an exhaustive list of all of our endpoints and is only testing GET endpoints
	// right now. However it provides decent coverage that the proxy token resolution
	// is happening properly
	tests := []endpointCheck{
		{"/v1/acl/info/root", (*HTTPServer).ACLGet},
		{"/v1/agent/self", (*HTTPServer).AgentSelf},
		{"/v1/agent/metrics", (*HTTPServer).AgentMetrics},
		{"/v1/agent/services", (*HTTPServer).AgentServices},
		{"/v1/agent/checks", (*HTTPServer).AgentChecks},
		{"/v1/agent/members", (*HTTPServer).AgentMembers},
		{"/v1/agent/connect/ca/roots", (*HTTPServer).AgentConnectCARoots},
		{"/v1/agent/connect/ca/leaf/test", (*HTTPServer).AgentConnectCALeafCert},
		{"/v1/agent/connect/ca/proxy/test", (*HTTPServer).AgentConnectProxyConfig},
		{"/v1/catalog/connect", (*HTTPServer).CatalogConnectServiceNodes},
		{"/v1/catalog/datacenters", (*HTTPServer).CatalogDatacenters},
		{"/v1/catalog/nodes", (*HTTPServer).CatalogNodes},
		{"/v1/catalog/node/" + t.Name(), (*HTTPServer).CatalogNodeServices},
		{"/v1/catalog/services", (*HTTPServer).CatalogServices},
		{"/v1/catalog/service/test", (*HTTPServer).CatalogServiceNodes},
		{"/v1/connect/ca/configuration", (*HTTPServer).ConnectCAConfiguration},
		{"/v1/connect/ca/roots", (*HTTPServer).ConnectCARoots},
		{"/v1/connect/intentions", (*HTTPServer).IntentionEndpoint},
		{"/v1/coordinate/datacenters", (*HTTPServer).CoordinateDatacenters},
		{"/v1/coordinate/nodes", (*HTTPServer).CoordinateNodes},
		{"/v1/coordinate/node/" + t.Name(), (*HTTPServer).CoordinateNode},
		{"/v1/event/list", (*HTTPServer).EventList},
		{"/v1/health/node/" + t.Name(), (*HTTPServer).HealthNodeChecks},
		{"/v1/health/checks/test", (*HTTPServer).HealthNodeChecks},
		{"/v1/health/state/passing", (*HTTPServer).HealthChecksInState},
		{"/v1/health/service/test", (*HTTPServer).HealthServiceNodes},
		{"/v1/health/connect/test", (*HTTPServer).HealthConnectServiceNodes},
		{"/v1/operator/raft/configuration", (*HTTPServer).OperatorRaftConfiguration},
		// keyring endpoint has issues with returning errors if you haven't enabled encryption
		// {"/v1/operator/keyring", (*HTTPServer).OperatorKeyringEndpoint},
		{"/v1/operator/autopilot/configuration", (*HTTPServer).OperatorAutopilotConfiguration},
		{"/v1/operator/autopilot/health", (*HTTPServer).OperatorServerHealth},
		{"/v1/query", (*HTTPServer).PreparedQueryGeneral},
		{"/v1/session/list", (*HTTPServer).SessionList},
		{"/v1/status/leader", (*HTTPServer).StatusLeader},
		{"/v1/status/peers", (*HTTPServer).StatusPeers},
	}

	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.Code, "body: %s", resp.Body.String())
	}

	// Get the proxy token from the agent directly, since there is no API.
	proxy := a.State.Proxy("test-id-proxy")
	require.NotNil(t, proxy)
	token := proxy.ProxyToken
	require.NotEmpty(t, token)

	for _, check := range tests {
		t.Run(fmt.Sprintf("GET(%s)", check.endpoint), func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("%s?token=%s", check.endpoint, token), nil)
			resp := httptest.NewRecorder()
			_, err := check.handler(a.srv, resp, req)
			require.NoError(t, err)
		})
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
