// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/armon/go-metrics"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/version"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/serf/serf"
	"github.com/mitchellh/hashstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/debug"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func createACLTokenWithAgentReadPolicy(t *testing.T, srv *HTTPHandlers) string {
	policyReq := &structs.ACLPolicy{
		Name:  "agent-read",
		Rules: `agent_prefix "" { policy = "read" }`,
	}

	req, _ := http.NewRequest("PUT", "/v1/acl/policy", jsonReader(policyReq))
	req.Header.Add("X-Consul-Token", "root")
	resp := httptest.NewRecorder()
	srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	tokenReq := &structs.ACLToken{
		Description: "agent-read-token-for-test",
		Policies:    []structs.ACLTokenPolicyLink{{Name: "agent-read"}},
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/token", jsonReader(tokenReq))
	req.Header.Add("X-Consul-Token", "root")
	resp = httptest.NewRecorder()
	srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	svcToken := &structs.ACLToken{}
	dec := json.NewDecoder(resp.Body)
	err := dec.Decode(svcToken)
	require.NoError(t, err)
	return svcToken.SecretID
}

func TestAgent_Services(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"primary"},
		Meta: map[string]string{
			"foo": "bar",
		},
		Port: 5000,
	}
	require.NoError(t, a.State.AddServiceWithChecks(srv1, nil, "", false))

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	decoder := json.NewDecoder(resp.Body)
	var val map[string]*api.AgentService
	err := decoder.Decode(&val)
	require.NoError(t, err)
	assert.Lenf(t, val, 1, "bad services: %v", val)
	assert.Equal(t, 5000, val["mysql"].Port)
	assert.Equal(t, srv1.Meta, val["mysql"].Meta)
}

func TestAgent_ServicesFiltered(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"primary"},
		Meta: map[string]string{
			"foo": "bar",
		},
		Port: 5000,
	}
	require.NoError(t, a.State.AddServiceWithChecks(srv1, nil, "", false))

	// Add another service
	srv2 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"kv"},
		Meta: map[string]string{
			"foo": "bar",
		},
		Port: 1234,
	}
	require.NoError(t, a.State.AddServiceWithChecks(srv2, nil, "", false))

	req, _ := http.NewRequest("GET", "/v1/agent/services?filter="+url.QueryEscape("foo in Meta"), nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	decoder := json.NewDecoder(resp.Body)
	var val map[string]*api.AgentService
	err := decoder.Decode(&val)
	require.NoError(t, err)
	require.Len(t, val, 2)

	req, _ = http.NewRequest("GET", "/v1/agent/services?filter="+url.QueryEscape("kv in Tags"), nil)
	resp = httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	decoder = json.NewDecoder(resp.Body)
	val = make(map[string]*api.AgentService)
	err = decoder.Decode(&val)
	require.NoError(t, err)
	require.Len(t, val, 1)
}

// This tests that the agent services endpoint (/v1/agent/services) returns
// Connect proxies.
func TestAgent_Services_ExternalConnectProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "db-proxy",
		Service: "db-proxy",
		Port:    5000,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "db",
			Upstreams:              structs.TestUpstreams(t, false),
		},
	}
	a.State.AddServiceWithChecks(srv1, nil, "", false)

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	decoder := json.NewDecoder(resp.Body)
	var val map[string]*api.AgentService
	err := decoder.Decode(&val)
	require.NoError(t, err)

	assert.Len(t, val, 1)
	actual := val["db-proxy"]
	assert.Equal(t, api.ServiceKindConnectProxy, actual.Kind)
	assert.Equal(t, srv1.Proxy.ToAPI(), actual.Proxy)
}

// Thie tests that a sidecar-registered service is returned as expected.
func TestAgent_Services_Sidecar(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "db-sidecar-proxy",
		Service: "db-sidecar-proxy",
		Port:    5000,
		// Set this internal state that we expect sidecar registrations to have.
		LocallyRegisteredAsSidecar: true,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "db",
			Upstreams:              structs.TestUpstreams(t, false),
			Mode:                   structs.ProxyModeTransparent,
			TransparentProxy: structs.TransparentProxyConfig{
				OutboundListenerPort: 10101,
			},
		},
	}
	a.State.AddServiceWithChecks(srv1, nil, "", false)

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	decoder := json.NewDecoder(resp.Body)
	var val map[string]*api.AgentService
	err := decoder.Decode(&val)
	require.NoError(t, err)

	assert.Len(t, val, 1)
	actual := val["db-sidecar-proxy"]
	require.NotNil(t, actual)
	assert.Equal(t, api.ServiceKindConnectProxy, actual.Kind)
	assert.Equal(t, srv1.Proxy.ToAPI(), actual.Proxy)

	// Sanity check that LocalRegisteredAsSidecar is not in the output (assuming
	// JSON encoding). Right now this is not the case because the services
	// endpoint happens to use the api struct which doesn't include that field,
	// but this test serves as a regression test incase we change the endpoint to
	// return the internal struct later and accidentally expose some "internal"
	// state.
	assert.NotContains(t, resp.Body.String(), "LocallyRegisteredAsSidecar")
	assert.NotContains(t, resp.Body.String(), "locally_registered_as_sidecar")
}

// This tests that a mesh gateway service is returned as expected.
func TestAgent_Services_MeshGateway(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		Kind:    structs.ServiceKindMeshGateway,
		ID:      "mg-dc1-01",
		Service: "mg-dc1",
		Port:    8443,
		Proxy: structs.ConnectProxyConfig{
			Config: map[string]interface{}{
				"foo": "bar",
			},
		},
	}
	a.State.AddServiceWithChecks(srv1, nil, "", false)

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	dec := json.NewDecoder(resp.Body)
	var val map[string]*api.AgentService
	err := dec.Decode(&val)
	require.NoError(t, err)

	require.Len(t, val, 1)
	actual := val["mg-dc1-01"]
	require.NotNil(t, actual)
	require.Equal(t, api.ServiceKindMeshGateway, actual.Kind)
	// Proxy.ToAPI() creates an empty Upstream list instead of keeping nil so do the same with actual.
	if actual.Proxy.Upstreams == nil {
		actual.Proxy.Upstreams = make([]api.Upstream, 0)
	}
	require.Equal(t, srv1.Proxy.ToAPI(), actual.Proxy)
}

// This tests that a terminating gateway service is returned as expected.
func TestAgent_Services_TerminatingGateway(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		Kind:    structs.ServiceKindTerminatingGateway,
		ID:      "tg-dc1-01",
		Service: "tg-dc1",
		Port:    8443,
		Proxy: structs.ConnectProxyConfig{
			Config: map[string]interface{}{
				"foo": "bar",
			},
		},
	}
	require.NoError(t, a.State.AddServiceWithChecks(srv1, nil, "", false))

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	dec := json.NewDecoder(resp.Body)
	var val map[string]*api.AgentService
	err := dec.Decode(&val)
	require.NoError(t, err)

	require.Len(t, val, 1)
	actual := val["tg-dc1-01"]
	require.NotNil(t, actual)
	require.Equal(t, api.ServiceKindTerminatingGateway, actual.Kind)
	// Proxy.ToAPI() creates an empty Upstream list instead of keeping nil so do the same with actual.
	if actual.Proxy.Upstreams == nil {
		actual.Proxy.Upstreams = make([]api.Upstream, 0)
	}
	require.Equal(t, srv1.Proxy.ToAPI(), actual.Proxy)
}

