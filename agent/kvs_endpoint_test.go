package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
)

func TestKVSEndpoint_PUT_GET_DELETE(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	obj, err = a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Check the body
	if !bytes.Equal(resp.Body.Bytes(), []byte("test")) {
		t.Fatalf("bad: %s", resp.Body.Bytes())
	}
}

func TestKVSEndpoint_PUT_ConflictingFlags(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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

func TestKVSEndpoint_DELETE_ConflictingFlags(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
