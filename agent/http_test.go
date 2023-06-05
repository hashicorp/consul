package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestHTTPServer_UnixSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tempDir := testutil.TempDir(t, "consul")
	socket := filepath.Join(tempDir, "test.sock")

	// Only testing mode, since uid/gid might not be settable
	// from test environment.
	a := NewTestAgent(t, `
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

	if body, err := io.ReadAll(resp.Body); err != nil || len(body) == 0 {
		t.Fatalf("bad: %s %v", body, err)
	}
}

func TestHTTPServer_UnixSocket_FileExists(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tempDir := testutil.TempDir(t, "consul")
	socket := filepath.Join(tempDir, "test.sock")

	// Create a regular file at the socket path
	if err := os.WriteFile(socket, []byte("hello world"), 0644); err != nil {
		t.Fatalf("err: %s", err)
	}
	fi, err := os.Stat(socket)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !fi.Mode().IsRegular() {
		t.Fatalf("not a regular file: %s", socket)
	}

	a := NewTestAgent(t, `
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

func TestHTTPSServer_UnixSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tempDir := testutil.TempDir(t, "consul")
	socket := filepath.Join(tempDir, "test.sock")

	a := StartTestAgent(t, TestAgent{
		UseHTTPS: true,
		HCL: `
			addresses {
				https = "unix://` + socket + `"
			}
			unix_sockets {
				mode = "0777"
			}
			tls {
				defaults {
					  ca_file = "../test/client_certs/rootca.crt"
					  cert_file = "../test/client_certs/server.crt"
					  key_file = "../test/client_certs/server.key"
				}
		  	}
		`,
	})
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
	transport.TLSHandshakeTimeout = 30 * time.Second
	transport.TLSClientConfig = tlsccfg
	if err := http2.ConfigureTransport(transport); err != nil {
		t.Fatalf("err: %v", err)
	}
	transport.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
		return net.Dial("unix", socket)
	}
	client := &http.Client{Transport: transport}

	u, err := url.Parse("https://unix" + socket)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	u.Path = "/v1/agent/self"
	u.Scheme = "https"
	resp, err := client.Get(u.String())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer resp.Body.Close()

	if body, err := io.ReadAll(resp.Body); err != nil || len(body) == 0 {
		t.Fatalf("bad: %s %v", body, err)
	} else if !strings.Contains(string(body), "NodeName") {
		t.Fatalf("NodeName not found in results: %s", string(body))
	}
}

func TestSetupHTTPServer_HTTP2(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// Fire up an agent with TLS enabled.
	a := StartTestAgent(t, TestAgent{
		UseHTTPS: true,
		HCL: `
			tls {
				defaults {
				  ca_file = "../test/client_certs/rootca.crt"
				  cert_file = "../test/client_certs/server.crt"
				  key_file = "../test/client_certs/server.key"
				}
		  	}
		`,
	})
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
	transport.TLSHandshakeTimeout = 30 * time.Second
	transport.TLSClientConfig = tlsccfg
	if err := http2.ConfigureTransport(transport); err != nil {
		t.Fatalf("err: %v", err)
	}
	httpClient := &http.Client{Transport: transport}

	// Hook a handler that echoes back the protocol.
	handler := func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusOK)
		fmt.Fprint(resp, req.Proto)
	}

	// Create an httpServer to be configured with setupHTTPS, and add our
	// custom handler.
	httpServer := &http.Server{}
	noopConnState := func(net.Conn, http.ConnState) {}
	err = setupHTTPS(httpServer, noopConnState, time.Second)
	require.NoError(t, err)

	a.config.EnableDebug = true
	srvHandler := a.srv.handler()
	mux, ok := srvHandler.(*wrappedMux)
	require.True(t, ok, "expected a *wrappedMux, got %T", handler)
	mux.mux.HandleFunc("/echo", handler)
	httpServer.Handler = mux

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	tlsListener := tls.NewListener(listener, a.tlsConfigurator.IncomingHTTPSConfig())

	go httpServer.Serve(tlsListener)
	defer httpServer.Shutdown(context.Background())

	// Call it and make sure we see HTTP/2.
	url := fmt.Sprintf("https://%s/echo", listener.Addr().String())
	resp, err := httpClient.Get(url)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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
		Address:    listener.Addr().String(),
		Scheme:     "https",
		HttpClient: httpClient,
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