func TestAgent_Services_ACLFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	services := []*structs.NodeService{
		{
			ID:      "web",
			Service: "web",
			Port:    5000,
		},
		{
			ID:      "api",
			Service: "api",
			Port:    6000,
		},
	}
	for _, s := range services {
		a.State.AddServiceWithChecks(s, nil, "", false)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		dec := json.NewDecoder(resp.Body)
		var val map[string]*api.AgentService
		err := dec.Decode(&val)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}

		if len(val) != 0 {
			t.Fatalf("bad: %v", val)
		}
		require.Len(t, val, 0)
		require.Empty(t, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
	})

	t.Run("limited token", func(t *testing.T) {

		token := testCreateToken(t, a, `
			service "web" {
				policy = "read"
			}
		`)

		req := httptest.NewRequest("GET", "/v1/agent/services", nil)
		req.Header.Add("X-Consul-Token", token)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		dec := json.NewDecoder(resp.Body)
		var val map[string]*api.AgentService
		if err := dec.Decode(&val); err != nil {
			t.Fatalf("Err: %v", err)
		}
		require.Len(t, val, 1)
		require.NotEmpty(t, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		dec := json.NewDecoder(resp.Body)
		var val map[string]*api.AgentService
		err := dec.Decode(&val)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		require.Len(t, val, 2)
		require.Empty(t, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
	})
}

func TestAgent_Service(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, TestACLConfig()+`
	services {
		name = "web"
		port = 8181
		tagged_addresses {
			wan {
				address = "198.18.0.1"
				port = 1818
			}
		}
	}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	proxy := structs.TestConnectProxyConfig(t)
	proxy.DestinationServiceID = "web1"

	// Define a valid local sidecar proxy service
	sidecarProxy := &structs.ServiceDefinition{
		Kind: structs.ServiceKindConnectProxy,
		Name: "web-sidecar-proxy",
		Check: structs.CheckType{
			TCP:      "127.0.0.1:8000",
			Interval: 10 * time.Second,
		},
		Port:  8000,
		Proxy: &proxy,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	// Define an updated version. Be careful to copy it.
	updatedProxy := *sidecarProxy
	updatedProxy.Port = 9999

	// Mangle the proxy config/upstreams into the expected for with defaults and
	// API struct types.
	expectProxy := proxy
	expectProxy.Upstreams =
		structs.TestAddDefaultsToUpstreams(t, sidecarProxy.Proxy.Upstreams, *structs.DefaultEnterpriseMetaInDefaultPartition())

	expectedResponse := &api.AgentService{
		Kind:    api.ServiceKindConnectProxy,
		ID:      "web-sidecar-proxy",
		Service: "web-sidecar-proxy",
		Port:    8000,
		Proxy:   expectProxy.ToAPI(),
		Weights: api.AgentWeights{
			Passing: 1,
			Warning: 1,
		},
		Meta:       map[string]string{},
		Tags:       []string{},
		Datacenter: "dc1",
	}
	fillAgentServiceEnterpriseMeta(expectedResponse, structs.DefaultEnterpriseMetaInDefaultPartition())
	hash1, err := hashstructure.Hash(expectedResponse, nil)
	require.NoError(t, err, "failed to generate hash")
	expectedResponse.ContentHash = fmt.Sprintf("%x", hash1)

	// Copy and modify
	updatedResponse := *expectedResponse
	updatedResponse.Port = 9999
	updatedResponse.ContentHash = "" // clear field before hashing
	hash2, err := hashstructure.Hash(updatedResponse, nil)
	require.NoError(t, err, "failed to generate hash")
	updatedResponse.ContentHash = fmt.Sprintf("%x", hash2)

	// Simple response for non-proxy service registered in TestAgent config
	expectWebResponse := &api.AgentService{
		ID:      "web",
		Service: "web",
		Port:    8181,
		Weights: api.AgentWeights{
			Passing: 1,
			Warning: 1,
		},
		TaggedAddresses: map[string]api.ServiceAddress{
			"wan": {
				Address: "198.18.0.1",
				Port:    1818,
			},
		},
		Meta:       map[string]string{},
		Tags:       []string{},
		Datacenter: "dc1",
	}
	fillAgentServiceEnterpriseMeta(expectWebResponse, structs.DefaultEnterpriseMetaInDefaultPartition())
	hash3, err := hashstructure.Hash(expectWebResponse, nil)
	require.NoError(t, err, "failed to generate hash")
	expectWebResponse.ContentHash = fmt.Sprintf("%x", hash3)

	tests := []struct {
		name       string
		policies   string
		url        string
		updateFunc func()
		wantWait   time.Duration
		wantCode   int
		wantErr    string
		wantResp   *api.AgentService
	}{
		{
			name:     "simple fetch - proxy",
			url:      "/v1/agent/service/web-sidecar-proxy",
			wantCode: 200,
			wantResp: expectedResponse,
		},
		{
			name:     "simple fetch - non-proxy",
			url:      "/v1/agent/service/web",
			wantCode: 200,
			wantResp: expectWebResponse,
		},
		{
			name:     "blocking fetch timeout, no change",
			url:      "/v1/agent/service/web-sidecar-proxy?hash=" + expectedResponse.ContentHash + "&wait=100ms",
			wantWait: 100 * time.Millisecond,
			wantCode: 200,
			wantResp: expectedResponse,
		},
		{
			name:     "blocking fetch old hash should return immediately",
			url:      "/v1/agent/service/web-sidecar-proxy?hash=123456789abcd&wait=10m",
			wantCode: 200,
			wantResp: expectedResponse,
		},
		{
			name: "blocking fetch returns change",
			url:  "/v1/agent/service/web-sidecar-proxy?hash=" + expectedResponse.ContentHash,
			updateFunc: func() {
				time.Sleep(100 * time.Millisecond)
				// Re-register with new proxy config, make sure we copy the struct so we
				// don't alter it and affect later test cases.
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(updatedProxy))
				req.Header.Add("X-Consul-Token", "root")
				resp := httptest.NewRecorder()
				a.srv.h.ServeHTTP(resp, req)
				require.Equal(t, 200, resp.Code, "body: %s", resp.Body.String())
			},
			wantWait: 100 * time.Millisecond,
			wantCode: 200,
			wantResp: &updatedResponse,
		},
		{
			// This test exercises a case that caused a busy loop to eat CPU for the
			// entire duration of the blocking query. If a service gets re-registered
			// wth same proxy config then the old proxy config chan is closed causing
			// blocked watchset.Watch to return false indicating a change. But since
			// the hash is the same when the blocking fn is re-called we should just
			// keep blocking on the next iteration. The bug hit was that the WatchSet
			// ws was not being reset in the loop and so when you try to `Watch` it
			// the second time it just returns immediately making the blocking loop
			// into a busy-poll!
			//
			// This test though doesn't catch that because busy poll still has the
			// correct external behavior. I don't want to instrument the loop to
			// assert it's not executing too fast here as I can't think of a clean way
			// and the issue is fixed now so this test doesn't actually catch the
			// error, but does provide an easy way to verify the behavior by hand:
			//  1. Make this test fail e.g. change wantErr to true
			//  2. Add a log.Println or similar into the blocking loop/function
			//  3. See whether it's called just once or many times in a tight loop.
			name: "blocking fetch interrupted with no change (same hash)",
			url:  "/v1/agent/service/web-sidecar-proxy?wait=200ms&hash=" + expectedResponse.ContentHash,
			updateFunc: func() {
				time.Sleep(100 * time.Millisecond)
				// Re-register with _same_ proxy config
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(sidecarProxy))
				req.Header.Add("X-Consul-Token", "root")
				resp := httptest.NewRecorder()
				a.srv.h.ServeHTTP(resp, req)
				require.Equal(t, 200, resp.Code, "body: %s", resp.Body.String())
			},
			wantWait: 200 * time.Millisecond,
			wantCode: 200,
			wantResp: expectedResponse,
		},
		{
			// When we reload config, the agent pauses Anti-entropy, then clears all
			// services (which causes their watch chans to be closed) before loading
			// state from config/snapshot again). If we do that naively then we don't
			// just get a spurios wakeup on the watch if the service didn't change,
			// but we get it wakeup and then race with the reload and probably see no
			// services and return a 404 error which is gross. This test exercises
			// that - even though the registrations were from API not config, they are
			// persisted and cleared/reloaded from snapshot which has same effect.
			//
			// The fix for this test is to allow the same mechanism that pauses
			// Anti-entropy during reload to also pause the hash blocking loop so we
			// don't resume until the state is reloaded and we get a chance to see if
			// it actually changed or not.
			name: "blocking fetch interrupted by reload shouldn't 404 - no change",
			url:  "/v1/agent/service/web-sidecar-proxy?wait=200ms&hash=" + expectedResponse.ContentHash,
			updateFunc: func() {
				time.Sleep(100 * time.Millisecond)
				// Reload
				require.NoError(t, a.reloadConfigInternal(a.Config))
			},
			// Should eventually timeout since there is no actual change
			wantWait: 200 * time.Millisecond,
			wantCode: 200,
			wantResp: expectedResponse,
		},
		{
			// As above but test actually altering the service with the config reload.
			// This simulates the API registration being overridden by a different one
			// on disk during reload.
			name: "blocking fetch interrupted by reload shouldn't 404 - changes",
			url:  "/v1/agent/service/web-sidecar-proxy?wait=10m&hash=" + expectedResponse.ContentHash,
			updateFunc: func() {
				time.Sleep(100 * time.Millisecond)
				// Reload
				newConfig := *a.Config
				newConfig.Services = append(newConfig.Services, &updatedProxy)
				require.NoError(t, a.reloadConfigInternal(&newConfig))
			},
			wantWait: 100 * time.Millisecond,
			wantCode: 200,
			wantResp: &updatedResponse,
		},
		{
			name:    "err: non-existent proxy",
			url:     "/v1/agent/service/nope",
			wantErr: fmt.Sprintf("unknown service ID: %s", structs.NewServiceID("nope", nil)),
		},
		{
			name: "err: bad ACL for service",
			url:  "/v1/agent/service/web-sidecar-proxy",
			// Limited token doesn't grant read to the service
			policies: `
			key "" {
				policy = "read"
			}
			`,
			// Note that because we return ErrPermissionDenied and handle writing
			// status at a higher level helper this actually gets a 200 in this test
			// case so just assert that it was an error.
			wantErr: "Permission denied",
		},
		{
			name: "good ACL for service",
			url:  "/v1/agent/service/web-sidecar-proxy",
			// Limited token doesn't grant read to the service
			policies: `
			service "web-sidecar-proxy" {
				policy = "read"
			}
			`,
			wantCode: 200,
			wantResp: expectedResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Register the basic service to ensure it's in a known state to start.
			{
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(sidecarProxy))
				req.Header.Add("X-Consul-Token", "root")
				resp := httptest.NewRecorder()
				a.srv.h.ServeHTTP(resp, req)
				require.Equal(t, 200, resp.Code, "body: %s", resp.Body.String())
			}

			req, _ := http.NewRequest("GET", tt.url, nil)

			// Inject the root token for tests that don't care about ACL
			token := "root"
			if tt.policies != "" {
				// Create new token and use that.
				token = testCreateToken(t, a, tt.policies)
			}
			req.Header.Set("X-Consul-Token", token)
			resp := httptest.NewRecorder()
			if tt.updateFunc != nil {
				go tt.updateFunc()
			}
			start := time.Now()
			a.srv.h.ServeHTTP(resp, req)
			elapsed := time.Since(start)

			if tt.wantErr != "" {
				require.Contains(t, strings.ToLower(resp.Body.String()), strings.ToLower(tt.wantErr))
			}
			if tt.wantCode != 0 {
				require.Equal(t, tt.wantCode, resp.Code, "body: %s", resp.Body.String())
			}
			if tt.wantWait != 0 {
				assert.True(t, elapsed >= tt.wantWait, "should have waited at least %s, "+
					"took %s", tt.wantWait, elapsed)
			} else {
				assert.True(t, elapsed < 10*time.Millisecond, "should not have waited, "+
					"took %s", elapsed)
			}

			if tt.wantResp != nil {
				dec := json.NewDecoder(resp.Body)
				val := &api.AgentService{}
				err := dec.Decode(&val)
				require.NoError(t, err)

				assert.Equal(t, tt.wantResp, val)
				assert.Equal(t, tt.wantResp.ContentHash, resp.Header().Get("X-Consul-ContentHash"))
			}
		})
	}
}

func TestAgent_Checks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	chk1 := &structs.HealthCheck{
		Node:     a.Config.NodeName,
		CheckID:  "mysql",
		Name:     "mysql",
		Interval: "30s",
		Timeout:  "5s",
		Status:   api.HealthPassing,
	}
	a.State.AddCheck(chk1, "", false)

	req, _ := http.NewRequest("GET", "/v1/agent/checks", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	dec := json.NewDecoder(resp.Body)
	var val map[types.CheckID]*structs.HealthCheck
	err := dec.Decode(&val)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	if len(val) != 1 {
		t.Fatalf("bad checks: %v", val)
	}
	if val["mysql"].Status != api.HealthPassing {
		t.Fatalf("bad check: %v", val)
	}
	if val["mysql"].Node != chk1.Node {
		t.Fatalf("bad check: %v", val)
	}
	if val["mysql"].Interval != chk1.Interval {
		t.Fatalf("bad check: %v", val)
	}
	if val["mysql"].Timeout != chk1.Timeout {
		t.Fatalf("bad check: %v", val)
	}
}

func TestAgent_ChecksWithFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	chk1 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "mysql",
		Name:    "mysql",
		Status:  api.HealthPassing,
	}
	a.State.AddCheck(chk1, "", false)

	chk2 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "redis",
		Name:    "redis",
		Status:  api.HealthPassing,
	}
	a.State.AddCheck(chk2, "", false)

	req, _ := http.NewRequest("GET", "/v1/agent/checks?filter="+url.QueryEscape("Name == `redis`"), nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	dec := json.NewDecoder(resp.Body)
	var val map[types.CheckID]*structs.HealthCheck
	err := dec.Decode(&val)
	require.NoError(t, err)

	require.Len(t, val, 1)
	_, ok := val["redis"]
	require.True(t, ok)
}

func TestAgent_HealthServiceByID(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	service := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
	}

	serviceReq := AddServiceRequest{
		Service:  service,
		chkTypes: nil,
		persist:  false,
		token:    "",
		Source:   ConfigSourceLocal,
	}
	if err := a.AddService(serviceReq); err != nil {
		t.Fatalf("err: %v", err)
	}
	serviceReq.Service = &structs.NodeService{
		ID:      "mysql2",
		Service: "mysql2",
	}
	if err := a.AddService(serviceReq); err != nil {
		t.Fatalf("err: %v", err)
	}
	serviceReq.Service = &structs.NodeService{
		ID:      "mysql3",
		Service: "mysql3",
	}
	if err := a.AddService(serviceReq); err != nil {
		t.Fatalf("err: %v", err)
	}

	chk1 := &structs.HealthCheck{
		Node:      a.Config.NodeName,
		CheckID:   "mysql",
		Name:      "mysql",
		ServiceID: "mysql",
		Status:    api.HealthPassing,
	}
	err := a.State.AddCheck(chk1, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk2 := &structs.HealthCheck{
		Node:      a.Config.NodeName,
		CheckID:   "mysql",
		Name:      "mysql",
		ServiceID: "mysql",
		Status:    api.HealthPassing,
	}
	err = a.State.AddCheck(chk2, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk3 := &structs.HealthCheck{
		Node:      a.Config.NodeName,
		CheckID:   "mysql2",
		Name:      "mysql2",
		ServiceID: "mysql2",
		Status:    api.HealthPassing,
	}
	err = a.State.AddCheck(chk3, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk4 := &structs.HealthCheck{
		Node:      a.Config.NodeName,
		CheckID:   "mysql2",
		Name:      "mysql2",
		ServiceID: "mysql2",
		Status:    api.HealthWarning,
	}
	err = a.State.AddCheck(chk4, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk5 := &structs.HealthCheck{
		Node:      a.Config.NodeName,
		CheckID:   "mysql3",
		Name:      "mysql3",
		ServiceID: "mysql3",
		Status:    api.HealthMaint,
	}
	err = a.State.AddCheck(chk5, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk6 := &structs.HealthCheck{
		Node:      a.Config.NodeName,
		CheckID:   "mysql3",
		Name:      "mysql3",
		ServiceID: "mysql3",
		Status:    api.HealthCritical,
	}
	err = a.State.AddCheck(chk6, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	eval := func(t *testing.T, url string, expectedCode int, expected string) {
		t.Helper()
		t.Run("format=text", func(t *testing.T) {
			t.Helper()
			req, _ := http.NewRequest("GET", url+"?format=text", nil)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			body := resp.Body.String()
			if got, want := resp.Code, expectedCode; got != want {
				t.Fatalf("returned bad status: expected %d, but had: %d", expectedCode, resp.Code)
			}
			if got, want := body, expected; got != want {
				t.Fatalf("got body %q want %q", got, want)
			}
		})
		t.Run("format=json", func(t *testing.T) {
			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if got, want := resp.Code, expectedCode; got != want {
				t.Fatalf("returned bad status: expected %d, but had: %d", expectedCode, resp.Code)
			}
			dec := json.NewDecoder(resp.Body)
			data := &api.AgentServiceChecksInfo{}
			if err := dec.Decode(data); err != nil {
				t.Fatalf("Cannot convert result from JSON: %v", err)
			}
			if resp.Code != http.StatusNotFound {
				if data != nil && data.AggregatedStatus != expected {
					t.Fatalf("got body %v want %v", data, expected)
				}
			}
		})
	}

	t.Run("passing checks", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/id/mysql", http.StatusOK, "passing")
	})
	t.Run("warning checks", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/id/mysql2", http.StatusTooManyRequests, "warning")
	})
	t.Run("critical checks", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/id/mysql3", http.StatusServiceUnavailable, "critical")
	})
	t.Run("unknown serviceid", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/id/mysql1", http.StatusNotFound, fmt.Sprintf("ServiceId %s not found", structs.ServiceIDString("mysql1", nil)))
	})

	nodeCheck := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "diskCheck",
		Name:    "diskCheck",
		Status:  api.HealthCritical,
	}
	err = a.State.AddCheck(nodeCheck, "", false)

	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	t.Run("critical check on node", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/id/mysql", http.StatusServiceUnavailable, "critical")
	})

	err = a.State.RemoveCheck(nodeCheck.CompoundCheckID())
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	nodeCheck = &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "_node_maintenance",
		Name:    "_node_maintenance",
		Status:  api.HealthMaint,
	}
	err = a.State.AddCheck(nodeCheck, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	t.Run("maintenance check on node", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/id/mysql", http.StatusServiceUnavailable, "maintenance")
	})
}

func TestAgent_HealthServiceByName(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	service := &structs.NodeService{
		ID:      "mysql1",
		Service: "mysql-pool-r",
	}
	serviceReq := AddServiceRequest{
		Service:  service,
		chkTypes: nil,
		persist:  false,
		token:    "",
		Source:   ConfigSourceLocal,
	}
	if err := a.AddService(serviceReq); err != nil {
		t.Fatalf("err: %v", err)
	}
	serviceReq.Service = &structs.NodeService{
		ID:      "mysql2",
		Service: "mysql-pool-r",
	}
	if err := a.AddService(serviceReq); err != nil {
		t.Fatalf("err: %v", err)
	}
	serviceReq.Service = &structs.NodeService{
		ID:      "mysql3",
		Service: "mysql-pool-rw",
	}
	if err := a.AddService(serviceReq); err != nil {
		t.Fatalf("err: %v", err)
	}
	serviceReq.Service = &structs.NodeService{
		ID:      "mysql4",
		Service: "mysql-pool-rw",
	}
	if err := a.AddService(serviceReq); err != nil {
		t.Fatalf("err: %v", err)
	}
	serviceReq.Service = &structs.NodeService{
		ID:      "httpd1",
		Service: "httpd",
	}
	if err := a.AddService(serviceReq); err != nil {
		t.Fatalf("err: %v", err)
	}
	serviceReq.Service = &structs.NodeService{
		ID:      "httpd2",
		Service: "httpd",
	}
	if err := a.AddService(serviceReq); err != nil {
		t.Fatalf("err: %v", err)
	}

	chk1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "mysql1",
		Name:        "mysql1",
		ServiceID:   "mysql1",
		ServiceName: "mysql-pool-r",
		Status:      api.HealthPassing,
	}
	err := a.State.AddCheck(chk1, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk2 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "mysql1",
		Name:        "mysql1",
		ServiceID:   "mysql1",
		ServiceName: "mysql-pool-r",
		Status:      api.HealthWarning,
	}
	err = a.State.AddCheck(chk2, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk3 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "mysql2",
		Name:        "mysql2",
		ServiceID:   "mysql2",
		ServiceName: "mysql-pool-r",
		Status:      api.HealthPassing,
	}
	err = a.State.AddCheck(chk3, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk4 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "mysql2",
		Name:        "mysql2",
		ServiceID:   "mysql2",
		ServiceName: "mysql-pool-r",
		Status:      api.HealthCritical,
	}
	err = a.State.AddCheck(chk4, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk5 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "mysql3",
		Name:        "mysql3",
		ServiceID:   "mysql3",
		ServiceName: "mysql-pool-rw",
		Status:      api.HealthWarning,
	}
	err = a.State.AddCheck(chk5, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk6 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "mysql4",
		Name:        "mysql4",
		ServiceID:   "mysql4",
		ServiceName: "mysql-pool-rw",
		Status:      api.HealthPassing,
	}
	err = a.State.AddCheck(chk6, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk7 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "httpd1",
		Name:        "httpd1",
		ServiceID:   "httpd1",
		ServiceName: "httpd",
		Status:      api.HealthPassing,
	}
	err = a.State.AddCheck(chk7, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	chk8 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "httpd2",
		Name:        "httpd2",
		ServiceID:   "httpd2",
		ServiceName: "httpd",
		Status:      api.HealthPassing,
	}
	err = a.State.AddCheck(chk8, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	eval := func(t *testing.T, url string, expectedCode int, expected string) {
		t.Helper()
		t.Run("format=text", func(t *testing.T) {
			t.Helper()
			req, _ := http.NewRequest("GET", url+"?format=text", nil)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if got, want := resp.Code, expectedCode; got != want {
				t.Fatalf("returned bad status: %d. Body: %q", resp.Code, resp.Body.String())
			}
			if got, want := resp.Body.String(), expected; got != want {
				t.Fatalf("got body %q want %q", got, want)
			}
		})
		t.Run("format=json", func(t *testing.T) {
			t.Helper()
			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			dec := json.NewDecoder(resp.Body)
			data := make([]*api.AgentServiceChecksInfo, 0)
			if err := dec.Decode(&data); err != nil {
				t.Fatalf("Cannot convert result from JSON: %v", err)
			}
			if got, want := resp.Code, expectedCode; got != want {
				t.Fatalf("returned bad code: %d. Body: %#v", resp.Code, data)
			}
			if resp.Code != http.StatusNotFound {
				matched := false
				for _, d := range data {
					if d.AggregatedStatus == expected {
						matched = true
						break
					}
				}

				if !matched {
					t.Fatalf("got wrong status, wanted %#v", expected)
				}
			}
		})
	}

	t.Run("passing checks", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/name/httpd", http.StatusOK, "passing")
	})
	t.Run("warning checks", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/name/mysql-pool-rw", http.StatusTooManyRequests, "warning")
	})
	t.Run("critical checks", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/name/mysql-pool-r", http.StatusServiceUnavailable, "critical")
	})
	t.Run("unknown serviceName", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/name/test", http.StatusNotFound, "ServiceName test Not Found")
	})
	nodeCheck := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "diskCheck",
		Name:    "diskCheck",
		Status:  api.HealthCritical,
	}
	err = a.State.AddCheck(nodeCheck, "", false)

	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	t.Run("critical check on node", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/name/mysql-pool-r", http.StatusServiceUnavailable, "critical")
	})

	err = a.State.RemoveCheck(nodeCheck.CompoundCheckID())
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	nodeCheck = &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "_node_maintenance",
		Name:    "_node_maintenance",
		Status:  api.HealthMaint,
	}
	err = a.State.AddCheck(nodeCheck, "", false)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	t.Run("maintenance check on node", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/name/mysql-pool-r", http.StatusServiceUnavailable, "maintenance")
	})
}

func TestAgent_HealthServicesACLEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfigWithParams(nil))
	defer a.Shutdown()

	service := &structs.NodeService{
		ID:      "mysql1",
		Service: "mysql",
	}
	serviceReq := AddServiceRequest{
		Service:  service,
		chkTypes: nil,
		persist:  false,
		token:    "",
		Source:   ConfigSourceLocal,
	}
	require.NoError(t, a.AddService(serviceReq))

	serviceReq.Service = &structs.NodeService{
		ID:      "foo1",
		Service: "foo",
	}
	require.NoError(t, a.AddService(serviceReq))

	// no token
	t.Run("no-token-health-by-id", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/agent/health/service/id/mysql1", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("no-token-health-by-name", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/agent/health/service/name/mysql", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root-token-health-by-id", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/agent/health/service/id/foo1", nil)
		require.NoError(t, err)
		req.Header.Add("X-Consul-Token", TestDefaultInitialManagementToken)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("root-token-health-by-name", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/agent/health/service/name/foo", nil)
		require.NoError(t, err)
		req.Header.Add("X-Consul-Token", TestDefaultInitialManagementToken)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_Checks_ACLFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	checks := structs.HealthChecks{
		{
			Node:        a.Config.NodeName,
			CheckID:     "web",
			ServiceName: "web",
			Status:      api.HealthPassing,
		},
		{
			Node:        a.Config.NodeName,
			CheckID:     "api",
			ServiceName: "api",
			Status:      api.HealthPassing,
		},
	}
	for _, c := range checks {
		a.State.AddCheck(c, "", false)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/checks", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		dec := json.NewDecoder(resp.Body)
		val := make(map[types.CheckID]*structs.HealthCheck)
		if err := dec.Decode(&val); err != nil {
			t.Fatalf("Err: %v", err)
		}

		require.Len(t, val, 0)
		require.Empty(t, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
	})

	t.Run("limited token", func(t *testing.T) {

		token := testCreateToken(t, a, fmt.Sprintf(`
			service "web" {
				policy = "read"
			}
			node "%s" {
				policy = "read"
			}
		`, a.Config.NodeName))

		req := httptest.NewRequest("GET", "/v1/agent/checks", nil)
		req.Header.Add("X-Consul-Token", token)
		resp := httptest.NewRecorder()

		a.srv.h.ServeHTTP(resp, req)
		dec := json.NewDecoder(resp.Body)
		var val map[types.CheckID]*structs.HealthCheck
		if err := dec.Decode(&val); err != nil {
			t.Fatalf("Err: %v", err)
		}
		require.Len(t, val, 1)
		require.NotEmpty(t, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/checks", nil)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		dec := json.NewDecoder(resp.Body)
		val := make(map[types.CheckID]*structs.HealthCheck)
		if err := dec.Decode(&val); err != nil {
			t.Fatalf("Err: %v", err)
		}
		require.Len(t, val, 2)
		require.Empty(t, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
	})
}

func TestAgent_Self(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	cases := map[string]struct {
		hcl       string
		expectXDS bool
		grpcTLS   bool
	}{
		"no grpc": {
			hcl: `
			node_meta {
				somekey = "somevalue"
			}
			ports = {
				grpc = -1
				grpc_tls = -1
			}`,
			expectXDS: false,
			grpcTLS:   false,
		},
		"plaintext grpc": {
			hcl: `
			node_meta {
				somekey = "somevalue"
			}
			ports = {
				grpc_tls = -1
			}`,
			expectXDS: true,
			grpcTLS:   false,
		},
		"tls grpc": {
			hcl: `
				node_meta {
					somekey = "somevalue"
				}`,
			expectXDS: true,
			grpcTLS:   true,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			a := StartTestAgent(t, TestAgent{
				HCL:        tc.hcl,
				UseGRPCTLS: tc.grpcTLS,
			})
			defer a.Shutdown()

			testrpc.WaitForTestAgent(t, a.RPC, "dc1")
			req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)

			dec := json.NewDecoder(resp.Body)
			val := &Self{}
			require.NoError(t, dec.Decode(val))

			require.Equal(t, a.Config.SerfPortLAN, int(val.Member.Port))
			require.Equal(t, a.Config.SerfPortLAN, int(val.DebugConfig["SerfPortLAN"].(float64)))

			cs, err := a.GetLANCoordinate()
			require.NoError(t, err)
			require.Equal(t, cs[a.config.SegmentName], val.Coord)

			delete(val.Meta, structs.MetaSegmentKey) // Added later, not in config.
			require.Equal(t, a.config.NodeMeta, val.Meta)

			if tc.expectXDS {
				require.NotNil(t, val.XDS, "xds component missing when gRPC is enabled")
				require.Equal(t,
					map[string][]string{"envoy": xdscommon.EnvoyVersions},
					val.XDS.SupportedProxies,
				)
				require.Equal(t, a.Config.GRPCTLSPort, val.XDS.Ports.TLS)
				require.Equal(t, a.Config.GRPCPort, val.XDS.Ports.Plaintext)
				if tc.grpcTLS {
					require.Equal(t, a.Config.GRPCTLSPort, val.XDS.Port)
				} else {
					require.Equal(t, a.Config.GRPCPort, val.XDS.Port)
				}

			} else {
				require.Nil(t, val.XDS, "xds component should be missing when gRPC is disabled")
			}
		})
	}
}

func TestAgent_Self_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("agent recovery token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
		req.Header.Add("X-Consul-Token", "towel")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := createACLTokenWithAgentReadPolicy(t, a.srv)
		req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
		req.Header.Add("X-Consul-Token", ro)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_Metrics_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/metrics", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("agent recovery token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/metrics", nil)
		req.Header.Add("X-Consul-Token", "towel")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := createACLTokenWithAgentReadPolicy(t, a.srv)
		req, _ := http.NewRequest("GET", "/v1/agent/metrics", nil)
		req.Header.Add("X-Consul-Token", ro)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestHTTPHandlers_AgentMetricsStream_ACLDeny(t *testing.T) {
	bd := BaseDeps{}
	bd.Tokens = new(tokenStore.Store)
	sink := metrics.NewInmemSink(30*time.Millisecond, time.Second)
	bd.MetricsConfig = &lib.MetricsConfig{
		Handler: sink,
	}
	d := fakeResolveTokenDelegate{authorizer: acl.DenyAll()}
	agent := &Agent{
		baseDeps: bd,
		delegate: d,
		tokens:   bd.Tokens,
		config:   &config.RuntimeConfig{NodeName: "the-node"},
		logger:   hclog.NewInterceptLogger(nil),
	}
	h := HTTPHandlers{agent: agent, denylist: NewDenylist(nil)}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/v1/agent/metrics/stream", nil)
	require.NoError(t, err)
	handle := h.handler()
	handle.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Contains(t, resp.Body.String(), "Permission denied")
}

func TestHTTPHandlers_AgentMetricsStream(t *testing.T) {
	bd := BaseDeps{}
	bd.Tokens = new(tokenStore.Store)
	sink := metrics.NewInmemSink(20*time.Millisecond, time.Second)
	bd.MetricsConfig = &lib.MetricsConfig{
		Handler: sink,
	}
	d := fakeResolveTokenDelegate{authorizer: acl.ManageAll()}
	agent := &Agent{
		baseDeps: bd,
		delegate: d,
		tokens:   bd.Tokens,
		config:   &config.RuntimeConfig{NodeName: "the-node"},
		logger:   hclog.NewInterceptLogger(nil),
	}
	h := HTTPHandlers{agent: agent, denylist: NewDenylist(nil)}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	// produce some metrics
	go func() {
		for ctx.Err() == nil {
			sink.SetGauge([]string{"the-key"}, 12)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	resp := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/v1/agent/metrics/stream", nil)
	require.NoError(t, err)
	handle := h.handler()
	handle.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	decoder := json.NewDecoder(resp.Body)
	var summary metrics.MetricsSummary
	err = decoder.Decode(&summary)
	require.NoError(t, err)

	expected := []metrics.GaugeValue{
		{Name: "the-key", Value: 12, DisplayLabels: map[string]string{}},
	}
	require.Equal(t, expected, summary.Gauges)

	// There should be at least two intervals worth of metrics
	err = decoder.Decode(&summary)
	require.NoError(t, err)
	require.Equal(t, expected, summary.Gauges)
}

type fakeResolveTokenDelegate struct {
	delegate
	authorizer acl.Authorizer
}

func (f fakeResolveTokenDelegate) ResolveTokenAndDefaultMeta(_ string, _ *acl.EnterpriseMeta, _ *acl.AuthorizerContext) (resolver.Result, error) {
	return resolver.Result{Authorizer: f.authorizer}, nil
}

func TestAgent_Reload(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dc1 := "dc1"
	a := NewTestAgent(t, `
		services = [
			{
				name = "redis"
			}
		]
		watches = [
			{
				datacenter = "`+dc1+`"
				type = "key"
				key = "test"
				handler = "true"
			}
		]
    limits = {
      rpc_rate=1
      rpc_max_burst=100
    }
	`)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, dc1)
	if a.State.Service(structs.NewServiceID("redis", nil)) == nil {
		t.Fatal("missing redis service")
	}

	cfg2 := TestConfig(testutil.Logger(t), config.FileSource{
		Name:   "reload",
		Format: "hcl",
		Data: `
			data_dir = "` + a.Config.DataDir + `"
			node_id = "` + string(a.Config.NodeID) + `"
			node_name = "` + a.Config.NodeName + `"

			services = [
				{
					name = "redis-reloaded"
				}
			]
      limits = {
        rpc_rate=2
        rpc_max_burst=200
      }
		`,
	})

	shim := &delegateConfigReloadShim{delegate: a.delegate}
	// NOTE: this may require refactoring to remove a potential test race
	a.delegate = shim
	if err := a.reloadConfigInternal(cfg2); err != nil {
		t.Fatalf("got error %v want nil", err)
	}
	if a.State.Service(structs.NewServiceID("redis-reloaded", nil)) == nil {
		t.Fatal("missing redis-reloaded service")
	}

	require.Equal(t, rate.Limit(2), shim.newCfg.RPCRateLimit)
	require.Equal(t, 200, shim.newCfg.RPCMaxBurst)

	for _, wp := range a.watchPlans {
		if !wp.IsStopped() {
			t.Fatalf("Reloading configs should stop watch plans of the previous configuration")
		}
	}
}

type delegateConfigReloadShim struct {
	delegate
	newCfg consul.ReloadableConfig
}

func (s *delegateConfigReloadShim) ReloadConfig(cfg consul.ReloadableConfig) error {
	s.newCfg = cfg
	return s.delegate.ReloadConfig(cfg)
}

// TestAgent_ReloadDoesNotTriggerWatch Ensure watches not triggered after reload
// see https://github.com/hashicorp/consul/issues/7446
func TestAgent_ReloadDoesNotTriggerWatch(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dc1 := "dc1"
	tmpFileRaw, err := os.CreateTemp("", "rexec")
	require.NoError(t, err)
	tmpFile := tmpFileRaw.Name()
	defer os.Remove(tmpFile)
	handlerShell := fmt.Sprintf("(cat ; echo CONSUL_INDEX $CONSUL_INDEX) | tee '%s.atomic' ; mv '%s.atomic' '%s'", tmpFile, tmpFile, tmpFile)

	a := NewTestAgent(t, `
		services = [
			{
				name = "redis"
				checks = [
					{
						id =  "red-is-dead"
						ttl = "30s"
						notes = "initial check"
					}
				]
			}
		]
		watches = [
			{
				datacenter = "`+dc1+`"
				type = "service"
				service = "redis"
				args = ["bash", "-c", "`+handlerShell+`"]
			}
		]
	`)
	checkID := structs.NewCheckID("red-is-dead", nil)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, dc1)
	require.NoError(t, a.updateTTLCheck(checkID, api.HealthPassing, "testing-agent-reload-001"))

	checkStr := func(r *retry.R, evaluator func(string) error) {
		t.Helper()
		contentsStr := ""
		// Wait for watch to be populated
		for i := 1; i < 7; i++ {
			contents, err := os.ReadFile(tmpFile)
			if err != nil {
				r.Fatalf("should be able to read file, but had: %#v", err)
			}
			contentsStr = string(contents)
			if contentsStr != "" {
				break
			}
			time.Sleep(time.Duration(i) * time.Second)
			testutil.Logger(t).Info("Watch not yet populated, retrying")
		}
		if err := evaluator(contentsStr); err != nil {
			r.Errorf("ERROR: Test failing: %s", err)
		}
	}
	ensureNothingCritical := func(r *retry.R, mustContain string) {
		t.Helper()
		eval := func(contentsStr string) error {
			if strings.Contains(contentsStr, "critical") {
				return fmt.Errorf("MUST NOT contain critical:= %s", contentsStr)
			}
			if !strings.Contains(contentsStr, mustContain) {
				return fmt.Errorf("MUST contain '%s' := %s", mustContain, contentsStr)
			}
			return nil
		}
		checkStr(r, eval)
	}

	retriesWithDelay := func() *retry.Counter {
		return &retry.Counter{Count: 10, Wait: 1 * time.Second}
	}

	retry.RunWith(retriesWithDelay(), t, func(r *retry.R) {
		testutil.Logger(t).Info("Consul is now ready")
		// it should contain the output
		checkStr(r, func(contentStr string) error {
			if contentStr == "[]" {
				return fmt.Errorf("Consul is still starting up")
			}
			return nil
		})
	})

	retry.RunWith(retriesWithDelay(), t, func(r *retry.R) {
		ensureNothingCritical(r, "testing-agent-reload-001")
	})

	// Let's take almost the same config
	cfg2 := TestConfig(testutil.Logger(t), config.FileSource{
		Name:   "reload",
		Format: "hcl",
		Data: `
			data_dir = "` + a.Config.DataDir + `"
			node_id = "` + string(a.Config.NodeID) + `"
			node_name = "` + a.Config.NodeName + `"

			services = [
				{
					name = "redis"
					checks = [
						{
							id  = "red-is-dead"
							ttl = "30s"
							notes = "initial check"
						}
					]
				}
			]
			watches = [
				{
					datacenter = "` + dc1 + `"
					type = "service"
					service = "redis"
					args = ["bash", "-c", "` + handlerShell + `"]
				}
			]
		`,
	})

	justOnce := func() *retry.Counter {
		return &retry.Counter{Count: 1, Wait: 25 * time.Millisecond}
	}

	retry.RunWith(justOnce(), t, func(r *retry.R) {
		// We check that reload does not go to critical
		ensureNothingCritical(r, "red-is-dead")

		if err := a.reloadConfigInternal(cfg2); err != nil {
			r.Fatalf("got error %v want nil", err)
		}

		// We check that reload does not go to critical
		ensureNothingCritical(r, "red-is-dead")
		ensureNothingCritical(r, "testing-agent-reload-001")

		require.NoError(r, a.updateTTLCheck(checkID, api.HealthPassing, "testing-agent-reload-002"))

		ensureNothingCritical(r, "red-is-dead")
	})
}

func TestAgent_Reload_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/reload", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := createACLTokenWithAgentReadPolicy(t, a.srv)
		req, _ := http.NewRequest("PUT", "/v1/agent/reload", nil)
		req.Header.Add("X-Consul-Token", ro)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	// This proves we call the ACL function, and we've got the other reload
	// test to prove we do the reload, which should be sufficient.
	// The reload logic is a little complex to set up so isn't worth
	// repeating again here.
}

func TestAgent_Members(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/members", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	dec := json.NewDecoder(resp.Body)
	val := make([]serf.Member, 0)
	if err := dec.Decode(&val); err != nil {
		t.Fatalf("Err: %v", err)
	}

	if len(val) == 0 {
		t.Fatalf("bad members: %v", val)
	}

	if int(val[0].Port) != a.Config.SerfPortLAN {
		t.Fatalf("not lan: %v", val)
	}
}

func TestAgent_Members_WAN(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/members?wan=true", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	dec := json.NewDecoder(resp.Body)
	val := make([]serf.Member, 0)
	if err := dec.Decode(&val); err != nil {
		t.Fatalf("Err: %v", err)
	}

	if len(val) == 0 {
		t.Fatalf("bad members: %v", val)
	}

	if int(val[0].Port) != a.Config.SerfPortWAN {
		t.Fatalf("not wan: %v", val)
	}
}

func TestAgent_Members_ACLFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// Start 2 agents and join them together.
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	b := NewTestAgent(t, TestACLConfig())
	defer b.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	testrpc.WaitForLeader(t, b.RPC, "dc1")

	joinPath := fmt.Sprintf("/v1/agent/join/127.0.0.1:%d", b.Config.SerfPortLAN)
	req := httptest.NewRequest("PUT", joinPath, nil)
	req.Header.Add("X-Consul-Token", "root")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/members", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		dec := json.NewDecoder(resp.Body)
		val := make([]serf.Member, 0)
		if err := dec.Decode(&val); err != nil {
			t.Fatalf("Err: %v", err)
		}
		require.Len(t, val, 0)
		require.Empty(t, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
	})

	t.Run("limited token", func(t *testing.T) {

		token := testCreateToken(t, a, fmt.Sprintf(`
			node "%s" {
				policy = "read"
			}
		`, b.Config.NodeName))

		req := httptest.NewRequest("GET", "/v1/agent/members", nil)
		req.Header.Add("X-Consul-Token", token)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		dec := json.NewDecoder(resp.Body)
		val := make([]serf.Member, 0)
		if err := dec.Decode(&val); err != nil {
			t.Fatalf("Err: %v", err)
		}
		require.Len(t, val, 1)
		require.NotEmpty(t, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/members", nil)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		dec := json.NewDecoder(resp.Body)
		val := make([]serf.Member, 0)
		if err := dec.Decode(&val); err != nil {
			t.Fatalf("Err: %v", err)
		}
		require.Len(t, val, 2)
		require.Empty(t, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
	})
}

func TestAgent_Join(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, "")
	defer a1.Shutdown()
	a2 := NewTestAgent(t, "")
	defer a2.Shutdown()
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s", addr), nil)
	resp := httptest.NewRecorder()
	a1.srv.h.ServeHTTP(resp, req)

	if len(a1.LANMembersInAgentPartition()) != 2 {
		t.Fatalf("should have 2 members")
	}

	retry.Run(t, func(r *retry.R) {
		if got, want := len(a2.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d LAN members want %d", got, want)
		}
	})
}

func TestAgent_Join_WAN(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, "")
	defer a1.Shutdown()
	a2 := NewTestAgent(t, "")
	defer a2.Shutdown()
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortWAN)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s?wan=true", addr), nil)
	resp := httptest.NewRecorder()
	a1.srv.h.ServeHTTP(resp, req)

	if len(a1.WANMembers()) != 2 {
		t.Fatalf("should have 2 members")
	}

	retry.Run(t, func(r *retry.R) {
		if got, want := len(a2.WANMembers()), 2; got != want {
			r.Fatalf("got %d WAN members want %d", got, want)
		}
	})
}

func TestAgent_Join_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, TestACLConfig())
	defer a1.Shutdown()
	a2 := NewTestAgent(t, "")
	defer a2.Shutdown()
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s", addr), nil)
		resp := httptest.NewRecorder()
		a1.srv.h.ServeHTTP(resp, req)

		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("agent recovery token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s", addr), nil)
		req.Header.Add("X-Consul-Token", "towel")
		resp := httptest.NewRecorder()
		a1.srv.h.ServeHTTP(resp, req)

		require.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := createACLTokenWithAgentReadPolicy(t, a1.srv)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s", addr), nil)
		req.Header.Add("X-Consul-Token", ro)
		resp := httptest.NewRecorder()
		a1.srv.h.ServeHTTP(resp, req)

		require.Equal(t, http.StatusForbidden, resp.Code)
	})
}

type mockNotifier struct{ s string }

func (n *mockNotifier) Notify(state string) error {
	n.s = state
	return nil
}

func TestAgent_JoinLANNotify(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, "")
	defer a1.Shutdown()
	testrpc.WaitForLeader(t, a1.RPC, "dc1")

	a2 := NewTestAgent(t, `
		server = false
		bootstrap = false
	`)
	defer a2.Shutdown()

	notif := &mockNotifier{}
	a1.joinLANNotifier = notif

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	_, err := a1.JoinLAN([]string{addr}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if got, want := notif.s, "READY=1"; got != want {
		t.Fatalf("got joinLAN notification %q want %q", got, want)
	}
}

func TestAgent_Leave(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, "")
	defer a1.Shutdown()
	testrpc.WaitForLeader(t, a1.RPC, "dc1")

	a2 := NewTestAgent(t, `
 		server = false
 		bootstrap = false
 	`)
	defer a2.Shutdown()

	// Join first
	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	_, err := a1.JoinLAN([]string{addr}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Graceful leave now
	req, _ := http.NewRequest("PUT", "/v1/agent/leave", nil)
	resp := httptest.NewRecorder()
	a2.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembersInAgentPartition()
		if got, want := m[1].Status, serf.StatusLeft; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})
}

func TestAgent_Leave_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/leave", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := createACLTokenWithAgentReadPolicy(t, a.srv)
		req, _ := http.NewRequest("PUT", "/v1/agent/leave", nil)
		req.Header.Add("X-Consul-Token", ro)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	// this sub-test will change the state so that there is no leader.
	// it must therefore be the last one in this list.
	t.Run("agent recovery token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/leave", nil)
		req.Header.Add("X-Consul-Token", "towel")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_ForceLeave(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, "")
	defer a1.Shutdown()
	a2 := NewTestAgent(t, "")
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	// Join first
	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	_, err := a1.JoinLAN([]string{addr}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// this test probably needs work
	a2.Shutdown()
	// Wait for agent being marked as failed, so we wait for full shutdown of Agent
	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembersInAgentPartition()
		if got, want := m[1].Status, serf.StatusFailed; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})

	// Force leave now
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/force-leave/%s", a2.Config.NodeName), nil)
	resp := httptest.NewRecorder()
	a1.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembersInAgentPartition()
		if got, want := m[1].Status, serf.StatusLeft; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})
}

func TestOpenMetricsMimeTypeHeaders(t *testing.T) {
	t.Parallel()
	assert.False(t, acceptsOpenMetricsMimeType(""))
	assert.False(t, acceptsOpenMetricsMimeType(";;;"))
	assert.False(t, acceptsOpenMetricsMimeType(",,,"))
	assert.False(t, acceptsOpenMetricsMimeType("text/plain"))
	assert.True(t, acceptsOpenMetricsMimeType("text/plain;version=0.4.0,"))
	assert.True(t, acceptsOpenMetricsMimeType("text/plain;version=0.4.0;q=1,*/*;q=0.1"))
	assert.True(t, acceptsOpenMetricsMimeType("text/plain   ;   version=0.4.0"))
	assert.True(t, acceptsOpenMetricsMimeType("*/*, application/openmetrics-text ;"))
	assert.True(t, acceptsOpenMetricsMimeType("*/*, application/openmetrics-text ;q=1"))
	assert.True(t, acceptsOpenMetricsMimeType("application/openmetrics-text, text/plain;version=0.4.0"))
	assert.True(t, acceptsOpenMetricsMimeType("application/openmetrics-text; version=0.0.1,text/plain;version=0.0.4;q=0.5,*/*;q=0.1"))
}

func TestAgent_ForceLeave_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	uri := fmt.Sprintf("/v1/agent/force-leave/%s", a.Config.NodeName)

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", uri, nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("agent recovery token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", uri, nil)
		req.Header.Add("X-Consul-Token", "towel")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := createACLTokenWithAgentReadPolicy(t, a.srv)
		req, _ := http.NewRequest("PUT", uri, nil)
		req.Header.Add("X-Consul-Token", ro)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("operator write token", func(t *testing.T) {
		// Create an ACL with operator read permissions.
		rules := `
                    operator = "write"
                `
		opToken := testCreateToken(t, a, rules)

		req, _ := http.NewRequest("PUT", uri, nil)
		req.Header.Add("X-Consul-Token", opToken)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_ForceLeavePrune(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := StartTestAgent(t, TestAgent{Name: "Agent1"})
	defer a1.Shutdown()
	a2 := StartTestAgent(t, TestAgent{Name: "Agent2"})
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	// Join first
	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	_, err := a1.JoinLAN([]string{addr}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// this test probably needs work
	a2.Shutdown()
	// Wait for agent being marked as failed, so we wait for full shutdown of Agent
	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembersInAgentPartition()
		for _, member := range m {
			if member.Name == a2.Config.NodeName {
				if member.Status != serf.StatusFailed {
					r.Fatalf("got status %q want %q", member.Status, serf.StatusFailed)
				}
			}
		}
	})

	// Force leave now
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/force-leave/%s?prune=true", a2.Config.NodeName), nil)
	resp := httptest.NewRecorder()
	a1.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	retry.Run(t, func(r *retry.R) {
		m := len(a1.LANMembersInAgentPartition())
		if m != 1 {
			r.Fatalf("want one member, got %v", m)
		}
	})
}

func TestAgent_ForceLeavePrune_WAN(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a1 := StartTestAgent(t, TestAgent{Name: "dc1", HCL: `
		datacenter = "dc1"
		primary_datacenter = "dc1"
		gossip_wan {
			probe_interval = "50ms"
			suspicion_mult = 2
		}
	`})
	defer a1.Shutdown()

	a2 := StartTestAgent(t, TestAgent{Name: "dc2", HCL: `
		datacenter = "dc2"
		primary_datacenter = "dc1"
	`})
	defer a2.Shutdown()

	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc2")

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	_, err := a2.JoinWAN([]string{addr})
	require.NoError(t, err)

	testrpc.WaitForLeader(t, a1.RPC, "dc2")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	retry.Run(t, func(r *retry.R) {
		require.Len(r, a1.WANMembers(), 2)
		require.Len(r, a2.WANMembers(), 2)
	})

	wanNodeName_a2 := a2.Config.NodeName + ".dc2"

	// Shutdown and wait for agent being marked as failed, so we wait for full
	// shutdown of Agent.
	require.NoError(t, a2.Shutdown())
	retry.Run(t, func(r *retry.R) {
		m := a1.WANMembers()
		for _, member := range m {
			if member.Name == wanNodeName_a2 {
				if member.Status != serf.StatusFailed {
					r.Fatalf("got status %q want %q", member.Status, serf.StatusFailed)
				}
			}
		}
	})

	// Force leave now
	req, err := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/force-leave/%s?prune=1&wan=1", wanNodeName_a2), nil)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	a1.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	retry.Run(t, func(r *retry.R) {
		require.Len(r, a1.WANMembers(), 1)
	})
}

func TestAgent_RegisterCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.CheckDefinition{
		Name: "test",
		TTL:  15 * time.Second,
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(args))
	req.Header.Add("X-Consul-Token", "abc123")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// Ensure we have a check mapping
	checkID := structs.NewCheckID("test", nil)
	if existing := a.State.Check(checkID); existing == nil {
		t.Fatalf("missing test check")
	}

	if _, ok := a.checkTTLs[checkID]; !ok {
		t.Fatalf("missing test check ttl")
	}

	// Ensure the token was configured
	if token := a.State.CheckToken(checkID); token == "" {
		t.Fatalf("missing token")
	}

	// By default, checks start in critical state.
	state := a.State.Check(checkID)
	if state.Status != api.HealthCritical {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_RegisterCheck_UDP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.CheckDefinition{
		UDP:      "1.1.1.1",
		Name:     "test",
		Interval: 10 * time.Second,
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(args))
	req.Header.Add("X-Consul-Token", "abc123")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// Ensure we have a check mapping
	checkID := structs.NewCheckID("test", nil)
	if existing := a.State.Check(checkID); existing == nil {
		t.Fatalf("missing test check")
	}

	if _, ok := a.checkUDPs[checkID]; !ok {
		t.Fatalf("missing test check udp")
	}

	// Ensure the token was configured
	if token := a.State.CheckToken(checkID); token == "" {
		t.Fatalf("missing token")
	}

	// By default, checks start in critical state.
	state := a.State.Check(checkID)
	if state.Status != api.HealthCritical {
		t.Fatalf("bad: %v", state)
	}
}

// This verifies all the forms of the new args-style check that we need to
// support as a result of https://github.com/hashicorp/consul/issues/3587.
func TestAgent_RegisterCheck_Scripts(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		enable_script_checks = true
`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	tests := []struct {
		name  string
		check map[string]interface{}
	}{
		{
			"== Consul 1.0.0",
			map[string]interface{}{
				"Name":       "test",
				"Interval":   "2s",
				"ScriptArgs": []string{"true"},
			},
		},
		{
			"> Consul 1.0.0 (fixup)",
			map[string]interface{}{
				"Name":        "test",
				"Interval":    "2s",
				"script_args": []string{"true"},
			},
		},
		{
			"> Consul 1.0.0",
			map[string]interface{}{
				"Name":     "test",
				"Interval": "2s",
				"Args":     []string{"true"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name+" as node check", func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(tt.check))
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("bad: %d", resp.Code)
			}
		})

		t.Run(tt.name+" as top-level service check", func(t *testing.T) {
			args := map[string]interface{}{
				"Name":  "a",
				"Port":  1234,
				"Check": tt.check,
			}

			req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("bad: %d", resp.Code)
			}
		})

		t.Run(tt.name+" as slice-based service check", func(t *testing.T) {
			args := map[string]interface{}{
				"Name":   "a",
				"Port":   1234,
				"Checks": []map[string]interface{}{tt.check},
			}

			req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("bad: %d", resp.Code)
			}
		})
	}
}

func TestAgent_RegisterCheckScriptsExecDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.CheckDefinition{
		Name:       "test",
		ScriptArgs: []string{"true"},
		Interval:   time.Second,
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(args))
	req.Header.Add("X-Consul-Token", "abc123")
	res := httptest.NewRecorder()
	a.srv.h.ServeHTTP(res, req)
	if http.StatusInternalServerError != res.Code {
		t.Fatalf("expected 500 code error but got %v", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Scripts are disabled on this agent") {
		t.Fatalf("expected script disabled error, got: %s", res.Body.String())
	}
	checkID := structs.NewCheckID("test", nil)
	require.Nil(t, a.State.Check(checkID), "check registered with exec disabled")
}

func TestAgent_RegisterCheckScriptsExecRemoteDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		enable_local_script_checks = true
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.CheckDefinition{
		Name:       "test",
		ScriptArgs: []string{"true"},
		Interval:   time.Second,
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(args))
	req.Header.Add("X-Consul-Token", "abc123")
	res := httptest.NewRecorder()
	a.srv.h.ServeHTTP(res, req)
	if http.StatusInternalServerError != res.Code {
		t.Fatalf("expected 500 code error but got %v", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Scripts are disabled on this agent") {
		t.Fatalf("expected script disabled error, got: %s", res.Body.String())
	}
	checkID := structs.NewCheckID("test", nil)
	require.Nil(t, a.State.Check(checkID), "check registered with exec disabled")
}

func TestAgent_RegisterCheck_Passing(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.CheckDefinition{
		Name:   "test",
		TTL:    15 * time.Second,
		Status: api.HealthPassing,
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if http.StatusOK != resp.Code {
		t.Fatalf("expcted 200 but got %v", resp.Code)
	}

	// Ensure we have a check mapping
	checkID := structs.NewCheckID("test", nil)
	if existing := a.State.Check(checkID); existing == nil {
		t.Fatalf("missing test check")
	}

	if _, ok := a.checkTTLs[checkID]; !ok {
		t.Fatalf("missing test check ttl")
	}

	state := a.State.Check(checkID)
	if state.Status != api.HealthPassing {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_RegisterCheck_BadStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.CheckDefinition{
		Name:   "test",
		TTL:    15 * time.Second,
		Status: "fluffy",
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("accepted bad status")
	}
}

func TestAgent_RegisterCheck_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfigNew())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	nodeCheck := &structs.CheckDefinition{
		Name: "test",
		TTL:  15 * time.Second,
	}

	svc := &structs.ServiceDefinition{
		ID:   "foo:1234",
		Name: "foo",
		Port: 1234,
	}

	svcCheck := &structs.CheckDefinition{
		Name:      "test2",
		ServiceID: "foo:1234",
		TTL:       15 * time.Second,
	}

	// ensure the service is ready for registering a check for it.
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(svc))
	req.Header.Add("X-Consul-Token", "root")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// create a policy that has write on service foo
	policyReq := &structs.ACLPolicy{
		Name:  "write-foo",
		Rules: `service "foo" { policy = "write"}`,
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/policy", jsonReader(policyReq))
	req.Header.Add("X-Consul-Token", "root")
	resp = httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// create a policy that has write on the node name of the agent
	policyReq = &structs.ACLPolicy{
		Name:  "write-node",
		Rules: fmt.Sprintf(`node "%s" { policy = "write" }`, a.config.NodeName),
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/policy", jsonReader(policyReq))
	req.Header.Add("X-Consul-Token", "root")
	resp = httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// create a token using the write-foo policy
	tokenReq := &structs.ACLToken{
		Description: "write-foo",
		Policies: []structs.ACLTokenPolicyLink{
			{
				Name: "write-foo",
			},
		},
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/token", jsonReader(tokenReq))
	req.Header.Add("X-Consul-Token", "root")
	resp = httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	dec := json.NewDecoder(resp.Body)
	svcToken := &structs.ACLToken{}
	if err := dec.Decode(svcToken); err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NotNil(t, svcToken)

	// create a token using the write-node policy
	tokenReq = &structs.ACLToken{
		Description: "write-node",
		Policies: []structs.ACLTokenPolicyLink{
			{
				Name: "write-node",
			},
		},
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/token", jsonReader(tokenReq))
	req.Header.Add("X-Consul-Token", "root")
	resp = httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	dec = json.NewDecoder(resp.Body)
	nodeToken := &structs.ACLToken{}
	if err := dec.Decode(nodeToken); err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NotNil(t, nodeToken)

	t.Run("no token - node check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(nodeCheck))
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			require.Equal(r, http.StatusForbidden, resp.Code)
		})
	})

	t.Run("svc token - node check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(nodeCheck))
			req.Header.Add("X-Consul-Token", svcToken.SecretID)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			require.Equal(r, http.StatusForbidden, resp.Code)
		})
	})

	t.Run("node token - node check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(nodeCheck))
			req.Header.Add("X-Consul-Token", nodeToken.SecretID)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			require.Equal(r, http.StatusOK, resp.Code)
		})
	})

	t.Run("no token - svc check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(svcCheck))
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			require.Equal(r, http.StatusForbidden, resp.Code)
		})
	})

	t.Run("node token - svc check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(svcCheck))
			req.Header.Add("X-Consul-Token", nodeToken.SecretID)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			require.Equal(r, http.StatusForbidden, resp.Code)
		})
	})

	t.Run("svc token - svc check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(svcCheck))
			req.Header.Add("X-Consul-Token", svcToken.SecretID)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			require.Equal(r, http.StatusOK, resp.Code)
		})
	})
}

