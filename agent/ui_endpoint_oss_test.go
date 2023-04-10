// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestUIEndpoint_MetricsProxy_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	var (
		lastHeadersSent atomic.Value
		backendCalled   atomic.Value
	)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled.Store(true)
		lastHeadersSent.Store(r.Header)
		if r.URL.Path == "/some/prefix/ok" {
			w.Write([]byte("OK"))
			return
		}
		http.Error(w, "not found on backend", http.StatusNotFound)
	}))
	defer backend.Close()

	backendURL := backend.URL + "/some/prefix"

	a := NewTestAgent(t, TestACLConfig()+fmt.Sprintf(`
		ui_config {
			enabled = true
			metrics_proxy {
				base_url = %q
			}
		}
		http_config {
			response_headers {
				"Access-Control-Allow-Origin" = "*"
			}
		}
	`, backendURL))
	defer a.Shutdown()

	h := a.srv.handler(true)

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	const endpointPath = "/v1/internal/ui/metrics-proxy"

	// create some ACL things
	for name, rules := range map[string]string{
		"one-service":  `service "foo" { policy = "read" }`,
		"all-services": `service_prefix "" { policy = "read" }`,
		"one-node":     `node "bar" { policy = "read" }`,
		"all-nodes":    `node_prefix "" { policy = "read" }`,
	} {
		req := structs.ACLPolicySetRequest{
			Policy: structs.ACLPolicy{
				Name:  name,
				Rules: rules,
			},
			Datacenter:   "dc1",
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var policy structs.ACLPolicy
		require.NoError(t, a.RPC(context.Background(), "ACL.PolicySet", &req, &policy))
	}

	makeToken := func(t *testing.T, policyNames []string) string {
		req := structs.ACLTokenSetRequest{
			ACLToken:     structs.ACLToken{},
			Datacenter:   "dc1",
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		for _, name := range policyNames {
			req.ACLToken.Policies = append(req.ACLToken.Policies, structs.ACLTokenPolicyLink{Name: name})
		}
		require.Len(t, req.ACLToken.Policies, len(policyNames))

		var token structs.ACLToken
		require.NoError(t, a.RPC(context.Background(), "ACL.TokenSet", &req, &token))
		return token.SecretID
	}

	type testcase struct {
		name     string
		token    string
		policies []string
		expect   int
	}

	for _, tc := range []testcase{
		{name: "no token", token: "", expect: http.StatusForbidden},
		{name: "root token", token: "root", expect: http.StatusOK},
		//
		{name: "one node", policies: []string{"one-node"}, expect: http.StatusForbidden},
		{name: "all nodes", policies: []string{"all-nodes"}, expect: http.StatusForbidden},
		//
		{name: "one service", policies: []string{"one-service"}, expect: http.StatusForbidden},
		{name: "all services", policies: []string{"all-services"}, expect: http.StatusForbidden},
		//
		{name: "one service one node", policies: []string{"one-service", "one-node"}, expect: http.StatusForbidden},
		{name: "all services one node", policies: []string{"all-services", "one-node"}, expect: http.StatusForbidden},
		//
		{name: "one service all nodes", policies: []string{"one-service", "one-node"}, expect: http.StatusForbidden},
		{name: "all services all nodes", policies: []string{"all-services", "all-nodes"}, expect: http.StatusOK},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.token == "" {
				tc.token = makeToken(t, tc.policies)
			}

			t.Run("via query param should not work", func(t *testing.T) {
				req := httptest.NewRequest("GET", endpointPath+"/ok?token="+tc.token, nil)
				rec := httptest.NewRecorder()
				backendCalled.Store(false)
				h.ServeHTTP(rec, req)
				require.Equal(t, http.StatusForbidden, rec.Code)

				require.False(t, backendCalled.Load().(bool))
			})

			for _, headerName := range []string{"x-consul-token", "authorization"} {
				headerVal := tc.token
				if headerName == "authorization" {
					headerVal = "bearer " + tc.token
				}

				t.Run("via header "+headerName, func(t *testing.T) {
					req := httptest.NewRequest("GET", endpointPath+"/ok", nil)
					req.Header.Set(headerName, headerVal)
					rec := httptest.NewRecorder()
					backendCalled.Store(false)
					h.ServeHTTP(rec, req)
					require.Equal(t, tc.expect, rec.Code)

					headersSent, _ := lastHeadersSent.Load().(http.Header)
					if tc.expect == http.StatusOK {
						require.True(t, backendCalled.Load().(bool))
						// Ensure we didn't accidentally ship our consul token to the proxy.
						require.Empty(t, headersSent.Get("X-Consul-Token"))
						require.Empty(t, headersSent.Get("Authorization"))
					} else {
						require.False(t, backendCalled.Load().(bool))
					}
				})
			}
		})
	}
}