func TestSetFilteredByACLs(t *testing.T) {
	t.Parallel()
	resp := httptest.NewRecorder()
	setResultsFilteredByACLs(resp, true)
	header := resp.Header().Get("X-Consul-Results-Filtered-By-ACLs")
	if header != "true" {
		t.Fatalf("Bad: %v", header)
	}
	resp = httptest.NewRecorder()
	setResultsFilteredByACLs(resp, false)
	header = resp.Header().Get("X-Consul-Results-Filtered-By-ACLs")
	if header != "" {
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
		Index:                 1000,
		KnownLeader:           true,
		LastContact:           123456 * time.Microsecond,
		ResultsFilteredByACLs: true,
	}
	resp := httptest.NewRecorder()
	setMeta(resp, &meta)

	testCases := map[string]string{
		"X-Consul-Index":                    "1000",
		"X-Consul-KnownLeader":              "true",
		"X-Consul-LastContact":              "123",
		"X-Consul-Results-Filtered-By-ACLs": "true",
	}
	for header, expectedValue := range testCases {
		if v := resp.Header().Get(header); v != expectedValue {
			t.Fatalf("expected %q for header %s got %q", expectedValue, header, v)
		}
	}
}

func TestHTTPAPI_BlockEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, `
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	_, err := http.NewRequest("GET", "/v1/kv/bad\x00ness", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	req, err := http.NewRequest("GET", "/v1/kv/bad%00ness", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp := httptest.NewRecorder()
	a.config.EnableDebug = true
	a.srv.handler().ServeHTTP(resp, req)
	if got, want := resp.Code, http.StatusBadRequest; got != want {
		t.Fatalf("bad response code got %d want %d", got, want)
	}
}

func TestHTTPAPI_Allow_Nonprintable_Characters_With_Flag(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "disable_http_unprintable_char_filter = true")
	defer a.Shutdown()

	_, err := http.NewRequest("GET", "/v1/kv/bad\x00ness", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	req, err := http.NewRequest("GET", "/v1/kv/bad%00ness", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp := httptest.NewRecorder()
	a.config.EnableDebug = true
	a.srv.handler().ServeHTTP(resp, req)
	// Key doesn't actually exist so we should get 404
	if got, want := resp.Code, http.StatusNotFound; got != want {
		t.Fatalf("bad response code got %d want %d", got, want)
	}
}

func TestHTTPAPI_TranslateAddrHeader(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	// Header should not be present if address translation is off.
	{
		a := NewTestAgent(t, "")
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
		a := NewTestAgent(t, `
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

func TestHTTPAPI_DefaultACLPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	type testcase struct {
		name   string
		hcl    string
		expect string
	}

	cases := []testcase{
		{
			name:   "default is allow",
			hcl:    ``,
			expect: "allow",
		},
		{
			name:   "explicit allow",
			hcl:    `acl { default_policy = "allow" }`,
			expect: "allow",
		},
		{
			name:   "explicit deny",
			hcl:    `acl { default_policy = "deny" }`,
			expect: "deny",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := NewTestAgent(t, tc.hcl)
			defer a.Shutdown()

			resp := httptest.NewRecorder()
			handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
				return nil, nil
			}

			req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
			a.srv.wrap(handler, []string{"GET"})(resp, req)

			require.Equal(t, tc.expect, resp.Header().Get("X-Consul-Default-ACL-Policy"))
		})
	}
}

func TestHTTPAPIResponseHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		ui_config {
			# Explicitly disable UI so we can ensure the index replacement gets headers too.
			enabled = false
		}
		http_config {
			response_headers = {
				"Access-Control-Allow-Origin" = "*"
				"X-XSS-Protection" = "1; mode=block"
				"X-Frame-Options" = "SAMEORIGIN"
			}
		}
	`)
	defer a.Shutdown()

	requireHasHeadersSet(t, a, "/v1/agent/self")

	// Check the Index page that just renders a simple message with UI disabled
	// also gets the right headers.
	requireHasHeadersSet(t, a, "/")
}

func requireHasHeadersSet(t *testing.T, a *TestAgent, path string) {
	t.Helper()

	resp := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", path, nil)
	a.config.EnableDebug = true
	a.srv.handler().ServeHTTP(resp, req)

	hdrs := resp.Header()
	require.Equal(t, "*", hdrs.Get("Access-Control-Allow-Origin"),
		"Access-Control-Allow-Origin header value incorrect")

	require.Equal(t, "1; mode=block", hdrs.Get("X-XSS-Protection"),
		"X-XSS-Protection header value incorrect")
}

func TestUIResponseHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		http_config {
			response_headers = {
				"Access-Control-Allow-Origin" = "*"
				"X-XSS-Protection" = "1; mode=block"
				"X-Frame-Options" = "SAMEORIGIN"
			}
		}
	`)
	defer a.Shutdown()

	requireHasHeadersSet(t, a, "/ui")
}