func TestAgent_DeregisterCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	if err := a.AddCheck(chk, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("remove registered check", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("remove non-existent check", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusNotFound, resp.Code)
	})

	// Ensure we have a check mapping
	requireCheckMissing(t, a, "test")
}

func TestAgent_DeregisterCheckACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WithToken("root"))

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	if err := a.AddCheck(chk, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/test", nil)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("non-existent check without token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/_nope_", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("non-existent check with token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/_nope_", nil)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusNotFound, resp.Code)
	})
}

func TestAgent_PassCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/check/pass/test", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	if http.StatusOK != resp.Code {
		t.Fatalf("expected 200 by got %v", resp.Code)
	}

	// Ensure we have a check mapping
	state := a.State.Check(structs.NewCheckID("test", nil))
	if state.Status != api.HealthPassing {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_PassCheck_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/pass/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/pass/test", nil)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_WarnCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/check/warn/test", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	if http.StatusOK != resp.Code {
		t.Fatalf("expected 200 by got %v", resp.Code)
	}

	// Ensure we have a check mapping
	state := a.State.Check(structs.NewCheckID("test", nil))
	if state.Status != api.HealthWarning {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_WarnCheck_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/warn/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/warn/test", nil)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_FailCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/check/fail/test", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	if http.StatusOK != resp.Code {
		t.Fatalf("expected 200 by got %v", resp.Code)
	}

	// Ensure we have a check mapping
	state := a.State.Check(structs.NewCheckID("test", nil))
	if state.Status != api.HealthCritical {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_FailCheck_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/fail/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/fail/test", nil)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_UpdateCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	maxChecksSize := 256
	a := NewTestAgent(t, fmt.Sprintf("check_output_max_size=%d", maxChecksSize))
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	cases := []checkUpdate{
		{api.HealthPassing, "hello-passing"},
		{api.HealthCritical, "hello-critical"},
		{api.HealthWarning, "hello-warning"},
	}

	for _, c := range cases {
		t.Run(c.Status, func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(c))
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			state := a.State.Check(structs.NewCheckID("test", nil))
			if state.Status != c.Status || state.Output != c.Output {
				t.Fatalf("bad: %v", state)
			}
		})
	}

	t.Run("log output limit", func(t *testing.T) {
		args := checkUpdate{
			Status: api.HealthPassing,
			Output: strings.Repeat("-= bad -=", 5*maxChecksSize),
		}
		req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.Code)
		}

		// Since we append some notes about truncating, we just do a
		// rough check that the output buffer was cut down so this test
		// isn't super brittle.
		state := a.State.Check(structs.NewCheckID("test", nil))
		if state.Status != api.HealthPassing || len(state.Output) > 2*maxChecksSize {
			t.Fatalf("bad: %v, (len:=%d)", state, len(state.Output))
		}
	})

	t.Run("bogus status", func(t *testing.T) {
		args := checkUpdate{Status: "itscomplicated"}
		req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
	})
}

