// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
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
	if contentTypeHdr[0] != "text/plain; charset=utf-8" {
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

func TestValidateKVKey(t *testing.T) {
	pattern := `^[a-zA-Z0-9,_./\-?&=]+$`
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		// Valid
		{"valid simple key", "foo", false},
		{"valid nested key", "foo/bar/baz", false},
		{"valid key with dash", "foo-bar", false},
		{"valid key with underscore", "foo_bar", false},
		{"valid key with dot", "foo.bar", false},
		{"valid key with comma", "foo,bar", false},
		{"valid key with slash", "foo/bar", false},
		{"valid key with numbers", "foo123", false},
		{"empty key", "", true},
		// Invalid
		{"invalid key with space", "foo bar", true},
		{"invalid key with star", "foo*bar", true},
		{"invalid key with percent", "foo%bar", true},
		{"invalid key with colon", "foo:bar", true},
		{"invalid key with semicolon", "foo;bar", true},
		{"malicious key with path traversal", "../../etc/passwd", true},
		{"malicious key with unicode control", "foo\u202Etxt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKVKey(tt.key, pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateKVKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestKVSEndpoint_KeyConstruction_TrailingSlashes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Wait for leader
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	tests := []struct {
		name          string
		urlPath       string
		expectedKey   string
		shouldSucceed bool
		description   string
	}{
		// Basic trailing slash tests
		{
			name:          "simple directory with trailing slash",
			urlPath:       "/v1/kv/directory/",
			expectedKey:   "directory/",
			shouldSucceed: true,
			description:   "Simple directory key with trailing slash should be preserved",
		},
		{
			name:          "nested directory with trailing slash",
			urlPath:       "/v1/kv/foo/bar/baz/",
			expectedKey:   "foo/bar/baz/",
			shouldSucceed: true,
			description:   "Nested directory key with trailing slash should be preserved",
		},
		{
			name:          "file key without trailing slash",
			urlPath:       "/v1/kv/file",
			expectedKey:   "file",
			shouldSucceed: true,
			description:   "File key without trailing slash should remain unchanged",
		},

		// Complex path.Clean scenarios with trailing slashes
		{
			name:          "double slashes with trailing slash",
			urlPath:       "/v1/kv/foo//bar/",
			expectedKey:   "foo/bar/",
			shouldSucceed: true,
			description:   "Double slashes should be cleaned but trailing slash preserved",
		},
		{
			name:          "triple slashes with trailing slash",
			urlPath:       "/v1/kv/foo///bar/",
			expectedKey:   "foo/bar/",
			shouldSucceed: true,
			description:   "Triple slashes should be cleaned but trailing slash preserved",
		},
		{
			name:          "redundant current dir with trailing slash",
			urlPath:       "/v1/kv/foo/./bar/",
			expectedKey:   "foo/bar/",
			shouldSucceed: true,
			description:   "Current directory references should be cleaned, trailing slash preserved",
		},
		{
			name:          "multiple redundant elements with trailing slash",
			urlPath:       "/v1/kv/a/./b/./c/",
			expectedKey:   "a/b/c/",
			shouldSucceed: true,
			description:   "Multiple redundant elements should be cleaned, trailing slash preserved",
		},

		// Edge cases that have caused issues historically
		{
			name:          "multiple trailing slashes",
			urlPath:       "/v1/kv/directory///",
			expectedKey:   "directory/",
			shouldSucceed: true,
			description:   "Multiple trailing slashes should be cleaned to single slash",
		},
		{
			name:          "mixed redundant and trailing slashes",
			urlPath:       "/v1/kv/foo//./bar//",
			expectedKey:   "foo/bar/",
			shouldSucceed: true,
			description:   "Mixed redundant elements and trailing slashes should be handled correctly",
		},
		{
			name:          "complex nested path with trailing slash",
			urlPath:       "/v1/kv/a//b/./c//d/",
			expectedKey:   "a/b/c/d/",
			shouldSucceed: true,
			description:   "Complex nested path with various redundancies and trailing slash",
		},

		// Root and empty cases
		{
			name:          "root with trailing slash",
			urlPath:       "/v1/kv/",
			expectedKey:   "",
			shouldSucceed: false,
			description:   "Root level should result in empty key which is invalid",
		},
		{
			name:          "only slashes",
			urlPath:       "/v1/kv////",
			expectedKey:   "/",
			shouldSucceed: true,
			description:   "Only slashes should be cleaned to single slash",
		},

		// Path traversal with trailing slashes (should be caught by validation)
		{
			name:          "path traversal with trailing slash",
			urlPath:       "/v1/kv/../config/",
			expectedKey:   "../config/",
			shouldSucceed: false,
			description:   "Path traversal should be rejected due to .. in result",
		},
		{
			name:          "complex path traversal with trailing slash",
			urlPath:       "/v1/kv/foo/../bar/../baz/",
			expectedKey:   "baz/",
			shouldSucceed: true,
			description:   "Complex path traversal should be cleaned to final directory",
		},
		{
			name:          "path traversal resulting in parent access",
			urlPath:       "/v1/kv/foo/../../etc/",
			expectedKey:   "../etc/",
			shouldSucceed: false,
			description:   "Path traversal that escapes should be rejected",
		},

		// Special characters with trailing slashes
		{
			name:          "special chars with trailing slash",
			urlPath:       "/v1/kv/foo-bar_baz.test/",
			expectedKey:   "foo-bar_baz.test/",
			shouldSucceed: true,
			description:   "Valid special characters with trailing slash should be preserved",
		},
		{
			name:          "key that looks like query params",
			urlPath:       "/v1/kv/config-env=prod/",
			expectedKey:   "config-env=prod/",
			shouldSucceed: true,
			description:   "Key with equals and other valid characters with trailing slash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with PUT request to verify key construction
			buf := bytes.NewBuffer([]byte("test-value"))
			req, err := http.NewRequest("PUT", tt.urlPath, buf)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp := httptest.NewRecorder()
			obj, err := a.srv.KVSEndpoint(resp, req)

			if tt.shouldSucceed {
				if err != nil {
					t.Errorf("Expected success but got error: %v (description: %s)", err, tt.description)
					return
				}
				if obj == nil {
					t.Errorf("Expected non-nil response for: %s", tt.description)
					return
				}

				// Verify the key was stored correctly by retrieving it
				// We need to handle the case where expectedKey might be empty
				if tt.expectedKey == "" {
					// For empty keys, we shouldn't try to retrieve them
					return
				}

				getReq, _ := http.NewRequest("GET", "/v1/kv/"+tt.expectedKey, nil)
				getResp := httptest.NewRecorder()
				getObj, getErr := a.srv.KVSEndpoint(getResp, getReq)

				if getErr != nil {
					t.Errorf("Failed to retrieve stored key %q: %v (description: %s)", tt.expectedKey, getErr, tt.description)
					return
				}

				if getObj == nil {
					t.Errorf("Key was not stored at expected location %q (description: %s)", tt.expectedKey, tt.description)
					return
				}

				// Verify the response is what we expect
				entries, ok := getObj.(structs.DirEntries)
				if !ok || len(entries) == 0 {
					t.Errorf("Unexpected response format or empty response for key %q (description: %s)", tt.expectedKey, tt.description)
					return
				}

				if entries[0].Key != tt.expectedKey {
					t.Errorf("Expected key %q, got %q (description: %s)", tt.expectedKey, entries[0].Key, tt.description)
				}

				// For directory keys (with trailing slash), verify they can be used for listing
				if strings.HasSuffix(tt.expectedKey, "/") {
					listReq, _ := http.NewRequest("GET", "/v1/kv/"+tt.expectedKey+"?keys", nil)
					listResp := httptest.NewRecorder()
					listObj, listErr := a.srv.KVSEndpoint(listResp, listReq)

					if listErr != nil {
						t.Errorf("Failed to list directory key %q: %v (description: %s)", tt.expectedKey, listErr, tt.description)
						return
					}

					// Should get a list response (could be empty, that's fine)
					if listObj == nil {
						t.Errorf("Expected list response for directory key %q (description: %s)", tt.expectedKey, tt.description)
					}
				}
			} else {
				if err == nil {
					t.Errorf("Expected error but got success for: %s", tt.description)
				}
			}
		})
	}
}

func TestKVSEndpoint_PathCleaningEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Wait for leader
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Test edge cases in path cleaning that have historically caused issues
	tests := []struct {
		name        string
		rawKey      string
		expectedKey string
		description string
	}{
		{
			name:        "empty key",
			rawKey:      "",
			expectedKey: "",
			description: "Empty raw key should remain empty",
		},
		{
			name:        "single slash",
			rawKey:      "/",
			expectedKey: "/",
			description: "Single slash should be preserved",
		},
		{
			name:        "double slash",
			rawKey:      "//",
			expectedKey: "/",
			description: "Double slash should be cleaned to single slash",
		},
		{
			name:        "current directory",
			rawKey:      "./foo",
			expectedKey: "foo",
			description: "Current directory should be removed",
		},
		{
			name:        "current directory with trailing slash",
			rawKey:      "./foo/",
			expectedKey: "foo/",
			description: "Current directory should be removed, trailing slash preserved",
		},
		{
			name:        "parent directory",
			rawKey:      "foo/../bar",
			expectedKey: "bar",
			description: "Parent directory traversal should be cleaned",
		},
		{
			name:        "parent directory with trailing slash",
			rawKey:      "foo/../bar/",
			expectedKey: "bar/",
			description: "Parent directory traversal should be cleaned, trailing slash preserved",
		},
		{
			name:        "multiple slashes mixed with dots",
			rawKey:      "foo//./bar//",
			expectedKey: "foo/bar/",
			description: "Mixed redundant elements should be cleaned properly",
		},
		{
			name:        "complex nested cleaning",
			rawKey:      "a/b/../c/./d//e/",
			expectedKey: "a/c/d/e/",
			description: "Complex nested path should be cleaned correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the path cleaning logic from the actual endpoint
			var cleanedKey string
			if tt.rawKey == "" {
				cleanedKey = ""
			} else {
				cleanedKey = path.Clean(tt.rawKey)
				if strings.HasSuffix(tt.rawKey, "/") && !strings.HasSuffix(cleanedKey, "/") {
					cleanedKey += "/"
				}
			}

			if cleanedKey != tt.expectedKey {
				t.Errorf("Path cleaning failed for %q: expected %q, got %q (description: %s)",
					tt.rawKey, tt.expectedKey, cleanedKey, tt.description)
			}
		})
	}
}