func TestAcceptEncodingGzip(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Setting up the KV to store a short and a long value
	buf := bytes.NewBuffer([]byte("short"))
	req, _ := http.NewRequest("PUT", "/v1/kv/short", buf)
	resp := httptest.NewRecorder()
	_, err := a.srv.KVSEndpoint(resp, req)
	require.NoError(t, err)

	// this generates a string which is longer than
	// gziphandler.DefaultMinSize to trigger compression.
	long := fmt.Sprintf(fmt.Sprintf("%%0%dd", gziphandler.DefaultMinSize+1), 1)
	buf = bytes.NewBuffer([]byte(long))
	req, _ = http.NewRequest("PUT", "/v1/kv/long", buf)
	resp = httptest.NewRecorder()
	_, err = a.srv.KVSEndpoint(resp, req)
	require.NoError(t, err)

	resp = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/kv/short", nil)
	// Usually this would be automatically set by transport content
	// negotiation, but since this call doesn't go through a real
	// transport, the header has to be set manually
	req.Header["Accept-Encoding"] = []string{"gzip"}
	a.config.EnableDebug = true
	a.srv.handler().ServeHTTP(resp, req)
	require.Equal(t, 200, resp.Code)
	require.Equal(t, "", resp.Header().Get("Content-Encoding"))

	resp = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/kv/long", nil)
	req.Header["Accept-Encoding"] = []string{"gzip"}
	a.config.EnableDebug = true
	a.srv.handler().ServeHTTP(resp, req)
	require.Equal(t, 200, resp.Code)
	require.Equal(t, "gzip", resp.Header().Get("Content-Encoding"))
}

func TestContentTypeIsJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	buf := &syncBuffer{b: new(bytes.Buffer)}
	a := StartTestAgent(t, TestAgent{
		LogOutput: buf,
		LogLevel:  hclog.Debug,
	})
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
			bufout := buf.String()
			require.Contains(t, bufout, want)
		})
	}
}

func TestPrettyPrint(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	testPrettyPrint("pretty=1", t)
}

func TestPrettyPrintBare(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	testPrettyPrint("pretty", t)
}

func testPrettyPrint(pretty string, t *testing.T) {
	a := NewTestAgent(t, "")
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
	actual, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !bytes.Equal(expected, actual) {
		t.Fatalf("bad: %q", string(actual))
	}
}

