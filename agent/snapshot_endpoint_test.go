// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/testrpc"
)

func TestSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	var snap io.Reader
	t.Run("create snapshot", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/snapshot", body)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		if _, err := a.srv.Snapshot(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		snap = resp.Body

		header := resp.Header().Get("X-Consul-Index")
		if header == "" {
			t.Fatalf("bad: %v", header)
		}
		header = resp.Header().Get("X-Consul-KnownLeader")
		if header != "true" {
			t.Fatalf("bad: %v", header)
		}
		header = resp.Header().Get("X-Consul-LastContact")
		if header != "0" {
			t.Fatalf("bad: %v", header)
		}
	})

	t.Run("restore snapshot", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		req, _ := http.NewRequest("PUT", "/v1/snapshot", snap)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		if _, err := a.srv.Snapshot(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestSnapshot_Options(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, method := range []string{"GET", "PUT"} {
		t.Run(method, func(t *testing.T) {
			a := NewTestAgent(t, TestACLConfig())
			defer a.Shutdown()

			body := bytes.NewBuffer(nil)
			req, _ := http.NewRequest(method, "/v1/snapshot", body)
			req.Header.Add("X-Consul-Token", "anonymous")
			resp := httptest.NewRecorder()
			_, err := a.srv.Snapshot(resp, req)
			if !acl.IsErrPermissionDenied(err) {
				t.Fatalf("err: %v", err)
			}
		})

		t.Run(method, func(t *testing.T) {
			a := NewTestAgent(t, TestACLConfig())
			defer a.Shutdown()

			body := bytes.NewBuffer(nil)
			req, _ := http.NewRequest(method, "/v1/snapshot?dc=nope", body)
			resp := httptest.NewRecorder()
			_, err := a.srv.Snapshot(resp, req)
			if err == nil || !strings.Contains(err.Error(), "No path to datacenter") {
				t.Fatalf("err: %v", err)
			}
		})

		t.Run(method, func(t *testing.T) {
			a := NewTestAgent(t, TestACLConfig())
			defer a.Shutdown()

			body := bytes.NewBuffer(nil)
			req, _ := http.NewRequest(method, "/v1/snapshot?stale", body)
			req.Header.Add("X-Consul-Token", "root")
			resp := httptest.NewRecorder()
			_, err := a.srv.Snapshot(resp, req)
			if method == "GET" {
				if err != nil {
					t.Fatalf("err: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "stale not allowed") {
					t.Fatalf("err: %v", err)
				}
			}
		})
	}
}
