// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestKVSEndpoint_PUT_GET_DELETE(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	keys := []string{
		"baz",
		"bar",
		"foo/sub1",
		"foo/sub2",
		"zip",
	}

	for _, key := range keys {
		buf := bytes.NewBuffer([]byte("test"))
		req, _ := http.NewRequest("PUT", "/v1/kv/"+key, buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); !res {
			t.Fatalf("should work")
		}
	}

	for _, key := range keys {
		req, _ := http.NewRequest("GET", "/v1/kv/"+key, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		assertIndex(t, resp)

		res, ok := obj.(structs.DirEntries)
		if !ok {
			t.Fatalf("should work")
		}

		if len(res) != 1 {
			t.Fatalf("bad: %v", res)
		}

		if res[0].Key != key {
			t.Fatalf("bad: %v", res)
		}
	}

	for _, key := range keys {
		req, _ := http.NewRequest("DELETE", "/v1/kv/"+key, nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.KVSEndpoint(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestKVSEndpoint_Recurse(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	keys := []string{
		"bar",
		"baz",
		"foo/sub1",
		"foo/sub2",
		"zip",
	}

	for _, key := range keys {
		buf := bytes.NewBuffer([]byte("test"))
		req, _ := http.NewRequest("PUT", "/v1/kv/"+key, buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); !res {
			t.Fatalf("should work")
		}
	}

	{
		// Get all the keys
		req, _ := http.NewRequest("GET", "/v1/kv/?recurse", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		assertIndex(t, resp)

		res, ok := obj.(structs.DirEntries)
		if !ok {
			t.Fatalf("should work")
		}

		if len(res) != len(keys) {
			t.Fatalf("bad: %v", res)
		}

		for idx, key := range keys {
			if res[idx].Key != key {
				t.Fatalf("bad: %v %v", res[idx].Key, key)
			}
		}
	}

	{
		req, _ := http.NewRequest("DELETE", "/v1/kv/?recurse", nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.KVSEndpoint(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	{
		// Get all the keys
		req, _ := http.NewRequest("GET", "/v1/kv/?recurse", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if obj != nil {
			t.Fatalf("bad: %v", obj)
		}
	}
}

func TestKVSEndpoint_DELETE_CAS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	{
		buf := bytes.NewBuffer([]byte("test"))
		req, _ := http.NewRequest("PUT", "/v1/kv/test", buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); !res {
			t.Fatalf("should work")
		}
	}

	req, _ := http.NewRequest("GET", "/v1/kv/test", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	d := obj.(structs.DirEntries)[0]

	// Create a CAS request, bad index
	{
		buf := bytes.NewBuffer([]byte("zip"))
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/v1/kv/test?cas=%d", d.ModifyIndex-1), buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); res {
			t.Fatalf("should NOT work")
		}
	}

	// Create a CAS request, good index
	{
		buf := bytes.NewBuffer([]byte("zip"))
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/v1/kv/test?cas=%d", d.ModifyIndex), buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); !res {
			t.Fatalf("should work")
		}
	}

	// Verify the delete
	req, _ = http.NewRequest("GET", "/v1/kv/test", nil)
	resp = httptest.NewRecorder()
	obj, _ = a.srv.KVSEndpoint(resp, req)
	if obj != nil {
		t.Fatalf("should be destroyed")
	}
}

func TestKVSEndpoint_CAS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	{
		buf := bytes.NewBuffer([]byte("test"))
		req, _ := http.NewRequest("PUT", "/v1/kv/test?flags=50", buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); !res {
			t.Fatalf("should work")
		}
	}

	req, _ := http.NewRequest("GET", "/v1/kv/test", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	d := obj.(structs.DirEntries)[0]

	// Check the flags
	if d.Flags != 50 {
		t.Fatalf("bad: %v", d)
	}

	// Create a CAS request, bad index
	{
		buf := bytes.NewBuffer([]byte("zip"))
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/kv/test?flags=42&cas=%d", d.ModifyIndex-1), buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); res {
			t.Fatalf("should NOT work")
		}
	}

	// Create a CAS request, good index
	{
		buf := bytes.NewBuffer([]byte("zip"))
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/kv/test?flags=42&cas=%d", d.ModifyIndex), buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); !res {
			t.Fatalf("should work")
		}
	}

	// Verify the update
	req, _ = http.NewRequest("GET", "/v1/kv/test", nil)
	resp = httptest.NewRecorder()
	obj, _ = a.srv.KVSEndpoint(resp, req)
	d = obj.(structs.DirEntries)[0]

	if d.Flags != 42 {
		t.Fatalf("bad: %v", d)
	}
	if string(d.Value) != "zip" {
		t.Fatalf("bad: %v", d)
	}
}

func TestKVSEndpoint_ListKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	keys := []string{
		"bar",
		"baz",
		"foo/sub1",
		"foo/sub2",
		"zip",
	}

	for _, key := range keys {
		buf := bytes.NewBuffer([]byte("test"))
		req, _ := http.NewRequest("PUT", "/v1/kv/"+key, buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); !res {
			t.Fatalf("should work")
		}
	}

	{
		// Get all the keys
		req, _ := http.NewRequest("GET", "/v1/kv/?keys&seperator=/", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		assertIndex(t, resp)

		res, ok := obj.([]string)
		if !ok {
			t.Fatalf("should work")
		}

		expect := []string{"bar", "baz", "foo/", "zip"}
		if !reflect.DeepEqual(res, expect) {
			t.Fatalf("bad: %v", res)
		}
	}
}

func TestKVSEndpoint_AcquireRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Acquire the lock
	id := makeTestSession(t, a.srv)
	req, _ := http.NewRequest("PUT", "/v1/kv/test?acquire="+id, bytes.NewReader(nil))
	resp := httptest.NewRecorder()
	obj, err := a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res := obj.(bool); !res {
		t.Fatalf("should work")
	}

	// Verify we have the lock
	req, _ = http.NewRequest("GET", "/v1/kv/test", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	d := obj.(structs.DirEntries)[0]

	// Check the flags
	if d.Session != id {
		t.Fatalf("bad: %v", d)
	}

	// Release the lock
	req, _ = http.NewRequest("PUT", "/v1/kv/test?release="+id, bytes.NewReader(nil))
	resp = httptest.NewRecorder()
	obj, err = a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res := obj.(bool); !res {
		t.Fatalf("should work")
	}

	// Verify we do not have the lock
	req, _ = http.NewRequest("GET", "/v1/kv/test", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	d = obj.(structs.DirEntries)[0]

	// Check the flags
	if d.Session != "" {
		t.Fatalf("bad: %v", d)
	}
}

func TestKVSEndpoint_GET_Raw(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	buf := bytes.NewBuffer([]byte("test"))
	req, _ := http.NewRequest("PUT", "/v1/kv/test", buf)
	resp := httptest.NewRecorder()
	obj, err := a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res := obj.(bool); !res {
		t.Fatalf("should work")
	}

	req, _ = http.NewRequest("GET", "/v1/kv/test?raw", nil)
	resp = httptest.NewRecorder()
	_, err = a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Check the headers
	contentTypeHdr := resp.Header().Values("Content-Type")
	if len(contentTypeHdr) != 1 {
		t.Fatalf("expected 1 value for Content-Type header, got %d: %+v", len(contentTypeHdr), contentTypeHdr)
	}
	if contentTypeHdr[0] != "text/plain" {
		t.Fatalf("expected Content-Type header to be \"text/plain\", got %q", contentTypeHdr[0])
	}

	optionsHdr := resp.Header().Values("X-Content-Type-Options")
	if len(optionsHdr) != 1 {
		t.Fatalf("expected 1 value for X-Content-Type-Options header, got %d: %+v", len(optionsHdr), optionsHdr)
	}
	if optionsHdr[0] != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options header to be \"nosniff\", got %q", optionsHdr[0])
	}

	cspHeader := resp.Header().Values("Content-Security-Policy")
	if len(cspHeader) != 1 {
		t.Fatalf("expected 1 value for Content-Security-Policy header, got %d: %+v", len(optionsHdr), optionsHdr)
	}
	if cspHeader[0] != "sandbox" {
		t.Fatalf("expected X-Content-Type-Options header to be \"sandbox\", got %q", optionsHdr[0])
	}

	// Check the body
	if !bytes.Equal(resp.Body.Bytes(), []byte("test")) {
		t.Fatalf("bad: %s", resp.Body.Bytes())
	}
}

func TestKVSEndpoint_PUT_ConflictingFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	req, _ := http.NewRequest("PUT", "/v1/kv/test?cas=0&acquire=xxx", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.KVSEndpoint(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}

	if resp.Code != 400 {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("Conflicting")) {
		t.Fatalf("expected conflicting args error")
	}
}

func TestKVSEndpoint_GET(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	buf := bytes.NewBuffer([]byte("test"))
	req, _ := http.NewRequest("PUT", "/v1/kv/test", buf)
	resp := httptest.NewRecorder()
	obj, err := a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res := obj.(bool); !res {
		t.Fatalf("should work")
	}

	req, _ = http.NewRequest("GET", "/v1/kv/test", nil)
	resp = httptest.NewRecorder()
	_, err = a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// The following headers are only included when returning a raw KV response

	contentTypeHdr := resp.Header().Values("Content-Type")
	if len(contentTypeHdr) != 0 {
		t.Fatalf("expected no Content-Type header, got %d: %+v", len(contentTypeHdr), contentTypeHdr)
	}

	optionsHdr := resp.Header().Values("X-Content-Type-Options")
	if len(optionsHdr) != 0 {
		t.Fatalf("expected no X-Content-Type-Options header, got %d: %+v", len(optionsHdr), optionsHdr)
	}

	cspHeader := resp.Header().Values("Content-Security-Policy")
	if len(cspHeader) != 0 {
		t.Fatalf("expected no Content-Security-Policy header, got %d: %+v", len(optionsHdr), optionsHdr)
	}
}

func TestKVSEndpoint_DELETE_ConflictingFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	req, _ := http.NewRequest("DELETE", "/v1/kv/test?recurse&cas=0", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.KVSEndpoint(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}

	if resp.Code != 400 {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("Conflicting")) {
		t.Fatalf("expected conflicting args error")
	}
}

// TestKVSEndpoint_SecurityValidation tests comprehensive security validation
// for path traversal attacks, file extension abuse, and endpoint forwarding prevention
func TestKVSEndpoint_SecurityValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testCases := []struct {
		name        string
		key         string
		method      string
		body        string
		expectError bool
		statusCode  int
		description string
	}{
		// Valid keys (should succeed)
		{
			name:        "valid nested key",
			key:         "app/config/database",
			method:      "PUT",
			body:        "value",
			expectError: false,
			description: "Valid nested key should work",
		},
		{
			name:        "valid key with allowed characters",
			key:         "service-v2_cache.123/prod",
			method:      "PUT",
			body:        "value",
			expectError: false,
			description: "Valid key with allowed characters ,-_./ should work",
		},
		{
			name:        "valid key with comma",
			key:         "env,dev,staging",
			method:      "PUT",
			body:        "value",
			expectError: false,
			description: "Valid key with comma should work",
		},

		// Invalid character attacks (should be blocked)
		{
			name:        "key with question mark",
			key:         "app/config?test",
			method:      "PUT",
			body:        "value",
			expectError: false, // Query param is parsed separately, key is "app/config" which is valid
			description: "Key with question mark - query part parsed separately, key is valid",
		},
		{
			name:        "key with equals sign",
			key:         "app=config",
			method:      "PUT",
			body:        "value",
			expectError: true,
			statusCode:  400,
			description: "Key with equals sign should be blocked",
		},

		// Path traversal attacks (should be blocked)
		{
			name:        "basic path traversal",
			key:         "../admin",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Basic path traversal should be blocked",
		},
		{
			name:        "deep path traversal",
			key:         "../../config",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Deep path traversal should be blocked",
		},
		{
			name:        "mid-path traversal",
			key:         "app/../admin",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Mid-path traversal should be blocked",
		},
		{
			name:        "windows path traversal",
			key:         "..\\windows\\system32",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Windows-style path traversal should be blocked",
		},

		// URL encoded path traversal attacks (should be blocked)
		{
			name:        "URL encoded full traversal",
			key:         "%2e%2e%2fadmin",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "URL encoded path traversal should be blocked",
		},
		{
			name:        "URL encoded partial traversal",
			key:         "%2e%2e/secret",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Partially encoded path traversal should be blocked",
		},
		{
			name:        "mixed encoding traversal",
			key:         "..%2fconfig",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Mixed encoding path traversal should be blocked",
		},
		{
			name:        "double URL encoding",
			key:         "%252e%252e%252fadmin",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Double URL encoded traversal should be blocked",
		},

		// File extension attacks (should be blocked for security)
		{
			name:        "javascript extension",
			key:         "config.js",
			method:      "PUT",
			body:        "alert('xss')",
			expectError: true,
			statusCode:  400,
			description: "JavaScript file extension should be blocked",
		},
		{
			name:        "CSS extension",
			key:         "styles.css",
			method:      "PUT",
			body:        "body{display:none}",
			expectError: true,
			statusCode:  400,
			description: "CSS file extension should be blocked",
		},
		{
			name:        "HTML extension",
			key:         "admin.html",
			method:      "PUT",
			body:        "<script>alert('xss')</script>",
			expectError: true,
			statusCode:  400,
			description: "HTML file extension should be blocked",
		},
		{
			name:        "PHP extension",
			key:         "backdoor.php",
			method:      "PUT",
			body:        "<?php system($_GET['cmd']); ?>",
			expectError: true,
			statusCode:  400,
			description: "PHP file extension should be blocked",
		},
		{
			name:        "executable extension",
			key:         "malware.exe",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Executable file extension should be blocked",
		},
		{
			name:        "PDF extension",
			key:         "document.pdf",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "PDF file extension should be blocked",
		},

		// Case sensitivity tests (should be blocked)
		{
			name:        "uppercase JS extension",
			key:         "SCRIPT.JS",
			method:      "PUT",
			body:        "alert('xss')",
			expectError: true,
			statusCode:  400,
			description: "Uppercase JavaScript extension should be blocked",
		},
		{
			name:        "mixed case CSS extension",
			key:         "Style.CSS",
			method:      "PUT",
			body:        "body{display:none}",
			expectError: true,
			statusCode:  400,
			description: "Mixed case CSS extension should be blocked",
		},

		// Nested file extension attacks (should be blocked)
		{
			name:        "nested JavaScript file",
			key:         "app/config/main.js",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Nested JavaScript file should be blocked",
		},
		{
			name:        "deep nested PHP file",
			key:         "admin/secret/backdoor.php",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Deep nested PHP file should be blocked",
		},

		// Query parameter cache deception (should be blocked due to file extension detection)
		{
			name:        "JS with version parameter",
			key:         "script.js?v=1.0",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "JavaScript with query parameter should be blocked due to file extension",
		},
		{
			name:        "CSS with hash parameter",
			key:         "styles.css?hash=abc123",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "CSS with query parameter should be blocked due to file extension",
		},

		// Leading slash tests (should be blocked)
		{
			name:        "leading slash",
			key:         "/admin",
			method:      "PUT",
			body:        "malicious",
			expectError: true,
			statusCode:  400,
			description: "Leading slash should be blocked",
		},

		// Cross-method testing (should be blocked due to file extensions)
		{
			name:        "GET file extension attack",
			key:         "config.js",
			method:      "GET",
			body:        "",
			expectError: true,
			statusCode:  400,
			description: "GET request for file extension should be blocked",
		},
		{
			name:        "DELETE file extension attack",
			key:         "admin.php",
			method:      "DELETE",
			body:        "",
			expectError: true,
			statusCode:  400,
			description: "DELETE request for file extension should be blocked",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body != "" {
				req, _ = http.NewRequest(tc.method, "/v1/kv/"+tc.key, bytes.NewBuffer([]byte(tc.body)))
			} else {
				req, _ = http.NewRequest(tc.method, "/v1/kv/"+tc.key, nil)
			}
			resp := httptest.NewRecorder()

			_, err := a.srv.KVSEndpoint(resp, req)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.description)
					return
				}

				httpErr, ok := err.(HTTPError)
				if !ok {
					t.Errorf("Expected HTTPError for %s, got %T", tc.description, err)
					return
				}

				if httpErr.StatusCode != tc.statusCode {
					t.Errorf("Expected status %d for %s, got %d", tc.statusCode, tc.description, httpErr.StatusCode)
					return
				}

				t.Logf("✓ Correctly blocked: %s (HTTP %d)", tc.description, httpErr.StatusCode)
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got: %v", tc.description, err)
				} else {
					t.Logf("✓ Correctly allowed: %s", tc.description)
				}
			}
		})
	}
}