func TestParseSource(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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

func TestParseCacheControl(t *testing.T) {

	tests := []struct {
		name      string
		headerVal string
		want      structs.QueryOptions
		wantErr   bool
	}{
		{
			name:      "empty header",
			headerVal: "",
			want:      structs.QueryOptions{},
			wantErr:   false,
		},
		{
			name:      "simple max-age",
			headerVal: "max-age=30",
			want: structs.QueryOptions{
				MaxAge: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name:      "zero max-age",
			headerVal: "max-age=0",
			want: structs.QueryOptions{
				MustRevalidate: true,
			},
			wantErr: false,
		},
		{
			name:      "must-revalidate",
			headerVal: "must-revalidate",
			want: structs.QueryOptions{
				MustRevalidate: true,
			},
			wantErr: false,
		},
		{
			name:      "mixes age, must-revalidate",
			headerVal: "max-age=123, must-revalidate",
			want: structs.QueryOptions{
				MaxAge:         123 * time.Second,
				MustRevalidate: true,
			},
			wantErr: false,
		},
		{
			name:      "quoted max-age",
			headerVal: "max-age=\"30\"",
			want:      structs.QueryOptions{},
			wantErr:   true,
		},
		{
			name:      "mixed case max-age",
			headerVal: "Max-Age=30",
			want: structs.QueryOptions{
				MaxAge: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name:      "simple stale-if-error",
			headerVal: "stale-if-error=300",
			want: structs.QueryOptions{
				StaleIfError: 300 * time.Second,
			},
			wantErr: false,
		},
		{
			name:      "combined with space",
			headerVal: "max-age=30, stale-if-error=300",
			want: structs.QueryOptions{
				MaxAge:       30 * time.Second,
				StaleIfError: 300 * time.Second,
			},
			wantErr: false,
		},
		{
			name:      "combined no space",
			headerVal: "stale-IF-error=300,max-age=30",
			want: structs.QueryOptions{
				MaxAge:       30 * time.Second,
				StaleIfError: 300 * time.Second,
			},
			wantErr: false,
		},
		{
			name:      "unsupported directive",
			headerVal: "no-cache",
			want:      structs.QueryOptions{},
			wantErr:   false,
		},
		{
			name:      "mixed unsupported directive",
			headerVal: "no-cache, max-age=120",
			want: structs.QueryOptions{
				MaxAge: 120 * time.Second,
			},
			wantErr: false,
		},
		{
			name:      "garbage value",
			headerVal: "max-age=\"I'm not, an int\"",
			want:      structs.QueryOptions{},
			wantErr:   true,
		},
		{
			name:      "garbage value with quotes",
			headerVal: "max-age=\"I'm \\\"not an int\"",
			want:      structs.QueryOptions{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			r, _ := http.NewRequest("GET", "/foo/bar", nil)
			if tt.headerVal != "" {
				r.Header.Set("Cache-Control", tt.headerVal)
			}

			rr := httptest.NewRecorder()
			var got structs.QueryOptions

			failed := parseCacheControl(rr, r, &got)
			if tt.wantErr {
				require.True(t, failed)
				require.Equal(t, http.StatusBadRequest, rr.Code)
			} else {
				require.False(t, failed)
			}

			require.Equal(t, tt.want, got)
		})
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

func TestHTTPServer_PProfHandlers_EnableDebug(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, ``)
	defer a.Shutdown()

	resp := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/debug/pprof/profile?seconds=1", nil)

	a.config.EnableDebug = true
	httpServer := &HTTPHandlers{agent: a.Agent}
	httpServer.handler().ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
}

func TestHTTPServer_PProfHandlers_DisableDebugNoACLs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, ``)
	defer a.Shutdown()

	resp := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/debug/pprof/profile", nil)

	httpServer := &HTTPHandlers{agent: a.Agent}
	httpServer.handler().ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHTTPServer_PProfHandlers_ACLs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dc1 := "dc1"

	a := NewTestAgent(t, `
		primary_datacenter = "`+dc1+`"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
				agent = "agent"
				agent_recovery = "towel"
			}
		}

		enable_debug = false
	`)

	cases := []struct {
		code        int
		token       string
		endpoint    string
		nilResponse bool
	}{
		{
			code:        http.StatusOK,
			token:       "root",
			endpoint:    "/debug/pprof/heap",
			nilResponse: false,
		},
		{
			code:        http.StatusForbidden,
			token:       "agent",
			endpoint:    "/debug/pprof/heap",
			nilResponse: true,
		},
		{
			code:        http.StatusForbidden,
			token:       "agent",
			endpoint:    "/debug/pprof/",
			nilResponse: true,
		},
		{
			code:        http.StatusForbidden,
			token:       "",
			endpoint:    "/debug/pprof/",
			nilResponse: true,
		},
		{
			code:        http.StatusOK,
			token:       "root",
			endpoint:    "/debug/pprof/heap",
			nilResponse: false,
		},
		{
			code:        http.StatusForbidden,
			token:       "towel",
			endpoint:    "/debug/pprof/heap",
			nilResponse: true,
		},
	}

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d (%#v)", i, c), func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("%s?token=%s", c.endpoint, c.token), nil)
			resp := httptest.NewRecorder()
			a.config.EnableDebug = true
			a.srv.handler().ServeHTTP(resp, req)
			assert.Equal(t, c.code, resp.Code)
		})
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?stale", nil)
	a := NewTestAgent(t, "")
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
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
	a.config.DiscoveryMaxStale = 7 * time.Second
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?stale&consistent", nil)
	a := NewTestAgent(t, "")
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Check when no token is set
	a.tokens.UpdateUserToken("", tokenStore.TokenSourceConfig)
	a.srv.parseToken(req, &token)
	if token != "" {
		t.Fatalf("bad: %s", token)
	}

	// Check when ACLToken set
	a.tokens.UpdateUserToken("agent", tokenStore.TokenSourceAPI)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		ui_config {
			enabled = true
		}
	`)
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/ui/", nil)
	resp := httptest.NewRecorder()
	a.config.EnableDebug = true
	a.srv.handler().ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// Validate that it actually sent the index page we expect since an error
	// during serving the special intercepted index.html can result in an empty
	// response but a 200 status.
	require.Contains(t, resp.Body.String(), `<!-- CONSUL_VERSION:`)

	// Verify that we injected the variables we expected. The rest of injection
	// behavior is tested in the uiserver package, this just ensures it's plumbed
	// in correctly.
	require.NotContains(t, resp.Body.String(), `__RUNTIME_BOOL`)

	// Reload the config with changed metrics provider options and verify that
	// they are present in the output.
	newHCL := `
	data_dir = "` + a.DataDir + `"
	ui_config {
		enabled = true
		metrics_provider = "valid-but-unlikely-metrics-provider-name"
	}
	`
	c := TestConfig(testutil.Logger(t), config.FileSource{Name: t.Name(), Format: "hcl", Data: newHCL})
	require.NoError(t, a.reloadConfigInternal(c))

	// Now index requests should contain that metrics provider name.
	{
		req, _ := http.NewRequest("GET", "/ui/", nil)
		resp := httptest.NewRecorder()
		a.config.EnableDebug = true
		a.srv.handler().ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
		require.Contains(t, resp.Body.String(), `<!-- CONSUL_VERSION:`)
		require.Contains(t, resp.Body.String(), `valid-but-unlikely-metrics-provider-name`)
	}
}

func TestAllowedNets(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	type testVal struct {
		nets     []string
		ip       string
		expected bool
	}

	for _, v := range []testVal{
		{
			ip:       "156.124.222.351",
			expected: true,
		},
		{
			ip:       "[::2]",
			expected: true,
		},
		{
			nets:     []string{"0.0.0.0/0"},
			ip:       "115.124.32.64",
			expected: true,
		},
		{
			nets:     []string{"::0/0"},
			ip:       "[::3]",
			expected: true,
		},
		{
			nets:     []string{"127.0.0.1/8"},
			ip:       "127.0.0.1",
			expected: true,
		},
		{
			nets:     []string{"127.0.0.1/8"},
			ip:       "128.0.0.1",
			expected: false,
		},
		{
			nets:     []string{"::1/8"},
			ip:       "[::1]",
			expected: true,
		},
		{
			nets:     []string{"255.255.255.255/32"},
			ip:       "127.0.0.1",
			expected: false,
		},
		{
			nets:     []string{"255.255.255.255/32", "127.0.0.1/8"},
			ip:       "127.0.0.1",
			expected: true,
		},
	} {
		var nets []*net.IPNet
		for _, n := range v.nets {
			_, in, err := net.ParseCIDR(n)
			if err != nil {
				t.Fatal(err)
			}
			nets = append(nets, in)
		}

		a := NewTestAgent(t, "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		a.config.AllowWriteHTTPFrom = nets

		err := a.srv.checkWriteAccess(&http.Request{
			Method:     http.MethodPost,
			RemoteAddr: fmt.Sprintf("%s:16544", v.ip),
		})
		actual := err == nil

		if actual != v.expected {
			t.Fatalf("bad checkWriteAccess for values %+v, got %v", v, err)
		}

		if err != nil {
			if err, ok := err.(HTTPError); ok {
				if err.StatusCode != 403 {
					t.Fatalf("expected 403 but got %d", err.StatusCode)
				}
			} else {
				t.Fatalf("expected HTTP Error but got %v", err)
			}
		}

	}
}

// assertIndex tests that X-Consul-Index is set and non-zero
func assertIndex(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()
	require.NoError(t, checkIndex(resp))
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

func TestHTTPServer_HandshakeTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// Fire up an agent with TLS enabled.
	a := StartTestAgent(t, TestAgent{
		UseHTTPS: true,
		HCL: `
			key_file = "../test/client_certs/server.key"
			cert_file = "../test/client_certs/server.crt"
			ca_file = "../test/client_certs/rootca.crt"

			limits {
				https_handshake_timeout = "10ms"
			}
		`,
	})
	defer a.Shutdown()

	addr, err := firstAddr(a.Agent.apiServers, "https")
	require.NoError(t, err)
	// Connect to it with a plain TCP client that doesn't attempt to send HTTP or
	// complete a TLS handshake.
	conn, err := net.Dial("tcp", addr.String())
	require.NoError(t, err)
	defer conn.Close()

	// Wait for more than the timeout. This is timing dependent so could fail if
	// the CPU is super overloaded so the handler goroutine so I'm using a retry
	// loop below to be sure but this feels like a pretty generous margin for
	// error (10x the timeout and 100ms of scheduling time).
	time.Sleep(100 * time.Millisecond)

	// Set a read deadline on the Conn in case the timeout is not working we don't
	// want the read below to block forever. Needs to be much longer than what we
	// expect and the error should be different too.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	retry.Run(t, func(r *retry.R) {
		// Sanity check the conn was closed by attempting to read from it (a write
		// might not detect the close).
		buf := make([]byte, 10)
		_, err = conn.Read(buf)
		require.Error(r, err)
		require.Contains(r, err.Error(), "EOF")
	})
}

func TestRPC_HTTPSMaxConnsPerClient(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	cases := []struct {
		name       string
		tlsEnabled bool
	}{
		{"HTTP", false},
		{"HTTPS", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			hclPrefix := ""
			if tc.tlsEnabled {
				hclPrefix = `
				key_file = "../test/client_certs/server.key"
				cert_file = "../test/client_certs/server.crt"
				ca_file = "../test/client_certs/rootca.crt"
				`
			}

			// Fire up an agent with TLS enabled.
			a := StartTestAgent(t, TestAgent{
				UseHTTPS: tc.tlsEnabled,
				HCL: hclPrefix + `
					limits {
						http_max_conns_per_client = 2
					}
				`,
			})
			defer a.Shutdown()

			addr, err := firstAddr(a.Agent.apiServers, strings.ToLower(tc.name))
			require.NoError(t, err)

			assertConn := func(conn net.Conn, wantOpen bool) {
				retry.Run(t, func(r *retry.R) {
					// Don't wait around as server won't be sending data but the read will fail
					// immediately if the conn is closed.
					conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
					buf := make([]byte, 10)
					_, err := conn.Read(buf)
					require.Error(r, err)
					if wantOpen {
						require.Contains(r, err.Error(), "i/o timeout",
							"wanted an open conn (read timeout)")
					} else {
						require.Contains(r, err.Error(), "EOF", "wanted a closed conn")
					}
				})
			}

			// Connect to the server with bare TCP
			conn1, err := net.DialTimeout("tcp", addr.String(), time.Second)
			require.NoError(t, err)
			defer conn1.Close()

			assertConn(conn1, true)

			// Two conns should succeed
			conn2, err := net.DialTimeout("tcp", addr.String(), time.Second)
			require.NoError(t, err)
			defer conn2.Close()

			assertConn(conn2, true)

			// Third should succeed negotiating TCP handshake...
			conn3, err := net.DialTimeout("tcp", addr.String(), time.Second)
			require.NoError(t, err)
			defer conn3.Close()

			// But then be closed.
			assertConn(conn3, false)

			// Reload config with higher limit
			newCfg := *a.config
			newCfg.HTTPMaxConnsPerClient = 10
			require.NoError(t, a.reloadConfigInternal(&newCfg))

			// Now another conn should be allowed
			conn4, err := net.DialTimeout("tcp", addr.String(), time.Second)
			require.NoError(t, err)
			defer conn4.Close()

			assertConn(conn4, true)
		})
	}
}

func TestWithRemoteAddrHandler_ValidAddr(t *testing.T) {
	expected := net.TCPAddrFromAddrPort(netip.MustParseAddrPort("1.2.3.4:8080"))
	nextHandlerCalled := false

	assertionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		remoteAddr, ok := consul.RemoteAddrFromContext(r.Context())
		if !ok || remoteAddr.String() != expected.String() {
			t.Errorf("remote addr not present but expected %v", expected)
		}
	})

	remoteAddrHandler := withRemoteAddrHandler(assertionHandler)
	req := httptest.NewRequest("GET", "http://ignoreme", nil)
	req.RemoteAddr = expected.String()
	remoteAddrHandler.ServeHTTP(httptest.NewRecorder(), req)

	assert.True(t, nextHandlerCalled, "expected next handler to be called")
}

func TestWithRemoteAddrHandler_InvalidAddr(t *testing.T) {
	nextHandlerCalled := false

	assertionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		remoteAddr, ok := consul.RemoteAddrFromContext(r.Context())
		if ok || remoteAddr != nil {
			t.Errorf("remote addr %v present but not expected", remoteAddr)
		}
	})

	remoteAddrHandler := withRemoteAddrHandler(assertionHandler)
	req := httptest.NewRequest("GET", "http://ignoreme", nil)
	req.RemoteAddr = "i.am.not.a.valid.ipaddr:port"
	remoteAddrHandler.ServeHTTP(httptest.NewRecorder(), req)

	assert.True(t, nextHandlerCalled, "expected next handler to be called")
}