func TestAgent_UpdateCheck_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		args := checkUpdate{api.HealthPassing, "hello-passing"}
		req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root token", func(t *testing.T) {
		args := checkUpdate{api.HealthPassing, "hello-passing"}
		req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(args))
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_RegisterService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.ServiceDefinition{
		Name: "test",
		Meta: map[string]string{"hello": "world"},
		Tags: []string{"primary"},
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Checks: []*structs.CheckType{
			{
				TTL: 20 * time.Second,
			},
			{
				TTL: 30 * time.Second,
			},
			{
				UDP:      "1.1.1.1",
				Interval: 5 * time.Second,
			},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	req.Header.Add("X-Consul-Token", "abc123")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if http.StatusOK != resp.Code {
		t.Fatalf("expected 200 but got %v", resp.Code)
	}

	// Ensure the service
	sid := structs.NewServiceID("test", nil)
	svc := a.State.Service(sid)
	if svc == nil {
		t.Fatalf("missing test service")
	}
	if val := svc.Meta["hello"]; val != "world" {
		t.Fatalf("Missing meta: %v", svc.Meta)
	}
	if val := svc.Weights.Passing; val != 100 {
		t.Fatalf("Expected 100 for Weights.Passing, got: %v", val)
	}
	if val := svc.Weights.Warning; val != 3 {
		t.Fatalf("Expected 3 for Weights.Warning, got: %v", val)
	}

	// Ensure we have a check mapping
	checks := a.State.Checks(structs.WildcardEnterpriseMetaInDefaultPartition())
	if len(checks) != 4 {
		t.Fatalf("bad: %v", checks)
	}
	for _, c := range checks {
		if c.Type != "ttl" && c.Type != "udp" {
			t.Fatalf("expected ttl or udp check type, got %s", c.Type)
		}
	}

	if len(a.checkTTLs) != 3 {
		t.Fatalf("missing test check ttls: %v", a.checkTTLs)
	}

	// Ensure the token was configured
	if token := a.State.ServiceToken(sid); token == "" {
		t.Fatalf("missing token")
	}
}

func TestAgent_RegisterService_ReRegister(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ReRegister(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ReRegister(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_ReRegister(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.ServiceDefinition{
		Name: "test",
		Meta: map[string]string{"hello": "world"},
		Tags: []string{"primary"},
		Port: 8000,
		Checks: []*structs.CheckType{
			{
				CheckID: types.CheckID("check_1"),
				TTL:     20 * time.Second,
			},
			{
				CheckID: types.CheckID("check_2"),
				TTL:     30 * time.Second,
			},
			{
				CheckID:  types.CheckID("check_3"),
				UDP:      "1.1.1.1",
				Interval: 5 * time.Second,
			},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	args = &structs.ServiceDefinition{
		Name: "test",
		Meta: map[string]string{"hello": "world"},
		Tags: []string{"primary"},
		Port: 8000,
		Checks: []*structs.CheckType{
			{
				CheckID: types.CheckID("check_1"),
				TTL:     20 * time.Second,
			},
			{
				CheckID: types.CheckID("check_3"),
				TTL:     30 * time.Second,
			},
			{
				CheckID:  types.CheckID("check_3"),
				UDP:      "1.1.1.1",
				Interval: 5 * time.Second,
			},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ = http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	resp = httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	checks := a.State.Checks(structs.DefaultEnterpriseMetaInDefaultPartition())
	require.Equal(t, 3, len(checks))

	checkIDs := []string{}
	for id := range checks {
		checkIDs = append(checkIDs, string(id.ID))
	}
	require.ElementsMatch(t, []string{"check_1", "check_2", "check_3"}, checkIDs)
}

func TestAgent_RegisterService_ReRegister_ReplaceExistingChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ReRegister_ReplaceExistingChecks(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ReRegister_ReplaceExistingChecks(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_ReRegister_ReplaceExistingChecks(t *testing.T, extraHCL string) {
	t.Helper()
	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.ServiceDefinition{
		Name: "test",
		Meta: map[string]string{"hello": "world"},
		Tags: []string{"primary"},
		Port: 8000,
		Checks: []*structs.CheckType{
			{
				// explicitly not setting the check id to let it be auto-generated
				// we want to ensure that we are testing out the cases with autogenerated names/ids
				TTL: 20 * time.Second,
			},
			{
				CheckID: types.CheckID("check_2"),
				TTL:     30 * time.Second,
			},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?replace-existing-checks", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	args = &structs.ServiceDefinition{
		Name: "test",
		Meta: map[string]string{"hello": "world"},
		Tags: []string{"primary"},
		Port: 8000,
		Checks: []*structs.CheckType{
			{
				TTL: 20 * time.Second,
			},
			{
				CheckID: types.CheckID("check_3"),
				TTL:     30 * time.Second,
			},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ = http.NewRequest("PUT", "/v1/agent/service/register?replace-existing-checks", jsonReader(args))
	resp = httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	checks := a.State.Checks(structs.DefaultEnterpriseMetaInDefaultPartition())
	require.Len(t, checks, 2)

	checkIDs := []string{}
	for id := range checks {
		checkIDs = append(checkIDs, string(id.ID))
	}
	require.ElementsMatch(t, []string{"service:test:1", "check_3"}, checkIDs)
}

func TestAgent_RegisterService_TranslateKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_TranslateKeys(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_TranslateKeys(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_TranslateKeys(t *testing.T, extraHCL string) {
	t.Helper()

	tests := []struct {
		ip                    string
		expectedTCPCheckStart string
	}{
		{"127.0.0.1", "127.0.0.1:"}, // private network address
		{"::1", "[::1]:"},           // shared address space
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			a := NewTestAgent(t, `
	connect {}
`+extraHCL)
			defer a.Shutdown()
			testrpc.WaitForTestAgent(t, a.RPC, "dc1")

			json := `
	{
		"name":"test",
		"port":8000,
		"enable_tag_override": true,
		"tagged_addresses": {
			"lan": {
				"address": "1.2.3.4",
				"port": 5353
			},
			"wan": {
				"address": "2.3.4.5",
				"port": 53
			}
		},
		"meta": {
			"some": "meta",
			"enable_tag_override": "meta is 'opaque' so should not get translated"
		},
		"kind": "connect-proxy",` +
				// Note the uppercase P is important here - it ensures translation works
				// correctly in case-insensitive way. Without it this test can pass even
				// when translation is broken for other valid inputs.
				`"Proxy": {
			"destination_service_name": "web",
			"destination_service_id": "web",
			"local_service_port": 1234,
			"local_service_address": "` + tt.ip + `",
			"config": {
				"destination_type": "proxy.config is 'opaque' so should not get translated"
			},
			"upstreams": [
				{
					"destination_type": "service",
					"destination_namespace": "default",
					"destination_partition": "default",
					"destination_name": "db",
		      "local_bind_address": "` + tt.ip + `",
		      "local_bind_port": 1234,
					"config": {
						"destination_type": "proxy.upstreams.config is 'opaque' so should not get translated"
					}
				}
			]
		},
		"connect": {
			"sidecar_service": {
				"name":"test-proxy",
				"port":8001,
				"enable_tag_override": true,
				"meta": {
					"some": "meta",
					"enable_tag_override": "sidecar_service.meta is 'opaque' so should not get translated"
				},
				"kind": "connect-proxy",
				"proxy": {
					"destination_service_name": "test",
					"destination_service_id": "test",
					"local_service_port": 4321,
					"local_service_address": "` + tt.ip + `",
					"upstreams": [
						{
							"destination_type": "service",
							"destination_namespace": "default",
							"destination_partition": "default",
							"destination_name": "db",
							"local_bind_address": "` + tt.ip + `",
							"local_bind_port": 1234,
							"config": {
								"destination_type": "sidecar_service.proxy.upstreams.config is 'opaque' so should not get translated"
							}
						}
					]
				}
			}
		},
		"weights":{
			"passing": 16
		}
	}`
			req, _ := http.NewRequest("PUT", "/v1/agent/service/register", strings.NewReader(json))

			rr := httptest.NewRecorder()
			a.srv.h.ServeHTTP(rr, req)
			require.Equal(t, 200, rr.Code, "body: %s", rr.Body)

			svc := &structs.NodeService{
				ID:      "test",
				Service: "test",
				TaggedAddresses: map[string]structs.ServiceAddress{
					"lan": {
						Address: "1.2.3.4",
						Port:    5353,
					},
					"wan": {
						Address: "2.3.4.5",
						Port:    53,
					},
				},
				Meta: map[string]string{
					"some":                "meta",
					"enable_tag_override": "meta is 'opaque' so should not get translated",
				},
				Port:              8000,
				EnableTagOverride: true,
				Weights:           &structs.Weights{Passing: 16, Warning: 0},
				Kind:              structs.ServiceKindConnectProxy,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web",
					LocalServiceAddress:    tt.ip,
					LocalServicePort:       1234,
					Config: map[string]interface{}{
						"destination_type": "proxy.config is 'opaque' so should not get translated",
					},
					Upstreams: structs.Upstreams{
						{
							DestinationType:      structs.UpstreamDestTypeService,
							DestinationName:      "db",
							DestinationNamespace: "default",
							DestinationPartition: "default",
							LocalBindAddress:     tt.ip,
							LocalBindPort:        1234,
							Config: map[string]interface{}{
								"destination_type": "proxy.upstreams.config is 'opaque' so should not get translated",
							},
						},
					},
				},
				Connect: structs.ServiceConnect{
					// The sidecar service is nilled since it is only config sugar and
					// shouldn't be represented in state. We assert that the translations
					// there worked by inspecting the registered sidecar below.
					SidecarService: nil,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}

			got := a.State.Service(structs.NewServiceID("test", nil))
			require.Equal(t, svc, got)

			sidecarSvc := &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "test-sidecar-proxy",
				Service: "test-proxy",
				Meta: map[string]string{
					"some":                "meta",
					"enable_tag_override": "sidecar_service.meta is 'opaque' so should not get translated",
				},
				TaggedAddresses:            map[string]structs.ServiceAddress{},
				Port:                       8001,
				EnableTagOverride:          true,
				Weights:                    &structs.Weights{Passing: 1, Warning: 1},
				LocallyRegisteredAsSidecar: true,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "test",
					DestinationServiceID:   "test",
					LocalServiceAddress:    tt.ip,
					LocalServicePort:       4321,
					Upstreams: structs.Upstreams{
						{
							DestinationType:      structs.UpstreamDestTypeService,
							DestinationName:      "db",
							DestinationNamespace: "default",
							DestinationPartition: "default",
							LocalBindAddress:     tt.ip,
							LocalBindPort:        1234,
							Config: map[string]interface{}{
								"destination_type": "sidecar_service.proxy.upstreams.config is 'opaque' so should not get translated",
							},
						},
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			gotSidecar := a.State.Service(structs.NewServiceID("test-sidecar-proxy", nil))
			hasNoCorrectTCPCheck := true
			for _, v := range a.checkTCPs {
				if strings.HasPrefix(v.TCP, tt.expectedTCPCheckStart) {
					hasNoCorrectTCPCheck = false
					break
				}
				fmt.Println("TCP Check:= ", v)
			}
			if hasNoCorrectTCPCheck {
				t.Fatalf("Did not find the expected TCP Healthcheck '%s' in %#v ", tt.expectedTCPCheckStart, a.checkTCPs)
			}
			require.Equal(t, sidecarSvc, gotSidecar)
		})
	}
}

func TestAgent_RegisterService_TranslateKeys_UDP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_TranslateKeys(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_TranslateKeys(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_TranslateKeys_UDP(t *testing.T, extraHCL string) {
	t.Helper()

	tests := []struct {
		ip                    string
		expectedUDPCheckStart string
	}{
		{"127.0.0.1", "127.0.0.1:"}, // private network address
		{"::1", "[::1]:"},           // shared address space
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			a := NewTestAgent(t, `
	connect {}
`+extraHCL)
			defer a.Shutdown()
			testrpc.WaitForTestAgent(t, a.RPC, "dc1")

			json := `
	{
		"name":"test",
		"port":8000,
		"enable_tag_override": true,
		"tagged_addresses": {
			"lan": {
				"address": "1.2.3.4",
				"port": 5353
			},
			"wan": {
				"address": "2.3.4.5",
				"port": 53
			}
		},
		"meta": {
			"some": "meta",
			"enable_tag_override": "meta is 'opaque' so should not get translated"
		},
		"kind": "connect-proxy",` +
				// Note the uppercase P is important here - it ensures translation works
				// correctly in case-insensitive way. Without it this test can pass even
				// when translation is broken for other valid inputs.
				`"Proxy": {
			"destination_service_name": "web",
			"destination_service_id": "web",
			"local_service_port": 1234,
			"local_service_address": "` + tt.ip + `",
			"config": {
				"destination_type": "proxy.config is 'opaque' so should not get translated"
			},
			"upstreams": [
				{
					"destination_type": "service",
					"destination_namespace": "default",
					"destination_partition": "default",
					"destination_name": "db",
		      "local_bind_address": "` + tt.ip + `",
		      "local_bind_port": 1234,
					"config": {
						"destination_type": "proxy.upstreams.config is 'opaque' so should not get translated"
					}
				}
			]
		},
		"connect": {
			"sidecar_service": {
				"name":"test-proxy",
				"port":8001,
				"enable_tag_override": true,
				"meta": {
					"some": "meta",
					"enable_tag_override": "sidecar_service.meta is 'opaque' so should not get translated"
				},
				"kind": "connect-proxy",
				"proxy": {
					"destination_service_name": "test",
					"destination_service_id": "test",
					"local_service_port": 4321,
					"local_service_address": "` + tt.ip + `",
					"upstreams": [
						{
							"destination_type": "service",
							"destination_namespace": "default",
							"destination_partition": "default",
							"destination_name": "db",
							"local_bind_address": "` + tt.ip + `",
							"local_bind_port": 1234,
							"config": {
								"destination_type": "sidecar_service.proxy.upstreams.config is 'opaque' so should not get translated"
							}
						}
					]
				}
			}
		},
		"weights":{
			"passing": 16
		}
	}`
			req, _ := http.NewRequest("PUT", "/v1/agent/service/register", strings.NewReader(json))

			rr := httptest.NewRecorder()
			a.srv.h.ServeHTTP(rr, req)
			require.Equal(t, 200, rr.Code, "body: %s", rr.Body)

			svc := &structs.NodeService{
				ID:      "test",
				Service: "test",
				TaggedAddresses: map[string]structs.ServiceAddress{
					"lan": {
						Address: "1.2.3.4",
						Port:    5353,
					},
					"wan": {
						Address: "2.3.4.5",
						Port:    53,
					},
				},
				Meta: map[string]string{
					"some":                "meta",
					"enable_tag_override": "meta is 'opaque' so should not get translated",
				},
				Port:              8000,
				EnableTagOverride: true,
				Weights:           &structs.Weights{Passing: 16, Warning: 0},
				Kind:              structs.ServiceKindConnectProxy,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web",
					LocalServiceAddress:    tt.ip,
					LocalServicePort:       1234,
					Config: map[string]interface{}{
						"destination_type": "proxy.config is 'opaque' so should not get translated",
					},
					Upstreams: structs.Upstreams{
						{
							DestinationType:      structs.UpstreamDestTypeService,
							DestinationName:      "db",
							DestinationNamespace: "default",
							DestinationPartition: "default",
							LocalBindAddress:     tt.ip,
							LocalBindPort:        1234,
							Config: map[string]interface{}{
								"destination_type": "proxy.upstreams.config is 'opaque' so should not get translated",
							},
						},
					},
				},
				Connect: structs.ServiceConnect{
					// The sidecar service is nilled since it is only config sugar and
					// shouldn't be represented in state. We assert that the translations
					// there worked by inspecting the registered sidecar below.
					SidecarService: nil,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}

			got := a.State.Service(structs.NewServiceID("test", nil))
			require.Equal(t, svc, got)

			sidecarSvc := &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "test-sidecar-proxy",
				Service: "test-proxy",
				Meta: map[string]string{
					"some":                "meta",
					"enable_tag_override": "sidecar_service.meta is 'opaque' so should not get translated",
				},
				TaggedAddresses:            map[string]structs.ServiceAddress{},
				Port:                       8001,
				EnableTagOverride:          true,
				Weights:                    &structs.Weights{Passing: 1, Warning: 1},
				LocallyRegisteredAsSidecar: true,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "test",
					DestinationServiceID:   "test",
					LocalServiceAddress:    tt.ip,
					LocalServicePort:       4321,
					Upstreams: structs.Upstreams{
						{
							DestinationType:      structs.UpstreamDestTypeService,
							DestinationName:      "db",
							DestinationNamespace: "default",
							DestinationPartition: "default",
							LocalBindAddress:     tt.ip,
							LocalBindPort:        1234,
							Config: map[string]interface{}{
								"destination_type": "sidecar_service.proxy.upstreams.config is 'opaque' so should not get translated",
							},
						},
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			gotSidecar := a.State.Service(structs.NewServiceID("test-sidecar-proxy", nil))
			hasNoCorrectUDPCheck := true
			for _, v := range a.checkUDPs {
				if strings.HasPrefix(v.UDP, tt.expectedUDPCheckStart) {
					hasNoCorrectUDPCheck = false
					break
				}
				fmt.Println("UDP Check:= ", v)
			}
			if hasNoCorrectUDPCheck {
				t.Fatalf("Did not find the expected UDP Healtcheck '%s' in %#v ", tt.expectedUDPCheckStart, a.checkUDPs)
			}
			require.Equal(t, sidecarSvc, gotSidecar)
		})
	}
}

func TestAgent_RegisterService_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ACLDeny(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ACLDeny(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_ACLDeny(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, TestACLConfig()+" "+extraHCL)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.ServiceDefinition{
		Name: "test",
		Tags: []string{"primary"},
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Checks: []*structs.CheckType{
			{
				TTL: 20 * time.Second,
			},
			{
				TTL: 30 * time.Second,
			},
		},
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_RegisterService_InvalidAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_InvalidAddress(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_InvalidAddress(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_InvalidAddress(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	for _, addr := range []string{"0.0.0.0", "::", "[::]"} {
		t.Run("addr "+addr, func(t *testing.T) {
			args := &structs.ServiceDefinition{
				Name:    "test",
				Address: addr,
				Port:    8000,
			}
			req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			req.Header.Add("X-Consul-Token", "abc123")
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if got, want := resp.Code, 400; got != want {
				t.Fatalf("got code %d want %d", got, want)
			}
		})
	}
}

// This tests local agent service registration of a unmanaged connect proxy.
// This verifies that it is put in the local state store properly for syncing
// later.
func TestAgent_RegisterService_UnmanagedConnectProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_UnmanagedConnectProxy(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_UnmanagedConnectProxy(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_UnmanagedConnectProxy(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register a proxy. Note that the destination doesn't exist here on this
	// agent or in the catalog at all. This is intended and part of the design.
	args := &api.AgentServiceRegistration{
		Kind: api.ServiceKindConnectProxy,
		Name: "connect-proxy",
		Port: 8000,
		Proxy: &api.AgentServiceConnectProxyConfig{
			DestinationServiceName: "web",
			Upstreams: []api.Upstream{
				{
					// No type to force default
					DestinationName: "db",
					LocalBindPort:   1234,
				},
				{
					DestinationType: "prepared_query",
					DestinationName: "geo-cache",
					LocalBindPort:   1235,
				},
			},
			Mode: api.ProxyModeTransparent,
			TransparentProxy: &api.TransparentProxyConfig{
				OutboundListenerPort: 808,
			},
		},
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	req.Header.Add("X-Consul-Token", "abc123")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// Ensure the service
	sid := structs.NewServiceID("connect-proxy", nil)
	svc := a.State.Service(sid)
	require.NotNil(t, svc, "has service")
	require.Equal(t, structs.ServiceKindConnectProxy, svc.Kind)

	// Registration sets default types and namespaces
	for i := range args.Proxy.Upstreams {
		if args.Proxy.Upstreams[i].DestinationType == "" {
			args.Proxy.Upstreams[i].DestinationType = api.UpstreamDestTypeService
		}
		if args.Proxy.Upstreams[i].DestinationNamespace == "" {
			args.Proxy.Upstreams[i].DestinationNamespace =
				structs.DefaultEnterpriseMetaInDefaultPartition().NamespaceOrEmpty()
		}
		if args.Proxy.Upstreams[i].DestinationPartition == "" {
			args.Proxy.Upstreams[i].DestinationPartition =
				structs.DefaultEnterpriseMetaInDefaultPartition().PartitionOrEmpty()
		}
	}

	require.Equal(t, args.Proxy, svc.Proxy.ToAPI())

	// Ensure the token was configured
	require.Equal(t, "abc123", a.State.ServiceToken(structs.NewServiceID("connect-proxy", nil)))
}

func testDefaultSidecar(svc string, port int, fns ...func(*structs.NodeService)) *structs.NodeService {
	ns := &structs.NodeService{
		ID:              svc + "-sidecar-proxy",
		Kind:            structs.ServiceKindConnectProxy,
		Service:         svc + "-sidecar-proxy",
		Port:            2222,
		TaggedAddresses: map[string]structs.ServiceAddress{},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		// Note that LocallyRegisteredAsSidecar should be true on the internal
		// NodeService, but that we never want to see it in the HTTP response as
		// it's internal only state. This is being compared directly to local state
		// so should be present here.
		LocallyRegisteredAsSidecar: true,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: svc,
			DestinationServiceID:   svc,
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       port,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	for _, fn := range fns {
		fn(ns)
	}
	return ns
}

// testCreateToken creates a Policy for the provided rules and a Token linked to that Policy.
func testCreateToken(t *testing.T, a *TestAgent, rules string) string {
	policyName, err := uuid.GenerateUUID() // we just need a unique name for the test and UUIDs are definitely unique
	require.NoError(t, err)

	policyID := testCreatePolicy(t, a, policyName, rules)

	args := map[string]interface{}{
		"Description": "User Token",
		"Policies": []map[string]interface{}{
			{
				"ID": policyID,
			},
		},
		"Local": false,
	}
	req, _ := http.NewRequest("PUT", "/v1/acl/token", jsonReader(args))
	req.Header.Add("X-Consul-Token", "root")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	dec := json.NewDecoder(resp.Body)
	aclResp := &structs.ACLToken{}
	if err := dec.Decode(aclResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	return aclResp.SecretID
}

func testCreatePolicy(t *testing.T, a *TestAgent, name, rules string) string {
	args := map[string]interface{}{
		"Name":  name,
		"Rules": rules,
	}
	req, _ := http.NewRequest("PUT", "/v1/acl/policy", jsonReader(args))
	req.Header.Add("X-Consul-Token", "root")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	dec := json.NewDecoder(resp.Body)
	aclResp := &structs.ACLPolicy{}
	if err := dec.Decode(aclResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	return aclResp.ID
}

// This tests local agent service registration with a sidecar service. Note we
// only test simple defaults for the sidecar here since the actual logic for
// handling sidecar defaults and port assignment is tested thoroughly in
// TestAgent_sidecarServiceFromNodeService. Note it also tests Deregister
// explicitly too since setup is identical.
func TestAgent_RegisterServiceDeregisterService_Sidecar(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterServiceDeregisterService_Sidecar(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterServiceDeregisterService_Sidecar(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterServiceDeregisterService_Sidecar(t *testing.T, extraHCL string) {
	t.Helper()

	tests := []struct {
		name                      string
		preRegister, preRegister2 *structs.NodeService
		// Use raw JSON payloads rather than encoding to avoid subtleties with some
		// internal representations and different ways they encode and decode. We
		// rely on the payload being Unmarshalable to structs.ServiceDefinition
		// directly.
		json                        string
		enableACL                   bool
		policies                    string
		wantNS                      *structs.NodeService
		wantErr                     string
		wantSidecarIDLeftAfterDereg bool
		assertStateFn               func(t *testing.T, state *local.State)
	}{
		{
			name: "sanity check no sidecar case",
			json: `
			{
				"name": "web",
				"port": 1111
			}
			`,
			wantNS:  nil,
			wantErr: "",
		},
		{
			name: "default sidecar",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {}
				}
			}
			`,
			wantNS:  testDefaultSidecar("web", 1111),
			wantErr: "",
		},
		{
			name: "ACL OK defaults",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {}
				}
			}
			`,
			enableACL: true,
			policies: `
			service "web-sidecar-proxy" {
				policy = "write"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS:  testDefaultSidecar("web", 1111),
			wantErr: "",
		},
		{
			name: "ACL denied",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {}
				}
			}
			`,
			enableACL: true,
			policies:  ``, // No policy means no valid token
			wantNS:    nil,
			wantErr:   "Permission denied",
		},
		{
			name: "ACL OK for service but not for sidecar",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {}
				}
			}
			`,
			enableACL: true,
			// This will become more common/reasonable when ACLs support exact match.
			policies: `
			service "web-sidecar-proxy" {
				policy = "deny"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS:  nil,
			wantErr: "Permission denied",
		},
		{
			name: "ACL OK for service and sidecar but not sidecar's overridden destination",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"proxy": {
							"DestinationServiceName": "foo"
						}
					}
				}
			}
			`,
			enableACL: true,
			policies: `
			service "web-sidecar-proxy" {
				policy = "write"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS:  nil,
			wantErr: "Permission denied",
		},
		{
			name: "ACL OK for service but not for overridden sidecar",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"name": "foo-sidecar-proxy"
					}
				}
			}
			`,
			enableACL: true,
			policies: `
			service "web-sidecar-proxy" {
				policy = "write"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS:  nil,
			wantErr: "Permission denied",
		},
		{
			name: "ACL OK for service but and overridden for sidecar",
			// This test ensures that if the sidecar embeds it's own token with
			// different privs from the main request token it will be honored for the
			// sidecar registration. We use the test root token since that should have
			// permission.
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"name": "foo",
						"token": "root"
					}
				}
			}
			`,
			enableACL: true,
			policies: `
			service "web-sidecar-proxy" {
				policy = "write"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS: testDefaultSidecar("web", 1111, func(ns *structs.NodeService) {
				ns.Service = "foo"
			}),
			wantErr: "",
		},
		{
			name: "invalid check definition in sidecar",
			// Note no interval in the TCP check should fail validation
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"check": {
							"TCP": "foo"
						}
					}
				}
			}
			`,
			wantNS:  nil,
			wantErr: "invalid check in sidecar_service",
		},
		{
			name: "invalid checks definitions in sidecar",
			// Note no interval in the TCP check should fail validation
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"checks": [{
							"TCP": "foo"
						}]
					}
				}
			}
			`,
			wantNS:  nil,
			wantErr: "invalid check in sidecar_service",
		},
		{
			name: "invalid check status in sidecar",
			// Note no interval in the TCP check should fail validation
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"check": {
							"TCP": "foo",
							"Interval": 10,
							"Status": "unsupported-status"
						}
					}
				}
			}
			`,
			wantNS:  nil,
			wantErr: "Status for checks must 'passing', 'warning', 'critical'",
		},
		{
			name: "invalid checks status in sidecar",
			// Note no interval in the TCP check should fail validation
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"checks": [{
							"TCP": "foo",
							"Interval": 10,
							"Status": "unsupported-status"
						}]
					}
				}
			}
			`,
			wantNS:  nil,
			wantErr: "Status for checks must 'passing', 'warning', 'critical'",
		},
		{
			name: "another service registered with same ID as a sidecar should not be deregistered",
			// Add another service with the same ID that a sidecar for web would have
			preRegister: &structs.NodeService{
				ID:      "web-sidecar-proxy",
				Service: "fake-sidecar",
				Port:    9999,
			},
			// Register web with NO SIDECAR
			json: `
			{
				"name": "web",
				"port": 1111
			}
			`,
			// Note here that although the registration here didn't register it, we
			// should still see the NodeService we pre-registered here.
			wantNS: &structs.NodeService{
				ID:              "web-sidecar-proxy",
				Service:         "fake-sidecar",
				Port:            9999,
				TaggedAddresses: map[string]structs.ServiceAddress{},
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			// After we deregister the web service above, the fake sidecar with
			// clashing ID SHOULD NOT have been removed since it wasn't part of the
			// original registration.
			wantSidecarIDLeftAfterDereg: true,
		},
		{
			name: "updates to sidecar should work",
			// Add a valid sidecar already registered
			preRegister: &structs.NodeService{
				ID:                         "web-sidecar-proxy",
				Service:                    "web-sidecar-proxy",
				LocallyRegisteredAsSidecar: true,
				Port:                       9999,
			},
			// Register web with Sidecar on different port
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"Port": 6666
					}
				}
			}
			`,
			// Note here that although the registration here didn't register it, we
			// should still see the NodeService we pre-registered here.
			wantNS: &structs.NodeService{
				Kind:                       "connect-proxy",
				ID:                         "web-sidecar-proxy",
				Service:                    "web-sidecar-proxy",
				LocallyRegisteredAsSidecar: true,
				Port:                       6666,
				TaggedAddresses:            map[string]structs.ServiceAddress{},
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1111,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
		{
			name: "update that removes sidecar should NOT deregister it",
			// Add web with a valid sidecar already registered
			preRegister: &structs.NodeService{
				ID:      "web",
				Service: "web",
				Port:    1111,
			},
			preRegister2: testDefaultSidecar("web", 1111),
			// Register (update) web and remove sidecar (and port for sanity check)
			json: `
			{
				"name": "web",
				"port": 2222
			}
			`,
			// Sidecar should still be there such that API can update registration
			// without accidentally removing a sidecar. This is equivalent to embedded
			// checks which are not removed by just not being included in an update.
			// We will document that sidecar registrations via API must be explicitiy
			// deregistered.
			wantNS: testDefaultSidecar("web", 1111),
			// Sanity check the rest of the update happened though.
			assertStateFn: func(t *testing.T, state *local.State) {
				svc := state.Service(structs.NewServiceID("web", nil))
				require.NotNil(t, svc)
				require.Equal(t, 2222, svc.Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Constrain auto ports to 1 available to make it deterministic
			hcl := `ports {
				sidecar_min_port = 2222
				sidecar_max_port = 2222
			}
			`
			if tt.enableACL {
				hcl = hcl + TestACLConfig()
			}

			a := NewTestAgent(t, hcl+" "+extraHCL)
			defer a.Shutdown()
			testrpc.WaitForLeader(t, a.RPC, "dc1")

			if tt.preRegister != nil {
				require.NoError(t, a.addServiceFromSource(tt.preRegister, nil, false, "", ConfigSourceLocal))
			}
			if tt.preRegister2 != nil {
				require.NoError(t, a.addServiceFromSource(tt.preRegister2, nil, false, "", ConfigSourceLocal))
			}

			// Create an ACL token with require policy
			var token string
			if tt.enableACL && tt.policies != "" {
				token = testCreateToken(t, a, tt.policies)
			}

			br := bytes.NewBufferString(tt.json)

			req, _ := http.NewRequest("PUT", "/v1/agent/service/register", br)
			req.Header.Add("X-Consul-Token", token)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if tt.wantErr != "" {
				require.Contains(t, strings.ToLower(resp.Body.String()), strings.ToLower(tt.wantErr))
				return
			}
			require.Equal(t, 200, resp.Code, "request failed with body: %s",
				resp.Body.String())

			// Sanity the target service registration
			svcs := a.State.AllServices()

			// Parse the expected definition into a ServiceDefinition
			var sd structs.ServiceDefinition
			err := json.Unmarshal([]byte(tt.json), &sd)
			require.NoError(t, err)
			require.NotEmpty(t, sd.Name)

			svcID := sd.ID
			if svcID == "" {
				svcID = sd.Name
			}
			sid := structs.NewServiceID(svcID, nil)
			svc, ok := svcs[sid]
			require.True(t, ok, "has service "+sid.String())
			assert.Equal(t, sd.Name, svc.Service)
			assert.Equal(t, sd.Port, svc.Port)
			// Ensure that the actual registered service _doesn't_ still have it's
			// sidecar info since it's duplicate and we don't want that synced up to
			// the catalog or included in responses particularly - it's just
			// registration syntax sugar.
			assert.Nil(t, svc.Connect.SidecarService)

			if tt.wantNS == nil {
				// Sanity check that there was no service registered, we rely on there
				// being no services at start of test so we can just use the count.
				assert.Len(t, svcs, 1, "should be no sidecar registered")
				return
			}

			// Ensure sidecar
			svc, ok = svcs[structs.NewServiceID(tt.wantNS.ID, nil)]
			require.True(t, ok, "no sidecar registered at "+tt.wantNS.ID)
			assert.Equal(t, tt.wantNS, svc)

			if tt.assertStateFn != nil {
				tt.assertStateFn(t, a.State)
			}

			// Now verify deregistration also removes sidecar (if there was one and it
			// was added via sidecar not just coincidental ID clash)
			{
				req := httptest.NewRequest("PUT",
					"/v1/agent/service/deregister/"+svcID, nil)
				req.Header.Add("X-Consul-Token", token)
				resp := httptest.NewRecorder()
				a.srv.h.ServeHTTP(resp, req)
				require.Equal(t, http.StatusOK, resp.Code)

				svcs := a.State.AllServices()
				_, ok = svcs[structs.NewServiceID(tt.wantNS.ID, nil)]
				if tt.wantSidecarIDLeftAfterDereg {
					require.True(t, ok, "removed non-sidecar service at "+tt.wantNS.ID)
				} else {
					require.False(t, ok, "sidecar not deregistered with service "+svcID)
				}
			}
		})
	}
}

// This tests local agent service registration with a sidecar service. Note we
// only test simple defaults for the sidecar here since the actual logic for
// handling sidecar defaults and port assignment is tested thoroughly in
// TestAgent_sidecarServiceFromNodeService. Note it also tests Deregister
// explicitly too since setup is identical.
func TestAgent_RegisterServiceDeregisterService_Sidecar_UDP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterServiceDeregisterService_Sidecar_UDP(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterServiceDeregisterService_Sidecar_UDP(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterServiceDeregisterService_Sidecar_UDP(t *testing.T, extraHCL string) {
	t.Helper()

	tests := []struct {
		name                      string
		preRegister, preRegister2 *structs.NodeService
		// Use raw JSON payloads rather than encoding to avoid subtleties with some
		// internal representations and different ways they encode and decode. We
		// rely on the payload being Unmarshalable to structs.ServiceDefinition
		// directly.
		json                        string
		enableACL                   bool
		policies                    string
		wantNS                      *structs.NodeService
		wantErr                     string
		wantSidecarIDLeftAfterDereg bool
		assertStateFn               func(t *testing.T, state *local.State)
	}{
		{
			name: "sanity check no sidecar case",
			json: `
			{
				"name": "web",
				"port": 1111
			}
			`,
			wantNS:  nil,
			wantErr: "",
		},
		{
			name: "default sidecar",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {}
				}
			}
			`,
			wantNS:  testDefaultSidecar("web", 1111),
			wantErr: "",
		},
		{
			name: "ACL OK defaults",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {}
				}
			}
			`,
			enableACL: true,
			policies: `
			service "web-sidecar-proxy" {
				policy = "write"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS:  testDefaultSidecar("web", 1111),
			wantErr: "",
		},
		{
			name: "ACL denied",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {}
				}
			}
			`,
			enableACL: true,
			policies:  ``, // No policies means no valid token
			wantNS:    nil,
			wantErr:   "Permission denied",
		},
		{
			name: "ACL OK for service but not for sidecar",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {}
				}
			}
			`,
			enableACL: true,
			// This will become more common/reasonable when ACLs support exact match.
			policies: `
			service "web-sidecar-proxy" {
				policy = "deny"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS:  nil,
			wantErr: "Permission denied",
		},
		{
			name: "ACL OK for service and sidecar but not sidecar's overridden destination",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"proxy": {
							"DestinationServiceName": "foo"
						}
					}
				}
			}
			`,
			enableACL: true,
			policies: `
			service "web-sidecar-proxy" {
				policy = "write"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS:  nil,
			wantErr: "Permission denied",
		},
		{
			name: "ACL OK for service but not for overridden sidecar",
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"name": "foo-sidecar-proxy"
					}
				}
			}
			`,
			enableACL: true,
			policies: `
			service "web-sidecar-proxy" {
				policy = "write"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS:  nil,
			wantErr: "Permission denied",
		},
		{
			name: "ACL OK for service but and overridden for sidecar",
			// This test ensures that if the sidecar embeds it's own token with
			// different privs from the main request token it will be honored for the
			// sidecar registration. We use the test root token since that should have
			// permission.
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"name": "foo",
						"token": "root"
					}
				}
			}
			`,
			enableACL: true,
			policies: `
			service "web-sidecar-proxy" {
				policy = "write"
			}
			service "web" {
				policy = "write"
			}`,
			wantNS: testDefaultSidecar("web", 1111, func(ns *structs.NodeService) {
				ns.Service = "foo"
			}),
			wantErr: "",
		},
		{
			name: "invalid check definition in sidecar",
			// Note no interval in the UDP check should fail validation
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"check": {
							"UDP": "foo"
						}
					}
				}
			}
			`,
			wantNS:  nil,
			wantErr: "invalid check in sidecar_service",
		},
		{
			name: "invalid checks definitions in sidecar",
			// Note no interval in the UDP check should fail validation
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"checks": [{
							"UDP": "foo"
						}]
					}
				}
			}
			`,
			wantNS:  nil,
			wantErr: "invalid check in sidecar_service",
		},
		{
			name: "invalid check status in sidecar",
			// Note no interval in the UDP check should fail validation
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"check": {
							"UDP": "foo",
							"Interval": 10,
							"Status": "unsupported-status"
						}
					}
				}
			}
			`,
			wantNS:  nil,
			wantErr: "Status for checks must 'passing', 'warning', 'critical'",
		},
		{
			name: "invalid checks status in sidecar",
			// Note no interval in the UDP check should fail validation
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"checks": [{
							"UDP": "foo",
							"Interval": 10,
							"Status": "unsupported-status"
						}]
					}
				}
			}
			`,
			wantNS:  nil,
			wantErr: "Status for checks must 'passing', 'warning', 'critical'",
		},
		{
			name: "another service registered with same ID as a sidecar should not be deregistered",
			// Add another service with the same ID that a sidecar for web would have
			preRegister: &structs.NodeService{
				ID:      "web-sidecar-proxy",
				Service: "fake-sidecar",
				Port:    9999,
			},
			// Register web with NO SIDECAR
			json: `
			{
				"name": "web",
				"port": 1111
			}
			`,
			// Note here that although the registration here didn't register it, we
			// should still see the NodeService we pre-registered here.
			wantNS: &structs.NodeService{
				ID:              "web-sidecar-proxy",
				Service:         "fake-sidecar",
				Port:            9999,
				TaggedAddresses: map[string]structs.ServiceAddress{},
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			// After we deregister the web service above, the fake sidecar with
			// clashing ID SHOULD NOT have been removed since it wasn't part of the
			// original registration.
			wantSidecarIDLeftAfterDereg: true,
		},
		{
			name: "updates to sidecar should work",
			// Add a valid sidecar already registered
			preRegister: &structs.NodeService{
				ID:                         "web-sidecar-proxy",
				Service:                    "web-sidecar-proxy",
				LocallyRegisteredAsSidecar: true,
				Port:                       9999,
			},
			// Register web with Sidecar on different port
			json: `
			{
				"name": "web",
				"port": 1111,
				"connect": {
					"SidecarService": {
						"Port": 6666
					}
				}
			}
			`,
			// Note here that although the registration here didn't register it, we
			// should still see the NodeService we pre-registered here.
			wantNS: &structs.NodeService{
				Kind:                       "connect-proxy",
				ID:                         "web-sidecar-proxy",
				Service:                    "web-sidecar-proxy",
				LocallyRegisteredAsSidecar: true,
				Port:                       6666,
				TaggedAddresses:            map[string]structs.ServiceAddress{},
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					DestinationServiceID:   "web",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       1111,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
		{
			name: "update that removes sidecar should NOT deregister it",
			// Add web with a valid sidecar already registered
			preRegister: &structs.NodeService{
				ID:      "web",
				Service: "web",
				Port:    1111,
			},
			preRegister2: testDefaultSidecar("web", 1111),
			// Register (update) web and remove sidecar (and port for sanity check)
			json: `
			{
				"name": "web",
				"port": 2222
			}
			`,
			// Sidecar should still be there such that API can update registration
			// without accidentally removing a sidecar. This is equivalent to embedded
			// checks which are not removed by just not being included in an update.
			// We will document that sidecar registrations via API must be explicitiy
			// deregistered.
			wantNS: testDefaultSidecar("web", 1111),
			// Sanity check the rest of the update happened though.
			assertStateFn: func(t *testing.T, state *local.State) {
				svc := state.Service(structs.NewServiceID("web", nil))
				require.NotNil(t, svc)
				require.Equal(t, 2222, svc.Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Constrain auto ports to 1 available to make it deterministic
			hcl := `ports {
				sidecar_min_port = 2222
				sidecar_max_port = 2222
			}
			`
			if tt.enableACL {
				hcl = hcl + TestACLConfig()
			}

			a := NewTestAgent(t, hcl+" "+extraHCL)
			defer a.Shutdown()
			testrpc.WaitForLeader(t, a.RPC, "dc1")

			if tt.preRegister != nil {
				require.NoError(t, a.addServiceFromSource(tt.preRegister, nil, false, "", ConfigSourceLocal))
			}
			if tt.preRegister2 != nil {
				require.NoError(t, a.addServiceFromSource(tt.preRegister2, nil, false, "", ConfigSourceLocal))
			}

			// Create an ACL token with require policy
			var token string
			if tt.enableACL && tt.policies != "" {
				token = testCreateToken(t, a, tt.policies)
			}

			br := bytes.NewBufferString(tt.json)

			req, _ := http.NewRequest("PUT", "/v1/agent/service/register", br)
			req.Header.Add("X-Consul-Token", token)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if tt.wantErr != "" {
				require.Contains(t, strings.ToLower(resp.Body.String()), strings.ToLower(tt.wantErr))
				return
			}
			require.Equal(t, 200, resp.Code, "request failed with body: %s",
				resp.Body.String())

			// Sanity the target service registration
			svcs := a.State.AllServices()

			// Parse the expected definition into a ServiceDefinition
			var sd structs.ServiceDefinition
			err := json.Unmarshal([]byte(tt.json), &sd)
			require.NoError(t, err)
			require.NotEmpty(t, sd.Name)

			svcID := sd.ID
			if svcID == "" {
				svcID = sd.Name
			}
			sid := structs.NewServiceID(svcID, nil)
			svc, ok := svcs[sid]
			require.True(t, ok, "has service "+sid.String())
			assert.Equal(t, sd.Name, svc.Service)
			assert.Equal(t, sd.Port, svc.Port)
			// Ensure that the actual registered service _doesn't_ still have it's
			// sidecar info since it's duplicate and we don't want that synced up to
			// the catalog or included in responses particularly - it's just
			// registration syntax sugar.
			assert.Nil(t, svc.Connect.SidecarService)

			if tt.wantNS == nil {
				// Sanity check that there was no service registered, we rely on there
				// being no services at start of test so we can just use the count.
				assert.Len(t, svcs, 1, "should be no sidecar registered")
				return
			}

			// Ensure sidecar
			svc, ok = svcs[structs.NewServiceID(tt.wantNS.ID, nil)]
			require.True(t, ok, "no sidecar registered at "+tt.wantNS.ID)
			assert.Equal(t, tt.wantNS, svc)

			if tt.assertStateFn != nil {
				tt.assertStateFn(t, a.State)
			}

			// Now verify deregistration also removes sidecar (if there was one and it
			// was added via sidecar not just coincidental ID clash)
			{
				req := httptest.NewRequest("PUT",
					"/v1/agent/service/deregister/"+svcID, nil)
				req.Header.Add("X-Consul-Token", token)
				resp := httptest.NewRecorder()
				a.srv.h.ServeHTTP(resp, req)
				require.Equal(t, http.StatusOK, resp.Code)

				svcs := a.State.AllServices()
				_, ok = svcs[structs.NewServiceID(tt.wantNS.ID, nil)]
				if tt.wantSidecarIDLeftAfterDereg {
					require.True(t, ok, "removed non-sidecar service at "+tt.wantNS.ID)
				} else {
					require.False(t, ok, "sidecar not deregistered with service "+svcID)
				}
			}
		})
	}
}

// END HERE

// This tests that connect proxy validation is done for local agent
// registration. This doesn't need to test validation exhaustively since
// that is done via a table test in the structs package.
func TestAgent_RegisterService_UnmanagedConnectProxyInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_UnmanagedConnectProxyInvalid(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_UnmanagedConnectProxyInvalid(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_UnmanagedConnectProxyInvalid(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.ServiceDefinition{
		Kind: structs.ServiceKindConnectProxy,
		Name: "connect-proxy",
		Proxy: &structs.ConnectProxyConfig{
			DestinationServiceName: "db",
		},
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	req.Header.Add("X-Consul-Token", "abc123")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "Port")

	// Ensure the service doesn't exist
	assert.Nil(t, a.State.Service(structs.NewServiceID("connect-proxy", nil)))
}

// Tests agent registration of a service that is connect native.
func TestAgent_RegisterService_ConnectNative(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ConnectNative(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ConnectNative(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_ConnectNative(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register a proxy. Note that the destination doesn't exist here on
	// this agent or in the catalog at all. This is intended and part
	// of the design.
	args := &structs.ServiceDefinition{
		Name: "web",
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Connect: &structs.ServiceConnect{
			Native: true,
		},
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// Ensure the service
	svc := a.State.Service(structs.NewServiceID("web", nil))
	require.NotNil(t, svc)
	assert.True(t, svc.Connect.Native)
}

func TestAgent_RegisterService_ScriptCheck_ExecDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ScriptCheck_ExecDisable(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ScriptCheck_ExecDisable(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_ScriptCheck_ExecDisable(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.ServiceDefinition{
		Name: "test",
		Meta: map[string]string{"hello": "world"},
		Tags: []string{"primary"},
		Port: 8000,
		Check: structs.CheckType{
			Name:       "test-check",
			Interval:   time.Second,
			ScriptArgs: []string{"true"},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	req.Header.Add("X-Consul-Token", "abc123")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if http.StatusInternalServerError != resp.Code {
		t.Fatalf("expected 500 but got %v", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "Scripts are disabled on this agent") {
		t.Fatalf("expected script disabled error, got: %s", resp.Body.String())
	}
	checkID := types.CheckID("test-check")
	require.Nil(t, a.State.Check(structs.NewCheckID(checkID, nil)), "check registered with exec disabled")
}

func TestAgent_RegisterService_ScriptCheck_ExecRemoteDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ScriptCheck_ExecRemoteDisable(t, "enable_central_service_config = false")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ScriptCheck_ExecRemoteDisable(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_ScriptCheck_ExecRemoteDisable(t *testing.T, extraHCL string) {
	t.Helper()

	a := NewTestAgent(t, `
		enable_local_script_checks = true
	`+extraHCL)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.ServiceDefinition{
		Name: "test",
		Meta: map[string]string{"hello": "world"},
		Tags: []string{"primary"},
		Port: 8000,
		Check: structs.CheckType{
			Name:       "test-check",
			Interval:   time.Second,
			ScriptArgs: []string{"true"},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	req.Header.Add("X-Consul-Token", "abc123")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if http.StatusInternalServerError != resp.Code {
		t.Fatalf("expected 500 but got %v", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "Scripts are disabled on this agent") {
		t.Fatalf("expected script disabled error, got: %s", resp.Body.String())
	}
	checkID := types.CheckID("test-check")
	require.Nil(t, a.State.Check(structs.NewCheckID(checkID, nil)), "check registered with exec disabled")
}

func TestAgent_DeregisterService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	serviceReq := AddServiceRequest{
		Service: &structs.NodeService{
			ID:      "test",
			Service: "test",
		},
		chkTypes: nil,
		persist:  false,
		token:    "",
		Source:   ConfigSourceLocal,
	}
	require.NoError(t, a.AddService(serviceReq))

	req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if http.StatusOK != resp.Code {
		t.Fatalf("expected 200 but got %v", resp.Code)
	}

	// Ensure we have a check mapping
	assert.Nil(t, a.State.Service(structs.NewServiceID("test", nil)), "have test service")
	assert.Nil(t, a.State.Check(structs.NewCheckID("test", nil)), "have test check")
}

func TestAgent_DeregisterService_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	serviceReq := AddServiceRequest{
		Service: &structs.NodeService{
			ID:      "test",
			Service: "test",
		},
		chkTypes: nil,
		persist:  false,
		token:    "",
		Source:   ConfigSourceLocal,
	}
	require.NoError(t, a.AddService(serviceReq))

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test", nil)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_ServiceMaintenance_BadRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("not enabled", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		if resp.Code != 400 {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
	})

	t.Run("no service id", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/?enable=true", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		if resp.Code != 400 {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
	})

	t.Run("bad service id", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/_nope_?enable=true", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, 404, resp.Code)
		sid := structs.NewServiceID("_nope_", nil)
		require.Contains(t, resp.Body.String(), fmt.Sprintf(`Unknown service ID %q`, sid))
	})
}

func TestAgent_ServiceMaintenance_Enable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register the service
	serviceReq := AddServiceRequest{
		Service: &structs.NodeService{
			ID:      "test",
			Service: "test",
		},
		chkTypes: nil,
		persist:  false,
		token:    "",
		Source:   ConfigSourceLocal,
	}
	require.NoError(t, a.AddService(serviceReq))

	// Force the service into maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=true&reason=broken&token=mytoken", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure the maintenance check was registered
	checkID := serviceMaintCheckID(structs.NewServiceID("test", nil))
	check := a.State.Check(checkID)
	if check == nil {
		t.Fatalf("should have registered maintenance check")
	}

	// Ensure the token was added
	if token := a.State.CheckToken(checkID); token != "mytoken" {
		t.Fatalf("expected 'mytoken', got '%s'", token)
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_ServiceMaintenance_Disable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register the service
	serviceReq := AddServiceRequest{
		Service: &structs.NodeService{
			ID:      "test",
			Service: "test",
		},
		chkTypes: nil,
		persist:  false,
		token:    "",
		Source:   ConfigSourceLocal,
	}
	require.NoError(t, a.AddService(serviceReq))

	// Force the service into maintenance mode
	if err := a.EnableServiceMaintenance(structs.NewServiceID("test", nil), "", ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Leave maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=false", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure the maintenance check was removed
	checkID := serviceMaintCheckID(structs.NewServiceID("test", nil))
	if existing := a.State.Check(checkID); existing != nil {
		t.Fatalf("should have removed maintenance check")
	}
}

func TestAgent_ServiceMaintenance_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the service.
	serviceReq := AddServiceRequest{
		Service: &structs.NodeService{
			ID:      "test",
			Service: "test",
		},
		chkTypes: nil,
		persist:  false,
		token:    "",
		Source:   ConfigSourceLocal,
	}
	require.NoError(t, a.AddService(serviceReq))

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=true&reason=broken", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=true&reason=broken&token=root", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_NodeMaintenance_BadRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Fails when no enable flag provided
	req, _ := http.NewRequest("PUT", "/v1/agent/maintenance", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if resp.Code != 400 {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestAgent_NodeMaintenance_Enable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Force the node into maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/maintenance?enable=true&reason=broken&token=mytoken", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure the maintenance check was registered
	check := a.State.Check(structs.NodeMaintCheckID)
	if check == nil {
		t.Fatalf("should have registered maintenance check")
	}

	// Check that the token was used
	if token := a.State.CheckToken(structs.NodeMaintCheckID); token != "mytoken" {
		t.Fatalf("expected 'mytoken', got '%s'", token)
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_NodeMaintenance_Disable(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Force the node into maintenance mode
	a.EnableNodeMaintenance("", "")

	// Leave maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/maintenance?enable=false", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure the maintenance check was removed
	if existing := a.State.Check(structs.NodeMaintCheckID); existing != nil {
		t.Fatalf("should have removed maintenance check")
	}
}

func TestAgent_NodeMaintenance_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/maintenance?enable=true&reason=broken", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/maintenance?enable=true&reason=broken&token=root", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestAgent_RegisterCheck_Service(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.ServiceDefinition{
		Name: "memcache",
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
	}

	// First register the service
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Now register an additional check
	checkArgs := &structs.CheckDefinition{
		Name:      "memcache_check2",
		ServiceID: "memcache",
		TTL:       15 * time.Second,
	}
	req, _ = http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(checkArgs))
	resp = httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure we have a check mapping
	result := a.State.Checks(nil)
	if _, ok := result[structs.NewCheckID("service:memcache", nil)]; !ok {
		t.Fatalf("missing memcached check")
	}
	if _, ok := result[structs.NewCheckID("memcache_check2", nil)]; !ok {
		t.Fatalf("missing memcache_check2 check")
	}

	// Make sure the new check is associated with the service
	if result[structs.NewCheckID("memcache_check2", nil)].ServiceID != "memcache" {
		t.Fatalf("bad: %#v", result[structs.NewCheckID("memcached_check2", nil)])
	}

	// Make sure the new check has the right type
	if result[structs.NewCheckID("memcache_check2", nil)].Type != "ttl" {
		t.Fatalf("expected TTL type, got %s", result[structs.NewCheckID("memcache_check2", nil)].Type)
	}
}

func TestAgent_Monitor(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("unknown log level", func(t *testing.T) {
		// Try passing an invalid log level
		req, _ := http.NewRequest("GET", "/v1/agent/monitor?loglevel=invalid", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		if http.StatusBadRequest != resp.Code {
			t.Fatalf("expected 400 but got %v", resp.Code)
		}

		substring := "Unknown log level"
		if !strings.Contains(resp.Body.String(), substring) {
			t.Fatalf("got: %s, wanted message containing: %s", resp.Body.String(), substring)
		}
	})

	t.Run("stream unstructured logs", func(t *testing.T) {
		// Try to stream logs until we see the expected log line
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/agent/monitor?loglevel=debug", nil)
			cancelCtx, cancelFunc := context.WithCancel(context.Background())
			req = req.WithContext(cancelCtx)

			resp := httptest.NewRecorder()
			codeCh := make(chan int)
			go func() {
				a.srv.h.ServeHTTP(resp, req)
				codeCh <- resp.Code
			}()

			args := &structs.ServiceDefinition{
				Name: "monitor",
				Port: 8000,
				Check: structs.CheckType{
					TTL: 15 * time.Second,
				},
			}

			registerReq, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			res := httptest.NewRecorder()
			a.srv.h.ServeHTTP(res, registerReq)
			if http.StatusOK != res.Code {
				r.Fatalf("expected 200 but got %v", res.Code)
			}

			// Wait until we have received some type of logging output
			require.Eventually(r, func() bool {
				return len(resp.Body.Bytes()) > 0
			}, 3*time.Second, 100*time.Millisecond)

			cancelFunc()
			code := <-codeCh
			require.Equal(r, http.StatusOK, code)
			got := resp.Body.String()

			// Only check a substring that we are highly confident in finding
			want := "Synced service: service="
			if !strings.Contains(got, want) {
				r.Fatalf("got %q and did not find %q", got, want)
			}
		})
	})

	t.Run("stream compressed unstructured logs", func(t *testing.T) {
		// The only purpose of this test is to see something being
		// logged. Because /v1/agent/monitor is streaming the response
		// it needs special handling with the compression.
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/agent/monitor?loglevel=debug", nil)
			// Usually this would be automatically set by transport content
			// negotiation, but since this call doesn't go through a real
			// transport, the header has to be set manually
			req.Header["Accept-Encoding"] = []string{"gzip"}
			cancelCtx, cancelFunc := context.WithCancel(context.Background())
			req = req.WithContext(cancelCtx)

			resp := httptest.NewRecorder()
			handler := a.srv.handler()
			go handler.ServeHTTP(resp, req)

			args := &structs.ServiceDefinition{
				Name: "monitor",
				Port: 8000,
				Check: structs.CheckType{
					TTL: 15 * time.Second,
				},
			}

			registerReq, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			res := httptest.NewRecorder()
			a.srv.h.ServeHTTP(res, registerReq)
			if http.StatusOK != res.Code {
				r.Fatalf("expected 200 but got %v", res.Code)
			}

			// Wait until we have received some type of logging output
			require.Eventually(r, func() bool {
				return len(resp.Body.Bytes()) > 0
			}, 3*time.Second, 100*time.Millisecond)
			cancelFunc()
		})
	})

	t.Run("stream JSON logs", func(t *testing.T) {
		// Try to stream logs until we see the expected log line
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/agent/monitor?loglevel=debug&logjson", nil)
			cancelCtx, cancelFunc := context.WithCancel(context.Background())
			req = req.WithContext(cancelCtx)

			resp := httptest.NewRecorder()
			codeCh := make(chan int)
			go func() {
				a.srv.h.ServeHTTP(resp, req)
				codeCh <- resp.Code
			}()

			args := &structs.ServiceDefinition{
				Name: "monitor",
				Port: 8000,
				Check: structs.CheckType{
					TTL: 15 * time.Second,
				},
			}

			registerReq, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			res := httptest.NewRecorder()
			a.srv.h.ServeHTTP(res, registerReq)
			if http.StatusOK != res.Code {
				r.Fatalf("expected 200 but got %v", res.Code)
			}

			// Wait until we have received some type of logging output
			require.Eventually(r, func() bool {
				return len(resp.Body.Bytes()) > 0
			}, 3*time.Second, 100*time.Millisecond)

			cancelFunc()
			code := <-codeCh
			require.Equal(r, http.StatusOK, code)

			// Each line is output as a separate JSON object, we grab the first and
			// make sure it can be unmarshalled.
			firstLine := bytes.Split(resp.Body.Bytes(), []byte("\n"))[0]
			var output map[string]interface{}
			if err := json.Unmarshal(firstLine, &output); err != nil {
				r.Fatalf("err: %v", err)
			}
		})
	})

	// hopefully catch any potential regression in serf/memberlist logging setup.
	t.Run("serf shutdown logging", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/monitor?loglevel=debug", nil)
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		req = req.WithContext(cancelCtx)

		resp := httptest.NewRecorder()
		codeCh := make(chan int)
		chStarted := make(chan struct{})
		go func() {
			close(chStarted)
			a.srv.h.ServeHTTP(resp, req)
			codeCh <- resp.Code
		}()

		<-chStarted
		require.NoError(t, a.Shutdown())

		// Wait until we have received some type of logging output
		require.Eventually(t, func() bool {
			return len(resp.Body.Bytes()) > 0
		}, 3*time.Second, 100*time.Millisecond)

		cancelFunc()
		code := <-codeCh
		require.Equal(t, http.StatusOK, code)

		got := resp.Body.String()
		want := "serf: Shutdown without a Leave"
		if !strings.Contains(got, want) {
			t.Fatalf("got %q and did not find %q", got, want)
		}
	})
}

func TestAgent_Monitor_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Try without a token.
	req, _ := http.NewRequest("GET", "/v1/agent/monitor", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	if http.StatusForbidden != resp.Code {
		t.Fatalf("expected 403 but got %v", resp.Code)
	}

	// This proves we call the ACL function, and we've got the other monitor
	// test to prove monitor works, which should be sufficient. The monitor
	// logic is a little complex to set up so isn't worth repeating again
	// here.
}

func TestAgent_TokenTriggersFullSync(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	body := func(token string) io.Reader {
		return jsonReader(&api.AgentToken{Token: token})
	}

	createNodePolicy := func(t *testing.T, a *TestAgent, policyName string) *structs.ACLPolicy {
		policy := &structs.ACLPolicy{
			Name:  policyName,
			Rules: `node_prefix "" { policy = "write" }`,
		}

		req, err := http.NewRequest("PUT", "/v1/acl/policy", jsonBody(policy))
		req.Header.Add("X-Consul-Token", "root")
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		dec := json.NewDecoder(resp.Body)
		policy = &structs.ACLPolicy{}
		require.NoError(t, dec.Decode(policy))
		return policy
	}

	createNodeToken := func(t *testing.T, a *TestAgent, policyName string) *structs.ACLToken {
		createNodePolicy(t, a, policyName)

		token := &structs.ACLToken{
			Description: "test",
			Policies: []structs.ACLTokenPolicyLink{
				{Name: policyName},
			},
		}

		req, err := http.NewRequest("PUT", "/v1/acl/token", jsonBody(token))
		req.Header.Add("X-Consul-Token", "root")
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		dec := json.NewDecoder(resp.Body)
		token = &structs.ACLToken{}
		require.NoError(t, dec.Decode(token))
		return token
	}

	cases := []struct {
		path       string
		tokenGetFn func(*token.Store) string
	}{
		{
			path:       "acl_agent_token",
			tokenGetFn: (*token.Store).AgentToken,
		},
		{
			path:       "agent",
			tokenGetFn: (*token.Store).AgentToken,
		},
		{
			path:       "acl_token",
			tokenGetFn: (*token.Store).UserToken,
		},
		{
			path:       "default",
			tokenGetFn: (*token.Store).UserToken,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			url := fmt.Sprintf("/v1/agent/token/%s", tt.path)

			a := NewTestAgent(t, `
				primary_datacenter = "dc1"

				acl {
					enabled = true
					default_policy = "deny"

					tokens {
						initial_management = "root"
						default = ""
						agent = ""
						agent_recovery = ""
						replication = ""
					}
				}
			`)
			defer a.Shutdown()
			testrpc.WaitForLeader(t, a.RPC, "dc1")

			// create node policy and token
			token := createNodeToken(t, a, "test")

			req, err := http.NewRequest("PUT", url, body(token.SecretID))
			req.Header.Add("X-Consul-Token", "root")
			require.NoError(t, err)

			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)

			require.Equal(t, http.StatusOK, resp.Code)
			require.Equal(t, token.SecretID, tt.tokenGetFn(a.tokens))

			testrpc.WaitForTestAgent(t, a.RPC, "dc1",
				testrpc.WithToken("root"),
				testrpc.WaitForAntiEntropySync())
		})
	}
}

func TestAgent_Token(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// The behavior of this handler when ACLs are disabled is vetted over
	// in TestACL_Disabled_Response since there's already good infra set
	// up over there to test this, and it calls the common function.
	a := NewTestAgent(t, `
		primary_datacenter = "dc1"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
				default = ""
				agent = ""
				agent_recovery = ""
				replication = ""
				config_file_service_registration = ""
			}
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	type tokens struct {
		user                string
		userSource          tokenStore.TokenSource
		agent               string
		agentSource         tokenStore.TokenSource
		agentRecovery       string
		agentRecoverySource tokenStore.TokenSource
		repl                string
		replSource          tokenStore.TokenSource
		registration        string
		registrationSource  tokenStore.TokenSource
	}

	resetTokens := func(init tokens) {
		a.tokens.UpdateUserToken(init.user, init.userSource)
		a.tokens.UpdateAgentToken(init.agent, init.agentSource)
		a.tokens.UpdateAgentRecoveryToken(init.agentRecovery, init.agentRecoverySource)
		a.tokens.UpdateReplicationToken(init.repl, init.replSource)
		a.tokens.UpdateConfigFileRegistrationToken(init.registration, init.registrationSource)
	}

	body := func(token string) io.Reader {
		return jsonReader(&api.AgentToken{Token: token})
	}

	badJSON := func() io.Reader {
		return jsonReader(false)
	}

	tests := []struct {
		name        string
		method, url string
		body        io.Reader
		code        int
		init        tokens
		raw         tokens
		effective   tokens
		expectedErr string
	}{
		{
			name:        "bad token name",
			method:      "PUT",
			url:         "nope",
			body:        body("X"),
			code:        http.StatusNotFound,
			expectedErr: `Token "nope" is unknown`,
		},
		{
			name:        "bad JSON",
			method:      "PUT",
			url:         "acl_token",
			body:        badJSON(),
			code:        http.StatusBadRequest,
			expectedErr: `Request decode failed: json: cannot unmarshal bool into Go value of type api.AgentToken`,
		},
		{
			name:      "set user legacy",
			method:    "PUT",
			url:       "acl_token",
			body:      body("U"),
			code:      http.StatusOK,
			raw:       tokens{user: "U", userSource: tokenStore.TokenSourceAPI},
			effective: tokens{user: "U", agent: "U"},
		},
		{
			name:      "set default",
			method:    "PUT",
			url:       "default",
			body:      body("U"),
			code:      http.StatusOK,
			raw:       tokens{user: "U", userSource: tokenStore.TokenSourceAPI},
			effective: tokens{user: "U", agent: "U"},
		},
		{
			name:      "set agent legacy",
			method:    "PUT",
			url:       "acl_agent_token",
			body:      body("A"),
			code:      http.StatusOK,
			init:      tokens{user: "U", agent: "U"},
			raw:       tokens{user: "U", agent: "A", agentSource: tokenStore.TokenSourceAPI},
			effective: tokens{user: "U", agent: "A"},
		},
		{
			name:      "set agent",
			method:    "PUT",
			url:       "agent",
			body:      body("A"),
			code:      http.StatusOK,
			init:      tokens{user: "U", agent: "U"},
			raw:       tokens{user: "U", agent: "A", agentSource: tokenStore.TokenSourceAPI},
			effective: tokens{user: "U", agent: "A"},
		},
		{
			name:      "set master legacy",
			method:    "PUT",
			url:       "acl_agent_master_token",
			body:      body("M"),
			code:      http.StatusOK,
			raw:       tokens{agentRecovery: "M", agentRecoverySource: tokenStore.TokenSourceAPI},
			effective: tokens{agentRecovery: "M"},
		},
		{
			name:      "set master",
			method:    "PUT",
			url:       "agent_master",
			body:      body("M"),
			code:      http.StatusOK,
			raw:       tokens{agentRecovery: "M", agentRecoverySource: tokenStore.TokenSourceAPI},
			effective: tokens{agentRecovery: "M"},
		},
		{
			name:      "set recovery",
			method:    "PUT",
			url:       "agent_recovery",
			body:      body("R"),
			code:      http.StatusOK,
			raw:       tokens{agentRecovery: "R", agentRecoverySource: tokenStore.TokenSourceAPI},
			effective: tokens{agentRecovery: "R", agentRecoverySource: tokenStore.TokenSourceAPI},
		},
		{
			name:      "set repl legacy",
			method:    "PUT",
			url:       "acl_replication_token",
			body:      body("R"),
			code:      http.StatusOK,
			raw:       tokens{repl: "R", replSource: tokenStore.TokenSourceAPI},
			effective: tokens{repl: "R"},
		},
		{
			name:      "set repl",
			method:    "PUT",
			url:       "replication",
			body:      body("R"),
			code:      http.StatusOK,
			raw:       tokens{repl: "R", replSource: tokenStore.TokenSourceAPI},
			effective: tokens{repl: "R"},
		},
		{
			name:      "set registration",
			method:    "PUT",
			url:       "config_file_service_registration",
			body:      body("G"),
			code:      http.StatusOK,
			raw:       tokens{registration: "G", registrationSource: tokenStore.TokenSourceAPI},
			effective: tokens{registration: "G"},
		},
		{
			name:   "clear user legacy",
			method: "PUT",
			url:    "acl_token",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{user: "U"},
			raw:    tokens{userSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear default",
			method: "PUT",
			url:    "default",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{user: "U"},
			raw:    tokens{userSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear agent legacy",
			method: "PUT",
			url:    "acl_agent_token",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{agent: "A"},
			raw:    tokens{agentSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear agent",
			method: "PUT",
			url:    "agent",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{agent: "A"},
			raw:    tokens{agentSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear master legacy",
			method: "PUT",
			url:    "acl_agent_master_token",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{agentRecovery: "M"},
			raw:    tokens{agentRecoverySource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear master",
			method: "PUT",
			url:    "agent_master",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{agentRecovery: "M"},
			raw:    tokens{agentRecoverySource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear recovery",
			method: "PUT",
			url:    "agent_recovery",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{agentRecovery: "R"},
			raw:    tokens{agentRecoverySource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear repl legacy",
			method: "PUT",
			url:    "acl_replication_token",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{repl: "R"},
			raw:    tokens{replSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear repl",
			method: "PUT",
			url:    "replication",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{repl: "R"},
			raw:    tokens{replSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear registration",
			method: "PUT",
			url:    "config_file_service_registration",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{registration: "G"},
			raw:    tokens{registrationSource: tokenStore.TokenSourceAPI},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTokens(tt.init)
			url := fmt.Sprintf("/v1/agent/token/%s", tt.url)
			resp := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, url, tt.body)
			req.Header.Add("X-Consul-Token", "root")

			a.srv.h.ServeHTTP(resp, req)
			require.Equal(t, tt.code, resp.Code)
			if tt.expectedErr != "" {
				require.Contains(t, resp.Body.String(), tt.expectedErr)
				return
			}
			require.Equal(t, tt.effective.user, a.tokens.UserToken())
			require.Equal(t, tt.effective.agent, a.tokens.AgentToken())
			require.Equal(t, tt.effective.agentRecovery, a.tokens.AgentRecoveryToken())
			require.Equal(t, tt.effective.repl, a.tokens.ReplicationToken())
			require.Equal(t, tt.effective.registration, a.tokens.ConfigFileRegistrationToken())

			tok, src := a.tokens.UserTokenAndSource()
			require.Equal(t, tt.raw.user, tok)
			require.Equal(t, tt.raw.userSource, src)

			tok, src = a.tokens.AgentTokenAndSource()
			require.Equal(t, tt.raw.agent, tok)
			require.Equal(t, tt.raw.agentSource, src)

			tok, src = a.tokens.AgentRecoveryTokenAndSource()
			require.Equal(t, tt.raw.agentRecovery, tok)
			require.Equal(t, tt.raw.agentRecoverySource, src)

			tok, src = a.tokens.ReplicationTokenAndSource()
			require.Equal(t, tt.raw.repl, tok)
			require.Equal(t, tt.raw.replSource, src)

			tok, src = a.tokens.ConfigFileRegistrationTokenAndSource()
			require.Equal(t, tt.raw.registration, tok)
			require.Equal(t, tt.raw.registrationSource, src)
		})
	}

	// This one returns an error that is interpreted by the HTTP wrapper, so
	// doesn't fit into our table above.
	t.Run("permission denied", func(t *testing.T) {
		resetTokens(tokens{})
		req, _ := http.NewRequest("PUT", "/v1/agent/token/acl_token", body("X"))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		require.Equal(t, http.StatusForbidden, resp.Code)
		require.Equal(t, "", a.tokens.UserToken())
	})
}

func TestAgentConnectCARoots_empty(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "connect { enabled = false }")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Contains(t, resp.Body.String(), "Connect must be enabled")
}

func TestAgentConnectCARoots_list(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Disable peering to avoid setting up a roots watch for the server certificate,
	// which leads to cache hit on the first query below.
	a := NewTestAgent(t, "peering { enabled = false }")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Set some CAs. Note that NewTestAgent already bootstraps one CA so this just
	// adds a second and makes it active.
	ca2 := connect.TestCAConfigSet(t, a, nil)

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	dec := json.NewDecoder(resp.Body)
	value := &structs.IndexedCARoots{}
	require.NoError(t, dec.Decode(value))

	assert.Equal(t, value.ActiveRootID, ca2.ID)
	// Would like to assert that it's the same as the TestAgent domain but the
	// only way to access that state via this package is by RPC to the server
	// implementation running in TestAgent which is more or less a tautology.
	assert.NotEmpty(t, value.TrustDomain)
	assert.Len(t, value.Roots, 2)

	// We should never have the secret information
	for _, r := range value.Roots {
		assert.Equal(t, "", r.SigningCert)
		assert.Equal(t, "", r.SigningKey)
	}

	assert.Equal(t, "MISS", resp.Header().Get("X-Cache"))

	// Test caching
	{
		// List it again
		resp2 := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp2, req)

		dec := json.NewDecoder(resp2.Body)
		value2 := &structs.IndexedCARoots{}
		require.NoError(t, dec.Decode(value2))
		assert.Equal(t, value, value2)

		// Should cache hit this time and not make request
		assert.Equal(t, "HIT", resp2.Header().Get("X-Cache"))
	}

	// Test that caching is updated in the background
	{
		// Set a new CA
		ca := connect.TestCAConfigSet(t, a, nil)

		retry.Run(t, func(r *retry.R) {
			// List it again
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)

			dec := json.NewDecoder(resp.Body)
			value := &structs.IndexedCARoots{}
			require.NoError(r, dec.Decode(value))
			if ca.ID != value.ActiveRootID {
				r.Fatalf("%s != %s", ca.ID, value.ActiveRootID)
			}
			// There are now 3 CAs because we didn't complete rotation on the original
			// 2
			if len(value.Roots) != 3 {
				r.Fatalf("bad len: %d", len(value.Roots))
			}

			// Should be a cache hit! The data should've updated in the cache
			// in the background so this should've been fetched directly from
			// the cache.
			if resp.Header().Get("X-Cache") != "HIT" {
				r.Fatalf("should be a cache hit")
			}
		})
	}
}

func TestAgentConnectCALeafCert_aclDefaultDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

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
			Connect: &structs.ServiceConnect{},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(reg))
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, 200, resp.Code, "body: %s", resp.Body.String())
	}

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)
}

func TestAgentConnectCALeafCert_aclServiceWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

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
			Connect: &structs.ServiceConnect{},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(reg))
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, 200, resp.Code, "body: %s", resp.Body.String())
	}

	token := createACLTokenWithServicePolicy(t, a.srv, "write")

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	req.Header.Add("X-Consul-Token", token)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	// Get the issued cert
	dec := json.NewDecoder(resp.Body)
	value := &structs.IssuedCert{}
	require.NoError(t, dec.Decode(value))
	require.NotNil(t, value)
}

func createACLTokenWithServicePolicy(t *testing.T, srv *HTTPHandlers, policy string) string {
	policyReq := &structs.ACLPolicy{
		Name:  "service-test-write",
		Rules: fmt.Sprintf(`service "test" { policy = "%v" }`, policy),
	}

	req, _ := http.NewRequest("PUT", "/v1/acl/policy", jsonReader(policyReq))
	req.Header.Add("X-Consul-Token", "root")
	resp := httptest.NewRecorder()
	_, err := srv.ACLPolicyCreate(resp, req)
	require.NoError(t, err)

	tokenReq := &structs.ACLToken{
		Description: "token-for-test",
		Policies:    []structs.ACLTokenPolicyLink{{Name: "service-test-write"}},
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/token", jsonReader(tokenReq))
	req.Header.Add("X-Consul-Token", "root")
	resp = httptest.NewRecorder()
	srv.h.ServeHTTP(resp, req)

	dec := json.NewDecoder(resp.Body)
	svcToken := &structs.ACLToken{}
	require.NoError(t, dec.Decode(svcToken))
	return svcToken.SecretID
}

func TestAgentConnectCALeafCert_aclServiceReadDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

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
			Connect: &structs.ServiceConnect{},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(reg))
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, 200, resp.Code, "body: %s", resp.Body.String())
	}

	token := createACLTokenWithServicePolicy(t, a.srv, "read")

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	req.Header.Add("X-Consul-Token", token)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)
}

func TestAgentConnectCALeafCert_good(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := StartTestAgent(t, TestAgent{Overrides: `
		connect {
			test_ca_leaf_root_change_spread = "1ns"
		}
	`})
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

	// CA already setup by default by NewTestAgent but force a new one so we can
	// verify it was signed easily.
	ca1 := connect.TestCAConfigSet(t, a, nil)

	{
		// Register a local service
		args := &structs.ServiceDefinition{
			ID:      "foo",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
		}
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		if !assert.Equal(t, 200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, "MISS", resp.Header().Get("X-Cache"))

	// Get the issued cert
	dec := json.NewDecoder(resp.Body)
	issued := &structs.IssuedCert{}
	require.NoError(t, dec.Decode(issued))

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, ca1)

	// Verify blocking index
	assert.True(t, issued.ModifyIndex > 0)
	assert.Equal(t, fmt.Sprintf("%d", issued.ModifyIndex),
		resp.Header().Get("X-Consul-Index"))

	index := resp.Header().Get("X-Consul-Index")

	// Test caching
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		dec := json.NewDecoder(resp.Body)
		issued2 := &structs.IssuedCert{}
		require.NoError(t, dec.Decode(issued2))
		require.Equal(t, issued, issued2)
	}

	replyCh := make(chan *httptest.ResponseRecorder, 1)

	go func(index string) {
		resp := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?index="+index, nil)
		a.srv.h.ServeHTTP(resp, req)

		replyCh <- resp
	}(index)

	// Set a new CA
	ca2 := connect.TestCAConfigSet(t, a, nil)

	// Issue a blocking query to ensure that the cert gets updated appropriately
	t.Run("test blocking queries update leaf cert", func(t *testing.T) {
		var resp *httptest.ResponseRecorder
		select {
		case resp = <-replyCh:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("blocking query did not wake up during rotation")
		}
		dec := json.NewDecoder(resp.Body)
		issued2 := &structs.IssuedCert{}
		require.NoError(t, dec.Decode(issued2))
		require.NotEqual(t, issued.CertPEM, issued2.CertPEM)
		require.NotEqual(t, issued.PrivateKeyPEM, issued2.PrivateKeyPEM)

		// Verify that the cert is signed by the new CA
		requireLeafValidUnderCA(t, issued2, ca2)

		// Should not be a cache hit! The data was updated in response to the blocking
		// query being made.
		require.Equal(t, "MISS", resp.Header().Get("X-Cache"))
	})

	t.Run("test non-blocking queries update leaf cert", func(t *testing.T) {
		resp := httptest.NewRecorder()
		obj, err := a.srv.AgentConnectCALeafCert(resp, req)
		require.NoError(t, err)

		// Get the issued cert
		issued, ok := obj.(*structs.IssuedCert)
		assert.True(t, ok)

		// Verify that the cert is signed by the CA
		requireLeafValidUnderCA(t, issued, ca2)

		// Issue a non blocking query to ensure that the cert gets updated appropriately
		{
			// Set a new CA
			ca3 := connect.TestCAConfigSet(t, a, nil)

			req, err := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
			require.NoError(t, err)

			retry.Run(t, func(r *retry.R) {
				resp := httptest.NewRecorder()
				a.srv.h.ServeHTTP(resp, req)

				// Should not be a cache hit!
				require.Equal(r, "MISS", resp.Header().Get("X-Cache"))

				dec := json.NewDecoder(resp.Body)
				issued2 := &structs.IssuedCert{}
				require.NoError(r, dec.Decode(issued2))

				require.NotEqual(r, issued.CertPEM, issued2.CertPEM)
				require.NotEqual(r, issued.PrivateKeyPEM, issued2.PrivateKeyPEM)

				// Verify that the cert is signed by the new CA
				requireLeafValidUnderCA(r, issued2, ca3)
			})
		}
	})
}

// Test we can request a leaf cert for a service we have permission for
// but is not local to this agent.
func TestAgentConnectCALeafCert_goodNotLocal(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := StartTestAgent(t, TestAgent{Overrides: `
		connect {
			test_ca_leaf_root_change_spread = "1ns"
		}
	`})
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

	// CA already setup by default by NewTestAgent but force a new one so we can
	// verify it was signed easily.
	ca1 := connect.TestCAConfigSet(t, a, nil)

	{
		// Register a non-local service (central catalog)
		args := &structs.RegisterRequest{
			Node:    "foo",
			Address: "127.0.0.1",
			Service: &structs.NodeService{
				Service: "test",
				Address: "127.0.0.1",
				Port:    8080,
			},
		}
		req, _ := http.NewRequest("PUT", "/v1/catalog/register", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		if !assert.Equal(t, 200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, "MISS", resp.Header().Get("X-Cache"))

	// Get the issued cert
	dec := json.NewDecoder(resp.Body)
	issued := &structs.IssuedCert{}
	require.NoError(t, dec.Decode(issued))

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, ca1)

	// Verify blocking index
	assert.True(t, issued.ModifyIndex > 0)
	assert.Equal(t, fmt.Sprintf("%d", issued.ModifyIndex),
		resp.Header().Get("X-Consul-Index"))

	// Test caching
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		dec := json.NewDecoder(resp.Body)
		issued2 := &structs.IssuedCert{}
		require.NoError(t, dec.Decode(issued2))
		require.Equal(t, issued, issued2)
	}

	// Test Blocking - see https://github.com/hashicorp/consul/issues/4462
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		blockingReq, _ := http.NewRequest("GET", fmt.Sprintf("/v1/agent/connect/ca/leaf/test?wait=125ms&index=%d", issued.ModifyIndex), nil)
		doneCh := make(chan struct{})
		go func() {
			a.srv.h.ServeHTTP(resp, blockingReq)
			close(doneCh)
		}()

		select {
		case <-time.After(500 * time.Millisecond):
			require.FailNow(t, "Shouldn't block for this long - not respecting wait parameter in the query")

		case <-doneCh:
		}
	}

	// Test that caching is updated in the background
	{
		// Set a new CA
		ca := connect.TestCAConfigSet(t, a, nil)

		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			// Try and sign again (note no index/wait arg since cache should update in
			// background even if we aren't actively blocking)
			a.srv.h.ServeHTTP(resp, req)

			dec := json.NewDecoder(resp.Body)
			issued2 := &structs.IssuedCert{}
			require.NoError(r, dec.Decode(issued2))
			if issued.CertPEM == issued2.CertPEM {
				r.Fatalf("leaf has not updated")
			}

			// Got a new leaf. Sanity check it's a whole new key as well as different
			// cert.
			if issued.PrivateKeyPEM == issued2.PrivateKeyPEM {
				r.Fatalf("new leaf has same private key as before")
			}

			// Verify that the cert is signed by the new CA
			requireLeafValidUnderCA(r, issued2, ca)

			require.NotEqual(r, issued, issued2)
		})
	}
}

func TestAgentConnectCALeafCert_nonBlockingQuery_after_blockingQuery_shouldNotBlock(t *testing.T) {
	// see: https://github.com/hashicorp/consul/issues/12048

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

	{
		// Register a local service
		args := &structs.ServiceDefinition{
			ID:      "foo",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
		}
		req := httptest.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		if !assert.Equal(t, 200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	var (
		serialNumber string
		index        string
		issued       structs.IssuedCert
	)
	testutil.RunStep(t, "do initial non-blocking query", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		dec := json.NewDecoder(resp.Body)
		require.NoError(t, dec.Decode(&issued))
		serialNumber = issued.SerialNumber

		require.Equal(t, "MISS", resp.Header().Get("X-Cache"),
			"for the leaf cert cache type these are always MISS")
		index = resp.Header().Get("X-Consul-Index")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// launch goroutine for blocking query
		req := httptest.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?index="+index, nil).Clone(ctx)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
	}()

	// We just need to ensure that the above blocking query is in-flight before
	// the next step, so do a little sleep.
	time.Sleep(50 * time.Millisecond)

	// The initial non-blocking query populated the leaf cert cache entry
	// implicitly. The agent cache doesn't prune entries very often at all, so
	// in between both of these steps the data should still be there, causing
	// this to be a HIT that completes in less than 10m (the default inner leaf
	// cert blocking query timeout).
	testutil.RunStep(t, "do a non-blocking query that should not block", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		var issued2 structs.IssuedCert
		dec := json.NewDecoder(resp.Body)
		require.NoError(t, dec.Decode(&issued2))

		require.Equal(t, "HIT", resp.Header().Get("X-Cache"))

		// If this is actually returning a cached result, the serial number
		// should be unchanged.
		require.Equal(t, serialNumber, issued2.SerialNumber)

		require.Equal(t, issued, issued2)
	})
}

func TestAgentConnectCALeafCert_Vault_doesNotChurnLeafCertsAtIdle(t *testing.T) {
	ca.SkipIfVaultNotPresent(t)

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	testVault := ca.NewTestVaultServer(t)

	vaultToken := ca.CreateVaultTokenWithAttrs(t, testVault.Client(), &ca.VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	})

	a := StartTestAgent(t, TestAgent{Overrides: fmt.Sprintf(`
		connect {
			test_ca_leaf_root_change_spread = "1ns"
			ca_provider = "vault"
			ca_config {
				address = %[1]q
				token = %[2]q
				root_pki_path = "pki-root/"
				intermediate_pki_path = "pki-intermediate/"
			}
		}
	`, testVault.Addr, vaultToken)})
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

	var ca1 *structs.CARoot
	{
		args := &structs.DCSpecificRequest{Datacenter: "dc1"}
		var reply structs.IndexedCARoots
		require.NoError(t, a.RPC(context.Background(), "ConnectCA.Roots", args, &reply))
		for _, r := range reply.Roots {
			if r.ID == reply.ActiveRootID {
				ca1 = r
				break
			}
		}
		require.NotNil(t, ca1)
	}

	{
		// Register a local service
		args := &structs.ServiceDefinition{
			ID:      "foo",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
		}
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		if !assert.Equal(t, 200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, "MISS", resp.Header().Get("X-Cache"))

	// Get the issued cert
	dec := json.NewDecoder(resp.Body)
	issued := &structs.IssuedCert{}
	require.NoError(t, dec.Decode(issued))

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, ca1)

	// Verify blocking index
	assert.True(t, issued.ModifyIndex > 0)
	assert.Equal(t, fmt.Sprintf("%d", issued.ModifyIndex),
		resp.Header().Get("X-Consul-Index"))

	// Test caching
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		dec := json.NewDecoder(resp.Body)
		issued2 := &structs.IssuedCert{}
		require.NoError(t, dec.Decode(issued2))
		require.Equal(t, issued, issued2)
	}

	// Test that we aren't churning leaves for no reason at idle.
	{
		ch := make(chan error, 1)
		go func() {
			req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?index="+strconv.Itoa(int(issued.ModifyIndex)), nil)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				ch <- fmt.Errorf(resp.Body.String())
				return
			}

			dec := json.NewDecoder(resp.Body)
			issued2 := &structs.IssuedCert{}
			if err := dec.Decode(issued2); err != nil {
				ch <- err
			} else {
				if issued.CertPEM == issued2.CertPEM {
					ch <- fmt.Errorf("leaf woke up unexpectedly with same cert")
				} else {
					ch <- fmt.Errorf("leaf woke up unexpectedly with new cert")
				}
			}
		}()

		start := time.Now()
		select {
		case <-time.After(5 * time.Second):
		case err := <-ch:
			dur := time.Since(start)
			t.Fatalf("unexpected return from blocking query; leaf churned during idle period, took %s: %v", dur, err)
		}
	}
}

func TestAgentConnectCALeafCert_secondaryDC_good(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a1 := StartTestAgent(t, TestAgent{Name: "dc1", HCL: `
		datacenter = "dc1"
		primary_datacenter = "dc1"
	`, Overrides: `
		connect {
			test_ca_leaf_root_change_spread = "1ns"
		}
	`})
	defer a1.Shutdown()
	testrpc.WaitForTestAgent(t, a1.RPC, "dc1")

	a2 := StartTestAgent(t, TestAgent{Name: "dc2", HCL: `
		datacenter = "dc2"
		primary_datacenter = "dc1"
	`, Overrides: `
		connect {
			test_ca_leaf_root_change_spread = "1ns"
		}
	`})
	defer a2.Shutdown()
	testrpc.WaitForTestAgent(t, a2.RPC, "dc2")

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	_, err := a2.JoinWAN([]string{addr})
	require.NoError(t, err)

	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc2")
	retry.Run(t, func(r *retry.R) {
		if got, want := len(a1.WANMembers()), 2; got < want {
			r.Fatalf("got %d WAN members want at least %d", got, want)
		}
	})

	// CA already setup by default by NewTestAgent but force a new one so we can
	// verify it was signed easily.
	dc1_ca1 := connect.TestCAConfigSet(t, a1, nil)

	// Wait until root is updated in both dcs.
	waitForActiveCARoot(t, a1.srv, dc1_ca1)
	waitForActiveCARoot(t, a2.srv, dc1_ca1)

	{
		// Register a local service in the SECONDARY
		args := &structs.ServiceDefinition{
			ID:      "foo",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
		}
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		resp := httptest.NewRecorder()
		a2.srv.h.ServeHTTP(resp, req)
		if !assert.Equal(t, 200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	// List
	req, err := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	a2.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "MISS", resp.Header().Get("X-Cache"))

	// Get the issued cert
	dec := json.NewDecoder(resp.Body)
	issued := &structs.IssuedCert{}
	require.NoError(t, dec.Decode(issued))

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, dc1_ca1)

	// Verify blocking index
	assert.True(t, issued.ModifyIndex > 0)
	assert.Equal(t, fmt.Sprintf("%d", issued.ModifyIndex),
		resp.Header().Get("X-Consul-Index"))

	// Test caching
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		a2.srv.h.ServeHTTP(resp, req)
		dec := json.NewDecoder(resp.Body)
		issued2 := &structs.IssuedCert{}
		require.NoError(t, dec.Decode(issued2))
		require.Equal(t, issued, issued2)
	}

	// Test that we aren't churning leaves for no reason at idle.
	{
		ch := make(chan error, 1)
		go func() {
			req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?index="+strconv.Itoa(int(issued.ModifyIndex)), nil)
			resp := httptest.NewRecorder()
			a2.srv.h.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				ch <- fmt.Errorf(resp.Body.String())
				return
			}

			dec := json.NewDecoder(resp.Body)
			issued2 := &structs.IssuedCert{}
			if err := dec.Decode(issued2); err != nil {
				ch <- err
			} else {
				if issued.CertPEM == issued2.CertPEM {
					ch <- fmt.Errorf("leaf woke up unexpectedly with same cert")
				} else {
					ch <- fmt.Errorf("leaf woke up unexpectedly with new cert")
				}
			}
		}()

		start := time.Now()

		// Before applying the fix from PR-6513 this would reliably wake up
		// after ~20ms with a new cert. Since this test is necessarily a bit
		// timing dependent we'll chill out for 5 seconds which should be enough
		// time to disprove the original bug.
		select {
		case <-time.After(5 * time.Second):
		case err := <-ch:
			dur := time.Since(start)
			t.Fatalf("unexpected return from blocking query; leaf churned during idle period, took %s: %v", dur, err)
		}
	}

	// Set a new CA
	dc1_ca2 := connect.TestCAConfigSet(t, a2, nil)

	// Wait until root is updated in both dcs.
	waitForActiveCARoot(t, a1.srv, dc1_ca2)
	waitForActiveCARoot(t, a2.srv, dc1_ca2)

	// Test that caching is updated in the background
	retry.Run(t, func(r *retry.R) {
		resp := httptest.NewRecorder()
		// Try and sign again (note no index/wait arg since cache should update in
		// background even if we aren't actively blocking)
		a2.srv.h.ServeHTTP(resp, req)
		require.Equal(r, http.StatusOK, resp.Code)

		dec := json.NewDecoder(resp.Body)
		issued2 := &structs.IssuedCert{}
		require.NoError(r, dec.Decode(issued2))
		if issued.CertPEM == issued2.CertPEM {
			r.Fatalf("leaf has not updated")
		}

		// Got a new leaf. Sanity check it's a whole new key as well as different
		// cert.
		if issued.PrivateKeyPEM == issued2.PrivateKeyPEM {
			r.Fatalf("new leaf has same private key as before")
		}

		// Verify that the cert is signed by the new CA
		requireLeafValidUnderCA(r, issued2, dc1_ca2)

		require.NotEqual(r, issued, issued2)
	})
}

func waitForActiveCARoot(t *testing.T, srv *HTTPHandlers, expect *structs.CARoot) {
	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/roots", nil)
		resp := httptest.NewRecorder()
		srv.h.ServeHTTP(resp, req)
		if http.StatusOK != resp.Code {
			r.Fatalf("expected 200 but got %v", resp.Code)
		}

		dec := json.NewDecoder(resp.Body)
		roots := &structs.IndexedCARoots{}
		require.NoError(r, dec.Decode(roots))

		var root *structs.CARoot
		for _, r := range roots.Roots {
			if r.ID == roots.ActiveRootID {
				root = r
				break
			}
		}
		if root == nil {
			r.Fatal("no active root")
		}
		if root.ID != expect.ID {
			r.Fatalf("current active root is %s; waiting for %s", root.ID, expect.ID)
		}
	})
}

func requireLeafValidUnderCA(t require.TestingT, issued *structs.IssuedCert, ca *structs.CARoot) {
	leaf, intermediates, err := connect.ParseLeafCerts(issued.CertPEM)
	require.NoError(t, err)

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM([]byte(ca.RootCert)))

	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
	})
	require.NoError(t, err)

	// Verify the private key matches. tls.LoadX509Keypair does this for us!
	_, err = tls.X509KeyPair([]byte(issued.CertPEM), []byte(issued.PrivateKeyPEM))
	require.NoError(t, err)
}

func TestAgentConnectAuthorize_badBody(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	args := []string{}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "decode failed")
}

func TestAgentConnectAuthorize_noTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	args := &structs.ConnectAuthorizeRequest{}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "Target service must be specified")
}

// Client ID is not in the valid URI format
func TestAgentConnectAuthorize_idInvalidFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	args := &structs.ConnectAuthorizeRequest{
		Target:        "web",
		ClientCertURI: "tubes",
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "ClientCertURI not a valid Connect identifier")
}

// Client ID is a valid URI but its not a service URI
func TestAgentConnectAuthorize_idNotService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	args := &structs.ConnectAuthorizeRequest{
		Target:        "web",
		ClientCertURI: "spiffe://1234.consul",
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "ClientCertURI not a valid Service identifier")
}

// Test when there is an intention allowing the connection
func TestAgentConnectAuthorize_allow(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	target := "db"

	// Create some intentions
	var ixnId string
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionAllow

		require.Nil(t, a.RPC(context.Background(), "Intention.Apply", &req, &ixnId))
	}

	args := &structs.ConnectAuthorizeRequest{
		Target:        target,
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, 200, resp.Code)
	require.Equal(t, "MISS", resp.Header().Get("X-Cache"))

	dec := json.NewDecoder(resp.Body)
	obj := &connectAuthorizeResp{}
	require.NoError(t, dec.Decode(obj))
	require.True(t, obj.Authorized)
	require.Contains(t, obj.Reason, "Matched")

	// Make the request again
	{
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, 200, resp.Code)

		dec := json.NewDecoder(resp.Body)
		obj := &connectAuthorizeResp{}
		require.NoError(t, dec.Decode(obj))
		require.True(t, obj.Authorized)
		require.Contains(t, obj.Reason, "Matched")

		// That should've been a cache hit.
		require.Equal(t, "HIT", resp.Header().Get("X-Cache"))
	}

	// Change the intention
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpUpdate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.ID = ixnId
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionDeny

		require.Nil(t, a.RPC(context.Background(), "Intention.Apply", &req, &ixnId))
	}

	// Short sleep lets the cache background refresh happen
	time.Sleep(100 * time.Millisecond)

	// Make the request again
	{
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, 200, resp.Code)

		dec := json.NewDecoder(resp.Body)
		obj := &connectAuthorizeResp{}
		require.NoError(t, dec.Decode(obj))
		require.False(t, obj.Authorized)
		require.Contains(t, obj.Reason, "Matched")

		// That should've been a cache hit, too, since it updated in the
		// background.
		require.Equal(t, "HIT", resp.Header().Get("X-Cache"))
	}
}

// Test when there is an intention denying the connection
func TestAgentConnectAuthorize_deny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	target := "db"

	// Create some intentions
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionDeny

		var reply string
		assert.Nil(t, a.RPC(context.Background(), "Intention.Apply", &req, &reply))
	}

	args := &structs.ConnectAuthorizeRequest{
		Target:        target,
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	assert.Equal(t, 200, resp.Code)

	dec := json.NewDecoder(resp.Body)
	obj := &connectAuthorizeResp{}
	require.NoError(t, dec.Decode(obj))
	assert.False(t, obj.Authorized)
	assert.Contains(t, obj.Reason, "Matched")
}

// Test when there is an intention allowing service with a different trust
// domain. We allow this because migration between trust domains shouldn't cause
// an outage even if we have stale info about current trusted domains. It's safe
// because the CA root is either unique to this cluster and not used to sign
// anything external, or path validation can be used to ensure that the CA can
// only issue certs that are valid for the specific cluster trust domain at x509
// level which is enforced by TLS handshake.
func TestAgentConnectAuthorize_allowTrustDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	target := "db"

	// Create some intentions
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionAllow

		var reply string
		require.NoError(t, a.RPC(context.Background(), "Intention.Apply", &req, &reply))
	}

	{
		args := &structs.ConnectAuthorizeRequest{
			Target:        target,
			ClientCertURI: "spiffe://fake-domain.consul/ns/default/dc/dc1/svc/web",
		}
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		assert.Equal(t, 200, resp.Code)

		dec := json.NewDecoder(resp.Body)
		obj := &connectAuthorizeResp{}
		require.NoError(t, dec.Decode(obj))
		require.True(t, obj.Authorized)
		require.Contains(t, obj.Reason, "Matched")
	}
}

func TestAgentConnectAuthorize_denyWildcard(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	target := "db"

	// Create some intentions
	{
		// Deny wildcard to DB
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "*"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionDeny

		var reply string
		require.NoError(t, a.RPC(context.Background(), "Intention.Apply", &req, &reply))
	}
	{
		// Allow web to DB
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionAllow

		var reply string
		assert.Nil(t, a.RPC(context.Background(), "Intention.Apply", &req, &reply))
	}

	// Web should be allowed
	{
		args := &structs.ConnectAuthorizeRequest{
			Target:        target,
			ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
		}
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		assert.Equal(t, 200, resp.Code)

		dec := json.NewDecoder(resp.Body)
		obj := &connectAuthorizeResp{}
		require.NoError(t, dec.Decode(obj))
		assert.True(t, obj.Authorized)
		assert.Contains(t, obj.Reason, "Matched")
	}

	// API should be denied
	{
		args := &structs.ConnectAuthorizeRequest{
			Target:        target,
			ClientCertURI: connect.TestSpiffeIDService(t, "api").URI().String(),
		}
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		assert.Equal(t, 200, resp.Code)

		dec := json.NewDecoder(resp.Body)
		obj := &connectAuthorizeResp{}
		require.NoError(t, dec.Decode(obj))
		assert.False(t, obj.Authorized)
		assert.Contains(t, obj.Reason, "Matched")
	}
}

// Test that authorize fails without service:write for the target service.
func TestAgentConnectAuthorize_serviceWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	token := createACLTokenWithServicePolicy(t, a.srv, "read")

	args := &structs.ConnectAuthorizeRequest{
		Target:        "test",
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	req.Header.Add("X-Consul-Token", token)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code)
}

// Test when no intentions match w/ a default deny policy
func TestAgentConnectAuthorize_defaultDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.ConnectAuthorizeRequest{
		Target:        "foo",
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	req.Header.Add("X-Consul-Token", "root")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	assert.Equal(t, 200, resp.Code)

	dec := json.NewDecoder(resp.Body)
	obj := &connectAuthorizeResp{}
	require.NoError(t, dec.Decode(obj))
	assert.False(t, obj.Authorized)
	assert.Contains(t, obj.Reason, "Default behavior")
}

// Test when no intentions match w/ a default allow policy
func TestAgentConnectAuthorize_defaultAllow(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dc1 := "dc1"
	a := NewTestAgent(t, `
		primary_datacenter = "`+dc1+`"

		acl {
			enabled = true
			default_policy = "allow"

			tokens {
				initial_management = "root"
				agent = "root"
				agent_recovery = "towel"
			}
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, dc1)

	args := &structs.ConnectAuthorizeRequest{
		Target:        "foo",
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	req.Header.Add("X-Consul-Token", "root")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	assert.Equal(t, 200, resp.Code)

	dec := json.NewDecoder(resp.Body)
	obj := &connectAuthorizeResp{}
	require.NoError(t, dec.Decode(obj))
	assert.True(t, obj.Authorized)
	assert.Contains(t, obj.Reason, "Default behavior")
}

func TestAgent_Host(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dc1 := "dc1"
	a := NewTestAgent(t, `
		primary_datacenter = "`+dc1+`"

		acl {
			enabled = true
			default_policy = "allow"

			tokens {
				initial_management = "initial-management"
				agent = "agent"
				agent_recovery = "towel"
			}
		}
	`)
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/host", nil)
	req.Header.Add("X-Consul-Token", "initial-management")
	resp := httptest.NewRecorder()
	// TODO: AgentHost should write to response so that we can test using ServeHTTP()
	respRaw, err := a.srv.AgentHost(resp, req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotNil(t, respRaw)

	obj := respRaw.(*debug.HostInfo)
	assert.NotNil(t, obj.CollectionTime)
	assert.Empty(t, obj.Errors)
}

func TestAgent_HostBadACL(t *testing.T) {
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
	`)
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/host", nil)
	req.Header.Add("X-Consul-Token", "agent")
	resp := httptest.NewRecorder()
	// TODO: AgentHost should write to response so that we can test using ServeHTTP()
	_, err := a.srv.AgentHost(resp, req)
	assert.EqualError(t, err, "ACL not found")
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestAgent_Version(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dc1 := "dc1"
	a := NewTestAgent(t, `
		primary_datacenter = "`+dc1+`"
	`)
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/version", nil)
	// req.Header.Add("X-Consul-Token", "initial-management")
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentVersion(resp, req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotNil(t, respRaw)

	obj := respRaw.(*version.BuildInfo)
	assert.NotNil(t, obj.HumanVersion)
}

// Thie tests that a proxy with an ExposeConfig is returned as expected.
func TestAgent_Services_ExposeConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "proxy-id",
		Service: "proxy-name",
		Port:    8443,
		Proxy: structs.ConnectProxyConfig{
			Expose: structs.ExposeConfig{
				Checks: true,
				Paths: []structs.ExposePath{
					{
						ListenerPort:  8080,
						LocalPathPort: 21500,
						Protocol:      "http2",
						Path:          "/metrics",
					},
				},
			},
		},
	}
	a.State.AddServiceWithChecks(srv1, nil, "", false)

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	dec := json.NewDecoder(resp.Body)
	val := make(map[string]*api.AgentService)
	require.NoError(t, dec.Decode(&val))
	require.Len(t, val, 1)
	actual := val["proxy-id"]
	require.NotNil(t, actual)
	require.Equal(t, api.ServiceKindConnectProxy, actual.Kind)
	// Proxy.ToAPI() creates an empty Upstream list instead of keeping nil so do the same with actual.
	if actual.Proxy.Upstreams == nil {
		actual.Proxy.Upstreams = make([]api.Upstream, 0)
	}
	require.Equal(t, srv1.Proxy.ToAPI(), actual.Proxy)
}