// TestKVSEndpoint_RawPathTraversalBlocking tests that path traversal in raw URL paths
// is blocked before normalization occurs, preventing forwarding to unintended endpoints
func TestKVSEndpoint_RawPathTraversalBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testCases := []struct {
		name           string
		rawPath        string
		expectedStatus int
		description    string
	}{
		{
			name:           "Basic path traversal in URL",
			rawPath:        "/v1/kv/../admin",
			expectedStatus: 400,
			description:    "Should block before normalization prevents forwarding to /admin",
		},
		{
			name:           "Deep path traversal in URL",
			rawPath:        "/v1/kv/../../config",
			expectedStatus: 400,
			description:    "Should block before normalization prevents forwarding to /config",
		},
		{
			name:           "Windows path traversal in URL",
			rawPath:        "/v1/kv/..\\admin",
			expectedStatus: 400,
			description:    "Should block Windows-style traversal in URL path",
		},
		{
			name:           "URL encoded traversal",
			rawPath:        "/v1/kv/%2e%2e%2fadmin",
			expectedStatus: 400,
			description:    "Should block URL encoded path traversal",
		},
		{
			name:           "Partial URL encoding",
			rawPath:        "/v1/kv/%2e%2e/secret",
			expectedStatus: 400,
			description:    "Should block partially encoded traversal",
		},
		{
			name:           "Mixed encoding",
			rawPath:        "/v1/kv/..%2fconfig",
			expectedStatus: 400,
			description:    "Should block mixed encoding traversal",
		},
		{
			name:           "Double URL encoding",
			rawPath:        "/v1/kv/%252e%252e%252f",
			expectedStatus: 400,
			description:    "Should block double URL encoded traversal",
		},
		{
			name:           "Valid KV path",
			rawPath:        "/v1/kv/app/config",
			expectedStatus: 200,
			description:    "Should allow legitimate KV paths",
		},
		{
			name:           "Valid nested path",
			rawPath:        "/v1/kv/service/database/password",
			expectedStatus: 200,
			description:    "Should allow valid nested paths",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request with specific raw path
			req := httptest.NewRequest("PUT", tc.rawPath, strings.NewReader("test-value"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp := httptest.NewRecorder()

			// Call the KVS endpoint
			_, err := a.srv.KVSEndpoint(resp, req)

			if tc.expectedStatus == 400 {
				// Should return HTTP error for blocked requests
				if err == nil {
					t.Errorf("Expected HTTP error for path %s, but got none", tc.rawPath)
					return
				}

				httpErr, ok := err.(HTTPError)
				if !ok {
					t.Errorf("Expected HTTPError for path %s, got %T", tc.rawPath, err)
					return
				}

				if httpErr.StatusCode != tc.expectedStatus {
					t.Errorf("Expected status %d for path %s, got %d", tc.expectedStatus, tc.rawPath, httpErr.StatusCode)
				}

				// Verify error message mentions traversal
				if !strings.Contains(httpErr.Reason, "traversal") {
					t.Errorf("Expected error message to mention 'traversal' for path %s, got: %s", tc.rawPath, httpErr.Reason)
				}

				t.Logf("✓ Correctly blocked %s: %s", tc.rawPath, httpErr.Reason)

			} else {
				// Should succeed for valid requests
				if err != nil {
					t.Errorf("Expected no error for valid path %s, got: %v", tc.rawPath, err)
				} else {
					t.Logf("✓ Correctly allowed %s", tc.rawPath)
				}
			}
		})
	}
}

// TestKVSEndpoint_PreventEndpointForwarding specifically tests that path traversal
// cannot be used to access other Consul API endpoints
func TestKVSEndpoint_PreventEndpointForwarding(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// These paths could potentially be used to access other Consul endpoints
	// if path traversal forwarding is not properly blocked
	maliciousPaths := []string{
		"/v1/kv/../agent/members",    // Attempt to access agent members API
		"/v1/kv/../../health/checks", // Attempt to access health checks API
		"/v1/kv/../catalog/services", // Attempt to access catalog services API
		"/v1/kv/../../acl/tokens",    // Attempt to access ACL tokens API
		"/v1/kv/../status/leader",    // Attempt to access status API
	}

	for _, path := range maliciousPaths {
		t.Run("Block_"+path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			resp := httptest.NewRecorder()

			_, err := a.srv.KVSEndpoint(resp, req)

			// Should be blocked with 400 error
			if err == nil {
				t.Errorf("Expected path traversal to be blocked for %s, but request succeeded", path)
				return
			}

			httpErr, ok := err.(HTTPError)
			if !ok || httpErr.StatusCode != 400 {
				t.Errorf("Expected HTTP 400 error for %s, got %v", path, err)
				return
			}

			t.Logf("✓ Successfully blocked potential endpoint forwarding: %s", path)
		})
	}
}
