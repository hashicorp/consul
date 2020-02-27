package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/debug"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeReadOnlyAgentACL(t *testing.T, srv *HTTPServer) string {
	args := map[string]interface{}{
		"Name":  "User Token",
		"Type":  "client",
		"Rules": `agent "" { policy = "read" }`,
	}
	req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := srv.ACLCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	aclResp := obj.(aclCreateResponse)
	return aclResp.ID
}

func TestAgent_Services(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"master"},
		Meta: map[string]string{
			"foo": "bar",
		},
		Port: 5000,
	}
	require.NoError(t, a.State.AddService(srv1, ""))

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	obj, err := a.srv.AgentServices(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.(map[string]*api.AgentService)
	assert.Lenf(t, val, 1, "bad services: %v", obj)
	assert.Equal(t, 5000, val["mysql"].Port)
	assert.Equal(t, srv1.Meta, val["mysql"].Meta)
}

func TestAgent_ServicesFiltered(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"master"},
		Meta: map[string]string{
			"foo": "bar",
		},
		Port: 5000,
	}
	require.NoError(t, a.State.AddService(srv1, ""))

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
	require.NoError(t, a.State.AddService(srv2, ""))

	req, _ := http.NewRequest("GET", "/v1/agent/services?filter="+url.QueryEscape("foo in Meta"), nil)
	obj, err := a.srv.AgentServices(nil, req)
	require.NoError(t, err)
	val := obj.(map[string]*api.AgentService)
	require.Len(t, val, 2)

	req, _ = http.NewRequest("GET", "/v1/agent/services?filter="+url.QueryEscape("kv in Tags"), nil)
	obj, err = a.srv.AgentServices(nil, req)
	require.NoError(t, err)
	val = obj.(map[string]*api.AgentService)
	require.Len(t, val, 1)
}

// This tests that the agent services endpoint (/v1/agent/services) returns
// Connect proxies.
func TestAgent_Services_ExternalConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
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
			Upstreams:              structs.TestUpstreams(t),
		},
	}
	a.State.AddService(srv1, "")

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	obj, err := a.srv.AgentServices(nil, req)
	assert.Nil(err)
	val := obj.(map[string]*api.AgentService)
	assert.Len(val, 1)
	actual := val["db-proxy"]
	assert.Equal(api.ServiceKindConnectProxy, actual.Kind)
	assert.Equal(srv1.Proxy.ToAPI(), actual.Proxy)
}

// Thie tests that a sidecar-registered service is returned as expected.
func TestAgent_Services_Sidecar(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)
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
			Upstreams:              structs.TestUpstreams(t),
		},
	}
	a.State.AddService(srv1, "")

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	obj, err := a.srv.AgentServices(nil, req)
	require.NoError(err)
	val := obj.(map[string]*api.AgentService)
	assert.Len(val, 1)
	actual := val["db-sidecar-proxy"]
	require.NotNil(actual)
	assert.Equal(api.ServiceKindConnectProxy, actual.Kind)
	assert.Equal(srv1.Proxy.ToAPI(), actual.Proxy)

	// Sanity check that LocalRegisteredAsSidecar is not in the output (assuming
	// JSON encoding). Right now this is not the case because the services
	// endpoint happens to use the api struct which doesn't include that field,
	// but this test serves as a regression test incase we change the endpoint to
	// return the internal struct later and accidentally expose some "internal"
	// state.
	output, err := json.Marshal(obj)
	require.NoError(err)
	assert.NotContains(string(output), "LocallyRegisteredAsSidecar")
	assert.NotContains(string(output), "locally_registered_as_sidecar")
}

// This tests that a mesh gateway service is returned as expected.
func TestAgent_Services_MeshGateway(t *testing.T) {
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
	a.State.AddService(srv1, "")

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	obj, err := a.srv.AgentServices(nil, req)
	require.NoError(t, err)
	val := obj.(map[string]*api.AgentService)
	require.Len(t, val, 1)
	actual := val["mg-dc1-01"]
	require.NotNil(t, actual)
	require.Equal(t, api.ServiceKindMeshGateway, actual.Kind)
	require.Equal(t, srv1.Proxy.ToAPI(), actual.Proxy)
}

// This tests that a terminating gateway service is returned as expected.
func TestAgent_Services_TerminatingGateway(t *testing.T) {
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
	require.NoError(t, a.State.AddService(srv1, ""))

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	obj, err := a.srv.AgentServices(nil, req)
	require.NoError(t, err)
	val := obj.(map[string]*api.AgentService)
	require.Len(t, val, 1)
	actual := val["tg-dc1-01"]
	require.NotNil(t, actual)
	require.Equal(t, api.ServiceKindTerminatingGateway, actual.Kind)
	require.Equal(t, srv1.Proxy.ToAPI(), actual.Proxy)
}

func TestAgent_Services_ACLFilter(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"master"},
		Port:    5000,
	}
	a.State.AddService(srv1, "")

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
		obj, err := a.srv.AgentServices(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.(map[string]*api.AgentService)
		if len(val) != 0 {
			t.Fatalf("bad: %v", obj)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/services?token=root", nil)
		obj, err := a.srv.AgentServices(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.(map[string]*api.AgentService)
		if len(val) != 1 {
			t.Fatalf("bad: %v", obj)
		}
	})
}

func TestAgent_Service(t *testing.T) {
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
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}

	// Define an updated version. Be careful to copy it.
	updatedProxy := *sidecarProxy
	updatedProxy.Port = 9999

	// Mangle the proxy config/upstreams into the expected for with defaults and
	// API struct types.
	expectProxy := proxy
	expectProxy.Upstreams =
		structs.TestAddDefaultsToUpstreams(t, sidecarProxy.Proxy.Upstreams)

	expectedResponse := &api.AgentService{
		Kind:        api.ServiceKindConnectProxy,
		ID:          "web-sidecar-proxy",
		Service:     "web-sidecar-proxy",
		Port:        8000,
		Proxy:       expectProxy.ToAPI(),
		ContentHash: "4c7d5f8d3748be6d",
		Weights: api.AgentWeights{
			Passing: 1,
			Warning: 1,
		},
		Meta: map[string]string{},
		Tags: []string{},
	}
	fillAgentServiceEnterpriseMeta(expectedResponse, structs.DefaultEnterpriseMeta())

	// Copy and modify
	updatedResponse := *expectedResponse
	updatedResponse.Port = 9999
	updatedResponse.ContentHash = "713435ba1f5badcf"

	// Simple response for non-proxy service registered in TestAgent config
	expectWebResponse := &api.AgentService{
		ID:          "web",
		Service:     "web",
		Port:        8181,
		ContentHash: "6c247f8ffa5d1fb2",
		Weights: api.AgentWeights{
			Passing: 1,
			Warning: 1,
		},
		TaggedAddresses: map[string]api.ServiceAddress{
			"wan": api.ServiceAddress{
				Address: "198.18.0.1",
				Port:    1818,
			},
		},
		Meta: map[string]string{},
		Tags: []string{},
	}
	fillAgentServiceEnterpriseMeta(expectWebResponse, structs.DefaultEnterpriseMeta())

	tests := []struct {
		name       string
		tokenRules string
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
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(updatedProxy))
				resp := httptest.NewRecorder()
				_, err := a.srv.AgentRegisterService(resp, req)
				require.NoError(t, err)
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
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(sidecarProxy))
				resp := httptest.NewRecorder()
				_, err := a.srv.AgentRegisterService(resp, req)
				require.NoError(t, err)
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
				require.NoError(t, a.ReloadConfig(a.Config))
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
				require.NoError(t, a.ReloadConfig(&newConfig))
			},
			wantWait: 100 * time.Millisecond,
			wantCode: 200,
			wantResp: &updatedResponse,
		},
		{
			name:     "err: non-existent proxy",
			url:      "/v1/agent/service/nope",
			wantCode: 404,
		},
		{
			name: "err: bad ACL for service",
			url:  "/v1/agent/service/web-sidecar-proxy",
			// Limited token doesn't grant read to the service
			tokenRules: `
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
			tokenRules: `
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
			assert := assert.New(t)
			require := require.New(t)

			// Register the basic service to ensure it's in a known state to start.
			{
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(sidecarProxy))
				resp := httptest.NewRecorder()
				_, err := a.srv.AgentRegisterService(resp, req)
				require.NoError(err)
				require.Equal(200, resp.Code, "body: %s", resp.Body.String())
			}

			req, _ := http.NewRequest("GET", tt.url, nil)

			// Inject the root token for tests that don't care about ACL
			var token = "root"
			if tt.tokenRules != "" {
				// Create new token and use that.
				token = testCreateToken(t, a, tt.tokenRules)
			}
			req.Header.Set("X-Consul-Token", token)
			resp := httptest.NewRecorder()
			if tt.updateFunc != nil {
				go tt.updateFunc()
			}
			start := time.Now()
			obj, err := a.srv.AgentService(resp, req)
			elapsed := time.Since(start)

			if tt.wantErr != "" {
				require.Error(err)
				require.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErr))
			} else {
				require.NoError(err)
			}
			if tt.wantCode != 0 {
				require.Equal(tt.wantCode, resp.Code, "body: %s", resp.Body.String())
			}
			if tt.wantWait != 0 {
				assert.True(elapsed >= tt.wantWait, "should have waited at least %s, "+
					"took %s", tt.wantWait, elapsed)
			} else {
				assert.True(elapsed < 10*time.Millisecond, "should not have waited, "+
					"took %s", elapsed)
			}

			if tt.wantResp != nil {
				assert.Equal(tt.wantResp, obj)
				assert.Equal(tt.wantResp.ContentHash, resp.Header().Get("X-Consul-ContentHash"))
			} else {
				// Janky but Equal doesn't help here because nil !=
				// *api.AgentService((*api.AgentService)(nil))
				assert.Nil(obj)
			}
		})
	}
}

func TestAgent_Checks(t *testing.T) {
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
	a.State.AddCheck(chk1, "")

	req, _ := http.NewRequest("GET", "/v1/agent/checks", nil)
	obj, err := a.srv.AgentChecks(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.(map[types.CheckID]*structs.HealthCheck)
	if len(val) != 1 {
		t.Fatalf("bad checks: %v", obj)
	}
	if val["mysql"].Status != api.HealthPassing {
		t.Fatalf("bad check: %v", obj)
	}
}

func TestAgent_ChecksWithFilter(t *testing.T) {
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
	a.State.AddCheck(chk1, "")

	chk2 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "redis",
		Name:    "redis",
		Status:  api.HealthPassing,
	}
	a.State.AddCheck(chk2, "")

	req, _ := http.NewRequest("GET", "/v1/agent/checks?filter="+url.QueryEscape("Name == `redis`"), nil)
	obj, err := a.srv.AgentChecks(nil, req)
	require.NoError(t, err)
	val := obj.(map[types.CheckID]*structs.HealthCheck)
	require.Len(t, val, 1)
	_, ok := val["redis"]
	require.True(t, ok)
}

func TestAgent_HealthServiceByID(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	service := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	service = &structs.NodeService{
		ID:      "mysql2",
		Service: "mysql2",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	service = &structs.NodeService{
		ID:      "mysql3",
		Service: "mysql3",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	chk1 := &structs.HealthCheck{
		Node:      a.Config.NodeName,
		CheckID:   "mysql",
		Name:      "mysql",
		ServiceID: "mysql",
		Status:    api.HealthPassing,
	}
	err := a.State.AddCheck(chk1, "")
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
	err = a.State.AddCheck(chk2, "")
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
	err = a.State.AddCheck(chk3, "")
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
	err = a.State.AddCheck(chk4, "")
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
	err = a.State.AddCheck(chk5, "")
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
	err = a.State.AddCheck(chk6, "")
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	eval := func(t *testing.T, url string, expectedCode int, expected string) {
		t.Helper()
		t.Run("format=text", func(t *testing.T) {
			t.Helper()
			req, _ := http.NewRequest("GET", url+"?format=text", nil)
			resp := httptest.NewRecorder()
			data, err := a.srv.AgentHealthServiceByID(resp, req)
			codeWithPayload, ok := err.(CodeWithPayloadError)
			if !ok {
				t.Fatalf("Err: %v", err)
			}
			if got, want := codeWithPayload.StatusCode, expectedCode; got != want {
				t.Fatalf("returned bad status: expected %d, but had: %d in %#v", expectedCode, codeWithPayload.StatusCode, codeWithPayload)
			}
			body, ok := data.(string)
			if !ok {
				t.Fatalf("Cannot get result as string in := %#v", data)
			}
			if got, want := body, expected; got != want {
				t.Fatalf("got body %q want %q", got, want)
			}
			if got, want := codeWithPayload.Reason, expected; got != want {
				t.Fatalf("got body %q want %q", got, want)
			}
		})
		t.Run("format=json", func(t *testing.T) {
			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			dataRaw, err := a.srv.AgentHealthServiceByID(resp, req)
			codeWithPayload, ok := err.(CodeWithPayloadError)
			if !ok {
				t.Fatalf("Err: %v", err)
			}
			if got, want := codeWithPayload.StatusCode, expectedCode; got != want {
				t.Fatalf("returned bad status: expected %d, but had: %d in %#v", expectedCode, codeWithPayload.StatusCode, codeWithPayload)
			}
			data, ok := dataRaw.(*api.AgentServiceChecksInfo)
			if !ok {
				t.Fatalf("Cannot connvert result to JSON: %#v", dataRaw)
			}
			if codeWithPayload.StatusCode != http.StatusNotFound {
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
	err = a.State.AddCheck(nodeCheck, "")

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
	err = a.State.AddCheck(nodeCheck, "")
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	t.Run("maintenance check on node", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/id/mysql", http.StatusServiceUnavailable, "maintenance")
	})
}

func TestAgent_HealthServiceByName(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	service := &structs.NodeService{
		ID:      "mysql1",
		Service: "mysql-pool-r",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	service = &structs.NodeService{
		ID:      "mysql2",
		Service: "mysql-pool-r",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	service = &structs.NodeService{
		ID:      "mysql3",
		Service: "mysql-pool-rw",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	service = &structs.NodeService{
		ID:      "mysql4",
		Service: "mysql-pool-rw",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	service = &structs.NodeService{
		ID:      "httpd1",
		Service: "httpd",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	service = &structs.NodeService{
		ID:      "httpd2",
		Service: "httpd",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
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
	err := a.State.AddCheck(chk1, "")
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
	err = a.State.AddCheck(chk2, "")
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
	err = a.State.AddCheck(chk3, "")
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
	err = a.State.AddCheck(chk4, "")
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
	err = a.State.AddCheck(chk5, "")
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
	err = a.State.AddCheck(chk6, "")
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
	err = a.State.AddCheck(chk7, "")
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
	err = a.State.AddCheck(chk8, "")
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	eval := func(t *testing.T, url string, expectedCode int, expected string) {
		t.Helper()
		t.Run("format=text", func(t *testing.T) {
			t.Helper()
			req, _ := http.NewRequest("GET", url+"?format=text", nil)
			resp := httptest.NewRecorder()
			data, err := a.srv.AgentHealthServiceByName(resp, req)
			codeWithPayload, ok := err.(CodeWithPayloadError)
			if !ok {
				t.Fatalf("Err: %v", err)
			}
			if got, want := codeWithPayload.StatusCode, expectedCode; got != want {
				t.Fatalf("returned bad status: %d. Body: %q", resp.Code, resp.Body.String())
			}
			if got, want := codeWithPayload.Reason, expected; got != want {
				t.Fatalf("got reason %q want %q", got, want)
			}
			if got, want := data, expected; got != want {
				t.Fatalf("got body %q want %q", got, want)
			}
		})
		t.Run("format=json", func(t *testing.T) {
			t.Helper()
			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			dataRaw, err := a.srv.AgentHealthServiceByName(resp, req)
			codeWithPayload, ok := err.(CodeWithPayloadError)
			if !ok {
				t.Fatalf("Err: %v", err)
			}
			data, ok := dataRaw.([]api.AgentServiceChecksInfo)
			if !ok {
				t.Fatalf("Cannot connvert result to JSON")
			}
			if got, want := codeWithPayload.StatusCode, expectedCode; got != want {
				t.Fatalf("returned bad code: %d. Body: %#v", resp.Code, data)
			}
			if resp.Code != http.StatusNotFound {
				if codeWithPayload.Reason != expected {
					t.Fatalf("got wrong status %#v want %#v", codeWithPayload, expected)
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
	err = a.State.AddCheck(nodeCheck, "")

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
	err = a.State.AddCheck(nodeCheck, "")
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	t.Run("maintenance check on node", func(t *testing.T) {
		eval(t, "/v1/agent/health/service/name/mysql-pool-r", http.StatusServiceUnavailable, "maintenance")
	})
}

func TestAgent_HealthServicesACLEnforcement(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfigWithParams(nil))
	defer a.Shutdown()

	service := &structs.NodeService{
		ID:      "mysql1",
		Service: "mysql",
	}
	require.NoError(t, a.AddService(service, nil, false, "", ConfigSourceLocal))

	service = &structs.NodeService{
		ID:      "foo1",
		Service: "foo",
	}
	require.NoError(t, a.AddService(service, nil, false, "", ConfigSourceLocal))

	// no token
	t.Run("no-token-health-by-id", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/agent/health/service/id/mysql1", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		_, err = a.srv.AgentHealthServiceByID(resp, req)
		require.Equal(t, acl.ErrPermissionDenied, err)
	})

	t.Run("no-token-health-by-name", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/agent/health/service/name/mysql", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		_, err = a.srv.AgentHealthServiceByName(resp, req)
		require.Equal(t, acl.ErrPermissionDenied, err)
	})

	t.Run("root-token-health-by-id", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/agent/health/service/id/foo1", nil)
		require.NoError(t, err)
		req.Header.Add("X-Consul-Token", TestDefaultMasterToken)
		resp := httptest.NewRecorder()
		_, err = a.srv.AgentHealthServiceByID(resp, req)
		require.NotEqual(t, acl.ErrPermissionDenied, err)
	})

	t.Run("root-token-health-by-name", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/agent/health/service/name/foo", nil)
		require.NoError(t, err)
		req.Header.Add("X-Consul-Token", TestDefaultMasterToken)
		resp := httptest.NewRecorder()
		_, err = a.srv.AgentHealthServiceByName(resp, req)
		require.NotEqual(t, acl.ErrPermissionDenied, err)
	})
}

func TestAgent_Checks_ACLFilter(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	chk1 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "mysql",
		Name:    "mysql",
		Status:  api.HealthPassing,
	}
	a.State.AddCheck(chk1, "")

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/checks", nil)
		obj, err := a.srv.AgentChecks(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.(map[types.CheckID]*structs.HealthCheck)
		if len(val) != 0 {
			t.Fatalf("bad checks: %v", obj)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/checks?token=root", nil)
		obj, err := a.srv.AgentChecks(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.(map[types.CheckID]*structs.HealthCheck)
		if len(val) != 1 {
			t.Fatalf("bad checks: %v", obj)
		}
	})
}

func TestAgent_Self(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, `
		node_meta {
			somekey = "somevalue"
		}
	`)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
	obj, err := a.srv.AgentSelf(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	val := obj.(Self)
	if int(val.Member.Port) != a.Config.SerfPortLAN {
		t.Fatalf("incorrect port: %v", obj)
	}

	if val.DebugConfig["SerfPortLAN"].(int) != a.Config.SerfPortLAN {
		t.Fatalf("incorrect port: %v", obj)
	}

	cs, err := a.GetLANCoordinate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c := cs[a.config.SegmentName]; !reflect.DeepEqual(c, val.Coord) {
		t.Fatalf("coordinates are not equal: %v != %v", c, val.Coord)
	}
	delete(val.Meta, structs.MetaSegmentKey) // Added later, not in config.
	if !reflect.DeepEqual(a.config.NodeMeta, val.Meta) {
		t.Fatalf("meta fields are not equal: %v != %v", a.config.NodeMeta, val.Meta)
	}
}

func TestAgent_Self_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
		if _, err := a.srv.AgentSelf(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/self?token=towel", nil)
		if _, err := a.srv.AgentSelf(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/agent/self?token=%s", ro), nil)
		if _, err := a.srv.AgentSelf(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_Metrics_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/metrics", nil)
		if _, err := a.srv.AgentMetrics(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/metrics?token=towel", nil)
		if _, err := a.srv.AgentMetrics(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/agent/metrics?token=%s", ro), nil)
		if _, err := a.srv.AgentMetrics(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_Reload(t *testing.T) {
	t.Parallel()
	dc1 := "dc1"
	a := NewTestAgent(t, `
		acl_enforce_version_8 = false
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

	cfg2 := TestConfig(testutil.Logger(t), config.Source{
		Name:   "reload",
		Format: "hcl",
		Data: `
			data_dir = "` + a.Config.DataDir + `"
			node_id = "` + string(a.Config.NodeID) + `"
			node_name = "` + a.Config.NodeName + `"

			acl_enforce_version_8 = false
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

	if err := a.ReloadConfig(cfg2); err != nil {
		t.Fatalf("got error %v want nil", err)
	}
	if a.State.Service(structs.NewServiceID("redis-reloaded", nil)) == nil {
		t.Fatal("missing redis-reloaded service")
	}

	if a.config.RPCRateLimit != 2 {
		t.Fatalf("RPC rate not set correctly.  Got %v. Want 2", a.config.RPCRateLimit)
	}

	if a.config.RPCMaxBurst != 200 {
		t.Fatalf("RPC max burst not set correctly.  Got %v. Want 200", a.config.RPCMaxBurst)
	}

	for _, wp := range a.watchPlans {
		if !wp.IsStopped() {
			t.Fatalf("Reloading configs should stop watch plans of the previous configuration")
		}
	}
}

// TestAgent_ReloadDoesNotTriggerWatch Ensure watches not triggered after reload
// see https://github.com/hashicorp/consul/issues/7446
func TestAgent_ReloadDoesNotTriggerWatch(t *testing.T) {
	dc1 := "dc1"
	tmpFileRaw, err := ioutil.TempFile("", "rexec")
	require.NoError(t, err)
	tmpFile := tmpFileRaw.Name()
	defer os.Remove(tmpFile)
	handlerShell := fmt.Sprintf("(cat ; echo CONSUL_INDEX $CONSUL_INDEX) | tee '%s.atomic' ; mv '%s.atomic' '%s'", tmpFile, tmpFile, tmpFile)

	a := NewTestAgent(t, `
		acl_enforce_version_8 = false
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
			contents, err := ioutil.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("should be able to read file, but had: %#v", err)
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
	cfg2 := TestConfig(testutil.Logger(t), config.Source{
		Name:   "reload",
		Format: "hcl",
		Data: `
			data_dir = "` + a.Config.DataDir + `"
			node_id = "` + string(a.Config.NodeID) + `"
			node_name = "` + a.Config.NodeName + `"

			acl_enforce_version_8 = false
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

		if err := a.ReloadConfig(cfg2); err != nil {
			t.Fatalf("got error %v want nil", err)
		}

		// We check that reload does not go to critical
		ensureNothingCritical(r, "red-is-dead")
		ensureNothingCritical(r, "testing-agent-reload-001")

		require.NoError(t, a.updateTTLCheck(checkID, api.HealthPassing, "testing-agent-reload-002"))

		ensureNothingCritical(r, "red-is-dead")
	})
}

func TestAgent_Reload_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/reload", nil)
		if _, err := a.srv.AgentReload(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/reload?token=%s", ro), nil)
		if _, err := a.srv.AgentReload(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	// This proves we call the ACL function, and we've got the other reload
	// test to prove we do the reload, which should be sufficient.
	// The reload logic is a little complex to set up so isn't worth
	// repeating again here.
}

func TestAgent_Members(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/members", nil)
	obj, err := a.srv.AgentMembers(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.([]serf.Member)
	if len(val) == 0 {
		t.Fatalf("bad members: %v", obj)
	}

	if int(val[0].Port) != a.Config.SerfPortLAN {
		t.Fatalf("not lan: %v", obj)
	}
}

func TestAgent_Members_WAN(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/members?wan=true", nil)
	obj, err := a.srv.AgentMembers(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.([]serf.Member)
	if len(val) == 0 {
		t.Fatalf("bad members: %v", obj)
	}

	if int(val[0].Port) != a.Config.SerfPortWAN {
		t.Fatalf("not wan: %v", obj)
	}
}

func TestAgent_Members_ACLFilter(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/members", nil)
		obj, err := a.srv.AgentMembers(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.([]serf.Member)
		if len(val) != 0 {
			t.Fatalf("bad members: %v", obj)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/members?token=root", nil)
		obj, err := a.srv.AgentMembers(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.([]serf.Member)
		if len(val) != 1 {
			t.Fatalf("bad members: %v", obj)
		}
	})
}

func TestAgent_Join(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t, "")
	defer a1.Shutdown()
	a2 := NewTestAgent(t, "")
	defer a2.Shutdown()
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s", addr), nil)
	obj, err := a1.srv.AgentJoin(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}

	if len(a1.LANMembers()) != 2 {
		t.Fatalf("should have 2 members")
	}

	retry.Run(t, func(r *retry.R) {
		if got, want := len(a2.LANMembers()), 2; got != want {
			r.Fatalf("got %d LAN members want %d", got, want)
		}
	})
}

func TestAgent_Join_WAN(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t, "")
	defer a1.Shutdown()
	a2 := NewTestAgent(t, "")
	defer a2.Shutdown()
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortWAN)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s?wan=true", addr), nil)
	obj, err := a1.srv.AgentJoin(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}

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
		if _, err := a1.srv.AgentJoin(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s?token=towel", addr), nil)
		_, err := a1.srv.AgentJoin(nil, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a1.srv)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s?token=%s", addr, ro), nil)
		if _, err := a1.srv.AgentJoin(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})
}

type mockNotifier struct{ s string }

func (n *mockNotifier) Notify(state string) error {
	n.s = state
	return nil
}

func TestAgent_JoinLANNotify(t *testing.T) {
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
	_, err := a1.JoinLAN([]string{addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if got, want := notif.s, "READY=1"; got != want {
		t.Fatalf("got joinLAN notification %q want %q", got, want)
	}
}

func TestAgent_Leave(t *testing.T) {
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
	_, err := a1.JoinLAN([]string{addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Graceful leave now
	req, _ := http.NewRequest("PUT", "/v1/agent/leave", nil)
	obj, err := a2.srv.AgentLeave(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}
	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembers()
		if got, want := m[1].Status, serf.StatusLeft; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})
}

func TestAgent_Leave_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/leave", nil)
		if _, err := a.srv.AgentLeave(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/leave?token=%s", ro), nil)
		if _, err := a.srv.AgentLeave(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	// this sub-test will change the state so that there is no leader.
	// it must therefore be the last one in this list.
	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/leave?token=towel", nil)
		if _, err := a.srv.AgentLeave(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_ForceLeave(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t, "")
	defer a1.Shutdown()
	a2 := NewTestAgent(t, "")
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	// Join first
	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	_, err := a1.JoinLAN([]string{addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// this test probably needs work
	a2.Shutdown()
	// Wait for agent being marked as failed, so we wait for full shutdown of Agent
	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembers()
		if got, want := m[1].Status, serf.StatusFailed; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})

	// Force leave now
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/force-leave/%s", a2.Config.NodeName), nil)
	obj, err := a1.srv.AgentForceLeave(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}
	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembers()
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
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	uri := fmt.Sprintf("/v1/agent/force-leave/%s", a.Config.NodeName)

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", uri, nil)
		if _, err := a.srv.AgentForceLeave(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", uri+"?token=towel", nil)
		if _, err := a.srv.AgentForceLeave(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("PUT", fmt.Sprintf(uri+"?token=%s", ro), nil)
		if _, err := a.srv.AgentForceLeave(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("operator write token", func(t *testing.T) {
		// Create an ACL with operator read permissions.
		var rules = `
                    operator = "write"
                `
		opToken := testCreateToken(t, a, rules)

		req, _ := http.NewRequest("PUT", fmt.Sprintf(uri+"?token=%s", opToken), nil)
		if _, err := a.srv.AgentForceLeave(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_ForceLeavePrune(t *testing.T) {
	t.Parallel()
	a1 := StartTestAgent(t, TestAgent{Name: "Agent1"})
	defer a1.Shutdown()
	a2 := StartTestAgent(t, TestAgent{Name: "Agent2"})
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc1")

	// Join first
	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	_, err := a1.JoinLAN([]string{addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// this test probably needs work
	a2.Shutdown()
	// Wait for agent being marked as failed, so we wait for full shutdown of Agent
	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembers()
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
	obj, err := a1.srv.AgentForceLeave(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}
	retry.Run(t, func(r *retry.R) {
		m := len(a1.LANMembers())
		if m != 1 {
			r.Fatalf("want one member, got %v", m)
		}
	})

}

func TestAgent_RegisterCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.CheckDefinition{
		Name: "test",
		TTL:  15 * time.Second,
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register?token=abc123", jsonReader(args))
	obj, err := a.srv.AgentRegisterCheck(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

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

// This verifies all the forms of the new args-style check that we need to
// support as a result of https://github.com/hashicorp/consul/issues/3587.
func TestAgent_RegisterCheck_Scripts(t *testing.T) {
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
			if _, err := a.srv.AgentRegisterCheck(resp, req); err != nil {
				t.Fatalf("err: %v", err)
			}
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
			if _, err := a.srv.AgentRegisterService(resp, req); err != nil {
				t.Fatalf("err: %v", err)
			}
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
			if _, err := a.srv.AgentRegisterService(resp, req); err != nil {
				t.Fatalf("err: %v", err)
			}
			if resp.Code != http.StatusOK {
				t.Fatalf("bad: %d", resp.Code)
			}
		})
	}
}

func TestAgent_RegisterCheckScriptsExecDisable(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.CheckDefinition{
		Name:       "test",
		ScriptArgs: []string{"true"},
		Interval:   time.Second,
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register?token=abc123", jsonReader(args))
	res := httptest.NewRecorder()
	_, err := a.srv.AgentRegisterCheck(res, req)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("expected script disabled error, got: %s", err)
	}
	checkID := structs.NewCheckID("test", nil)
	require.Nil(t, a.State.Check(checkID), "check registered with exec disabled")
}

func TestAgent_RegisterCheckScriptsExecRemoteDisable(t *testing.T) {
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
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register?token=abc123", jsonReader(args))
	res := httptest.NewRecorder()
	_, err := a.srv.AgentRegisterCheck(res, req)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("expected script disabled error, got: %s", err)
	}
	checkID := structs.NewCheckID("test", nil)
	require.Nil(t, a.State.Check(checkID), "check registered with exec disabled")
}

func TestAgent_RegisterCheck_Passing(t *testing.T) {
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
	obj, err := a.srv.AgentRegisterCheck(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
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
	if _, err := a.srv.AgentRegisterCheck(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("accepted bad status")
	}
}

func TestAgent_RegisterCheck_ACLDeny(t *testing.T) {
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
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(svc))
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentRegisterService(resp, req)
	require.NoError(t, err)

	// create a policy that has write on service foo
	policyReq := &structs.ACLPolicy{
		Name:  "write-foo",
		Rules: `service "foo" { policy = "write"}`,
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/policy?token=root", jsonReader(policyReq))
	resp = httptest.NewRecorder()
	_, err = a.srv.ACLPolicyCreate(resp, req)
	require.NoError(t, err)

	// create a policy that has write on the node name of the agent
	policyReq = &structs.ACLPolicy{
		Name:  "write-node",
		Rules: fmt.Sprintf(`node "%s" { policy = "write" }`, a.config.NodeName),
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/policy?token=root", jsonReader(policyReq))
	resp = httptest.NewRecorder()
	_, err = a.srv.ACLPolicyCreate(resp, req)
	require.NoError(t, err)

	// create a token using the write-foo policy
	tokenReq := &structs.ACLToken{
		Description: "write-foo",
		Policies: []structs.ACLTokenPolicyLink{
			structs.ACLTokenPolicyLink{
				Name: "write-foo",
			},
		},
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/token?token=root", jsonReader(tokenReq))
	resp = httptest.NewRecorder()
	tokInf, err := a.srv.ACLTokenCreate(resp, req)
	require.NoError(t, err)
	svcToken, ok := tokInf.(*structs.ACLToken)
	require.True(t, ok)
	require.NotNil(t, svcToken)

	// create a token using the write-node policy
	tokenReq = &structs.ACLToken{
		Description: "write-node",
		Policies: []structs.ACLTokenPolicyLink{
			structs.ACLTokenPolicyLink{
				Name: "write-node",
			},
		},
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/token?token=root", jsonReader(tokenReq))
	resp = httptest.NewRecorder()
	tokInf, err = a.srv.ACLTokenCreate(resp, req)
	require.NoError(t, err)
	nodeToken, ok := tokInf.(*structs.ACLToken)
	require.True(t, ok)
	require.NotNil(t, nodeToken)

	t.Run("no token - node check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(nodeCheck))
			_, err := a.srv.AgentRegisterCheck(nil, req)
			require.True(r, acl.IsErrPermissionDenied(err))
		})
	})

	t.Run("svc token - node check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register?token="+svcToken.SecretID, jsonReader(nodeCheck))
			_, err := a.srv.AgentRegisterCheck(nil, req)
			require.True(r, acl.IsErrPermissionDenied(err))
		})
	})

	t.Run("node token - node check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register?token="+nodeToken.SecretID, jsonReader(nodeCheck))
			_, err := a.srv.AgentRegisterCheck(nil, req)
			require.NoError(r, err)
		})
	})

	t.Run("no token - svc check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(svcCheck))
			_, err := a.srv.AgentRegisterCheck(nil, req)
			require.True(r, acl.IsErrPermissionDenied(err))
		})
	})

	t.Run("node token - svc check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register?token="+nodeToken.SecretID, jsonReader(svcCheck))
			_, err := a.srv.AgentRegisterCheck(nil, req)
			require.True(r, acl.IsErrPermissionDenied(err))
		})
	})

	t.Run("svc token - svc check", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register?token="+svcToken.SecretID, jsonReader(svcCheck))
			_, err := a.srv.AgentRegisterCheck(nil, req)
			require.NoError(r, err)
		})
	})

}

func TestAgent_DeregisterCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	if err := a.AddCheck(chk, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/test", nil)
	obj, err := a.srv.AgentDeregisterCheck(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	requireCheckMissing(t, a, "test")
}

func TestAgent_DeregisterCheckACLDeny(t *testing.T) {
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
		if _, err := a.srv.AgentDeregisterCheck(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/test?token=root", nil)
		if _, err := a.srv.AgentDeregisterCheck(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_PassCheck(t *testing.T) {
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
	obj, err := a.srv.AgentCheckPass(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	state := a.State.Check(structs.NewCheckID("test", nil))
	if state.Status != api.HealthPassing {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_PassCheck_ACLDeny(t *testing.T) {
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
		if _, err := a.srv.AgentCheckPass(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/pass/test?token=root", nil)
		if _, err := a.srv.AgentCheckPass(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_WarnCheck(t *testing.T) {
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
	obj, err := a.srv.AgentCheckWarn(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	state := a.State.Check(structs.NewCheckID("test", nil))
	if state.Status != api.HealthWarning {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_WarnCheck_ACLDeny(t *testing.T) {
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
		if _, err := a.srv.AgentCheckWarn(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/warn/test?token=root", nil)
		if _, err := a.srv.AgentCheckWarn(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_FailCheck(t *testing.T) {
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
	obj, err := a.srv.AgentCheckFail(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	state := a.State.Check(structs.NewCheckID("test", nil))
	if state.Status != api.HealthCritical {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_FailCheck_ACLDeny(t *testing.T) {
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
		if _, err := a.srv.AgentCheckFail(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/fail/test?token=root", nil)
		if _, err := a.srv.AgentCheckFail(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_UpdateCheck(t *testing.T) {
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
		checkUpdate{api.HealthPassing, "hello-passing"},
		checkUpdate{api.HealthCritical, "hello-critical"},
		checkUpdate{api.HealthWarning, "hello-warning"},
	}

	for _, c := range cases {
		t.Run(c.Status, func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(c))
			resp := httptest.NewRecorder()
			obj, err := a.srv.AgentCheckUpdate(resp, req)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if obj != nil {
				t.Fatalf("bad: %v", obj)
			}
			if resp.Code != 200 {
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
		obj, err := a.srv.AgentCheckUpdate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if obj != nil {
			t.Fatalf("bad: %v", obj)
		}
		if resp.Code != 200 {
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
		obj, err := a.srv.AgentCheckUpdate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if obj != nil {
			t.Fatalf("bad: %v", obj)
		}
		if resp.Code != 400 {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
	})
}

func TestAgent_UpdateCheck_ACLDeny(t *testing.T) {
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
		if _, err := a.srv.AgentCheckUpdate(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		args := checkUpdate{api.HealthPassing, "hello-passing"}
		req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test?token=root", jsonReader(args))
		if _, err := a.srv.AgentCheckUpdate(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_RegisterService(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService(t, "")
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
		Tags: []string{"master"},
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Checks: []*structs.CheckType{
			&structs.CheckType{
				TTL: 20 * time.Second,
			},
			&structs.CheckType{
				TTL: 30 * time.Second,
			},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))

	obj, err := a.srv.AgentRegisterService(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
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
	checks := a.State.Checks(structs.WildcardEnterpriseMeta())
	if len(checks) != 3 {
		t.Fatalf("bad: %v", checks)
	}
	for _, c := range checks {
		if c.Type != "ttl" {
			t.Fatalf("expected ttl check type, got %s", c.Type)
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
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ReRegister(t, "")
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
		Tags: []string{"master"},
		Port: 8000,
		Checks: []*structs.CheckType{
			&structs.CheckType{
				CheckID: types.CheckID("check_1"),
				TTL:     20 * time.Second,
			},
			&structs.CheckType{
				CheckID: types.CheckID("check_2"),
				TTL:     30 * time.Second,
			},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	_, err := a.srv.AgentRegisterService(nil, req)
	require.NoError(t, err)

	args = &structs.ServiceDefinition{
		Name: "test",
		Meta: map[string]string{"hello": "world"},
		Tags: []string{"master"},
		Port: 8000,
		Checks: []*structs.CheckType{
			&structs.CheckType{
				CheckID: types.CheckID("check_1"),
				TTL:     20 * time.Second,
			},
			&structs.CheckType{
				CheckID: types.CheckID("check_3"),
				TTL:     30 * time.Second,
			},
		},
		Weights: &structs.Weights{
			Passing: 100,
			Warning: 3,
		},
	}
	req, _ = http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	_, err = a.srv.AgentRegisterService(nil, req)
	require.NoError(t, err)

	checks := a.State.Checks(structs.DefaultEnterpriseMeta())
	require.Equal(t, 3, len(checks))

	checkIDs := []string{}
	for id := range checks {
		checkIDs = append(checkIDs, string(id.ID))
	}
	require.ElementsMatch(t, []string{"check_1", "check_2", "check_3"}, checkIDs)
}

func TestAgent_RegisterService_ReRegister_ReplaceExistingChecks(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ReRegister_ReplaceExistingChecks(t, "")
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
		Tags: []string{"master"},
		Port: 8000,
		Checks: []*structs.CheckType{
			&structs.CheckType{
				// explicitly not setting the check id to let it be auto-generated
				// we want to ensure that we are testing out the cases with autogenerated names/ids
				TTL: 20 * time.Second,
			},
			&structs.CheckType{
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
	_, err := a.srv.AgentRegisterService(nil, req)
	require.NoError(t, err)

	args = &structs.ServiceDefinition{
		Name: "test",
		Meta: map[string]string{"hello": "world"},
		Tags: []string{"master"},
		Port: 8000,
		Checks: []*structs.CheckType{
			&structs.CheckType{
				TTL: 20 * time.Second,
			},
			&structs.CheckType{
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
	_, err = a.srv.AgentRegisterService(nil, req)
	require.NoError(t, err)

	checks := a.State.Checks(structs.DefaultEnterpriseMeta())
	require.Len(t, checks, 2)

	checkIDs := []string{}
	for id := range checks {
		checkIDs = append(checkIDs, string(id.ID))
	}
	require.ElementsMatch(t, []string{"service:test:1", "check_3"}, checkIDs)
}

func TestAgent_RegisterService_TranslateKeys(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ACLDeny(t, "")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ACLDeny(t, "enable_central_service_config = true")
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
			obj, err := a.srv.AgentRegisterService(rr, req)
			require.NoError(t, err)
			require.Nil(t, obj)
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
							LocalBindAddress:     tt.ip,
							LocalBindPort:        1234,
							Config: map[string]interface{}{
								"destination_type": "sidecar_service.proxy.upstreams.config is 'opaque' so should not get translated",
							},
						},
					},
				},
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
				t.Fatalf("Did not find the expected TCP Healtcheck '%s' in %#v ", tt.expectedTCPCheckStart, a.checkTCPs)
			}
			require.Equal(t, sidecarSvc, gotSidecar)
		})
	}
}

func TestAgent_RegisterService_ACLDeny(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ACLDeny(t, "")
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
		Tags: []string{"master"},
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Checks: []*structs.CheckType{
			&structs.CheckType{
				TTL: 20 * time.Second,
			},
			&structs.CheckType{
				TTL: 30 * time.Second,
			},
		},
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		if _, err := a.srv.AgentRegisterService(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(args))
		if _, err := a.srv.AgentRegisterService(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_RegisterService_InvalidAddress(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_UnmanagedConnectProxy(t, "")
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
			req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))
			resp := httptest.NewRecorder()
			_, err := a.srv.AgentRegisterService(resp, req)
			if err != nil {
				t.Fatalf("got error %v want nil", err)
			}
			if got, want := resp.Code, 400; got != want {
				t.Fatalf("got code %d want %d", got, want)
			}
			if got, want := resp.Body.String(), "Invalid service address"; got != want {
				t.Fatalf("got body %q want %q", got, want)
			}
		})
	}
}

// This tests local agent service registration of a unmanaged connect proxy.
// This verifies that it is put in the local state store properly for syncing
// later.
func TestAgent_RegisterService_UnmanagedConnectProxy(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_UnmanagedConnectProxy(t, "")
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
		},
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentRegisterService(resp, req)
	require.NoError(t, err)
	require.Nil(t, obj)

	// Ensure the service
	sid := structs.NewServiceID("connect-proxy", nil)
	svc := a.State.Service(sid)
	require.NotNil(t, svc, "has service")
	require.Equal(t, structs.ServiceKindConnectProxy, svc.Kind)
	// Registration must set that default type
	args.Proxy.Upstreams[0].DestinationType = api.UpstreamDestTypeService
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
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
			map[string]interface{}{
				"ID": policyID,
			},
		},
		"Local": false,
	}
	req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.ACLTokenCreate(resp, req)
	require.NoError(t, err)
	require.NotNil(t, obj)
	aclResp := obj.(*structs.ACLToken)
	return aclResp.SecretID
}

func testCreatePolicy(t *testing.T, a *TestAgent, name, rules string) string {
	args := map[string]interface{}{
		"Name":  name,
		"Rules": rules,
	}
	req, _ := http.NewRequest("PUT", "/v1/acl/policy?token=root", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.ACLPolicyCreate(resp, req)
	require.NoError(t, err)
	require.NotNil(t, obj)
	aclResp := obj.(*structs.ACLPolicy)
	return aclResp.ID
}

// This tests local agent service registration with a sidecar service. Note we
// only test simple defaults for the sidecar here since the actual logic for
// handling sidecar defaults and port assignment is tested thoroughly in
// TestAgent_sidecarServiceFromNodeService. Note it also tests Deregister
// explicitly too since setup is identical.
func TestAgent_RegisterServiceDeregisterService_Sidecar(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterServiceDeregisterService_Sidecar(t, "")
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
		tokenRules                  string
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
			tokenRules: `
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
			enableACL:  true,
			tokenRules: ``, // No token rules means no valid token
			wantNS:     nil,
			wantErr:    "Permission denied",
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
			tokenRules: `
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
			tokenRules: `
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
			tokenRules: `
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
			tokenRules: `
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
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
			assert := assert.New(t)
			require := require.New(t)

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
				require.NoError(a.AddService(tt.preRegister, nil, false, "", ConfigSourceLocal))
			}
			if tt.preRegister2 != nil {
				require.NoError(a.AddService(tt.preRegister2, nil, false, "", ConfigSourceLocal))
			}

			// Create an ACL token with require policy
			var token string
			if tt.enableACL && tt.tokenRules != "" {
				token = testCreateToken(t, a, tt.tokenRules)
			}

			br := bytes.NewBufferString(tt.json)

			req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token="+token, br)
			resp := httptest.NewRecorder()
			obj, err := a.srv.AgentRegisterService(resp, req)
			if tt.wantErr != "" {
				require.Error(err, "response code=%d, body:\n%s",
					resp.Code, resp.Body.String())
				require.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErr))
				return
			}
			require.NoError(err)
			assert.Nil(obj)
			require.Equal(200, resp.Code, "request failed with body: %s",
				resp.Body.String())

			// Sanity the target service registration
			svcs := a.State.Services(nil)

			// Parse the expected definition into a ServiceDefinition
			var sd structs.ServiceDefinition
			err = json.Unmarshal([]byte(tt.json), &sd)
			require.NoError(err)
			require.NotEmpty(sd.Name)

			svcID := sd.ID
			if svcID == "" {
				svcID = sd.Name
			}
			sid := structs.NewServiceID(svcID, nil)
			svc, ok := svcs[sid]
			require.True(ok, "has service "+sid.String())
			assert.Equal(sd.Name, svc.Service)
			assert.Equal(sd.Port, svc.Port)
			// Ensure that the actual registered service _doesn't_ still have it's
			// sidecar info since it's duplicate and we don't want that synced up to
			// the catalog or included in responses particularly - it's just
			// registration syntax sugar.
			assert.Nil(svc.Connect.SidecarService)

			if tt.wantNS == nil {
				// Sanity check that there was no service registered, we rely on there
				// being no services at start of test so we can just use the count.
				assert.Len(svcs, 1, "should be no sidecar registered")
				return
			}

			// Ensure sidecar
			svc, ok = svcs[structs.NewServiceID(tt.wantNS.ID, nil)]
			require.True(ok, "no sidecar registered at "+tt.wantNS.ID)
			assert.Equal(tt.wantNS, svc)

			if tt.assertStateFn != nil {
				tt.assertStateFn(t, a.State)
			}

			// Now verify deregistration also removes sidecar (if there was one and it
			// was added via sidecar not just coincidental ID clash)
			{
				req := httptest.NewRequest("PUT",
					"/v1/agent/service/deregister/"+svcID+"?token="+token, nil)
				resp := httptest.NewRecorder()
				obj, err := a.srv.AgentDeregisterService(resp, req)
				require.NoError(err)
				require.Nil(obj)

				svcs := a.State.Services(nil)
				_, ok = svcs[structs.NewServiceID(tt.wantNS.ID, nil)]
				if tt.wantSidecarIDLeftAfterDereg {
					require.True(ok, "removed non-sidecar service at "+tt.wantNS.ID)
				} else {
					require.False(ok, "sidecar not deregistered with service "+svcID)
				}
			}
		})
	}
}

// This tests that connect proxy validation is done for local agent
// registration. This doesn't need to test validation exhaustively since
// that is done via a table test in the structs package.
func TestAgent_RegisterService_UnmanagedConnectProxyInvalid(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_UnmanagedConnectProxyInvalid(t, "")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_UnmanagedConnectProxyInvalid(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_UnmanagedConnectProxyInvalid(t *testing.T, extraHCL string) {
	t.Helper()

	assert := assert.New(t)
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

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentRegisterService(resp, req)
	assert.Nil(err)
	assert.Nil(obj)
	assert.Equal(http.StatusBadRequest, resp.Code)
	assert.Contains(resp.Body.String(), "Port")

	// Ensure the service doesn't exist
	assert.Nil(a.State.Service(structs.NewServiceID("connect-proxy", nil)))
}

// Tests agent registration of a service that is connect native.
func TestAgent_RegisterService_ConnectNative(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ConnectNative(t, "")
	})
	t.Run("service manager", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ConnectNative(t, "enable_central_service_config = true")
	})
}

func testAgent_RegisterService_ConnectNative(t *testing.T, extraHCL string) {
	t.Helper()

	assert := assert.New(t)
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
	obj, err := a.srv.AgentRegisterService(resp, req)
	assert.Nil(err)
	assert.Nil(obj)

	// Ensure the service
	svc := a.State.Service(structs.NewServiceID("web", nil))
	require.NotNil(t, svc)
	assert.True(svc.Connect.Native)
}

func TestAgent_RegisterService_ScriptCheck_ExecDisable(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ScriptCheck_ExecDisable(t, "")
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
		Tags: []string{"master"},
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
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))

	_, err := a.srv.AgentRegisterService(nil, req)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("expected script disabled error, got: %s", err)
	}
	checkID := types.CheckID("test-check")
	require.Nil(t, a.State.Check(structs.NewCheckID(checkID, nil)), "check registered with exec disabled")
}

func TestAgent_RegisterService_ScriptCheck_ExecRemoteDisable(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		testAgent_RegisterService_ScriptCheck_ExecRemoteDisable(t, "")
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
		Tags: []string{"master"},
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
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))

	_, err := a.srv.AgentRegisterService(nil, req)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("expected script disabled error, got: %s", err)
	}
	checkID := types.CheckID("test-check")
	require.Nil(t, a.State.Check(structs.NewCheckID(checkID, nil)), "check registered with exec disabled")
}

func TestAgent_DeregisterService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test", nil)
	obj, err := a.srv.AgentDeregisterService(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	assert.Nil(t, a.State.Service(structs.NewServiceID("test", nil)), "have test service")
	assert.Nil(t, a.State.Check(structs.NewCheckID("test", nil)), "have test check")
}

func TestAgent_DeregisterService_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test", nil)
		if _, err := a.srv.AgentDeregisterService(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test?token=root", nil)
		if _, err := a.srv.AgentDeregisterService(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_ServiceMaintenance_BadRequest(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("not enabled", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test", nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
			t.Fatalf("err: %s", err)
		}
		if resp.Code != 400 {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
	})

	t.Run("no service id", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/?enable=true", nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
			t.Fatalf("err: %s", err)
		}
		if resp.Code != 400 {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
	})

	t.Run("bad service id", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/_nope_?enable=true", nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
			t.Fatalf("err: %s", err)
		}
		if resp.Code != 404 {
			t.Fatalf("expected 404, got %d", resp.Code)
		}
	})
}

func TestAgent_ServiceMaintenance_Enable(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register the service
	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force the service into maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=true&reason=broken&token=mytoken", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
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
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register the service
	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force the service into maintenance mode
	if err := a.EnableServiceMaintenance(structs.NewServiceID("test", nil), "", ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Leave maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=false", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
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
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the service.
	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=true&reason=broken", nil)
		if _, err := a.srv.AgentServiceMaintenance(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=true&reason=broken&token=root", nil)
		if _, err := a.srv.AgentServiceMaintenance(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_NodeMaintenance_BadRequest(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Fails when no enable flag provided
	req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentNodeMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestAgent_NodeMaintenance_Enable(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Force the node into maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance?enable=true&reason=broken&token=mytoken", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentNodeMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
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
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Force the node into maintenance mode
	a.EnableNodeMaintenance("", "")

	// Leave maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance?enable=false", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentNodeMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure the maintenance check was removed
	if existing := a.State.Check(structs.NodeMaintCheckID); existing != nil {
		t.Fatalf("should have removed maintenance check")
	}
}

func TestAgent_NodeMaintenance_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance?enable=true&reason=broken", nil)
		if _, err := a.srv.AgentNodeMaintenance(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance?enable=true&reason=broken&token=root", nil)
		if _, err := a.srv.AgentNodeMaintenance(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_RegisterCheck_Service(t *testing.T) {
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
	if _, err := a.srv.AgentRegisterService(nil, req); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now register an additional check
	checkArgs := &structs.CheckDefinition{
		Name:      "memcache_check2",
		ServiceID: "memcache",
		TTL:       15 * time.Second,
	}
	req, _ = http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(checkArgs))
	if _, err := a.srv.AgentRegisterCheck(nil, req); err != nil {
		t.Fatalf("err: %v", err)
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
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("unknown log level", func(t *testing.T) {
		// Try passing an invalid log level
		req, _ := http.NewRequest("GET", "/v1/agent/monitor?loglevel=invalid", nil)
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentMonitor(resp, req)
		if err == nil {
			t.Fatal("expected BadRequestError to have occurred, got nil")
		}

		// Note that BadRequestError is handled outside the endpoint handler so we
		// still see a 200 if we check here.
		if _, ok := err.(BadRequestError); !ok {
			t.Fatalf("expected BadRequestError to have occurred, got %#v", err)
		}

		substring := "Unknown log level"
		if !strings.Contains(err.Error(), substring) {
			t.Fatalf("got: %s, wanted message containing: %s", err.Error(), substring)
		}
	})

	t.Run("stream unstructured logs", func(t *testing.T) {
		// Try to stream logs until we see the expected log line
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/agent/monitor?loglevel=debug", nil)
			cancelCtx, cancelFunc := context.WithCancel(context.Background())
			req = req.WithContext(cancelCtx)

			resp := httptest.NewRecorder()
			errCh := make(chan error)
			go func() {
				_, err := a.srv.AgentMonitor(resp, req)
				errCh <- err
			}()

			args := &structs.ServiceDefinition{
				Name: "monitor",
				Port: 8000,
				Check: structs.CheckType{
					TTL: 15 * time.Second,
				},
			}

			registerReq, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			if _, err := a.srv.AgentRegisterService(nil, registerReq); err != nil {
				t.Fatalf("err: %v", err)
			}

			// Wait until we have received some type of logging output
			require.Eventually(t, func() bool {
				return len(resp.Body.Bytes()) > 0
			}, 3*time.Second, 100*time.Millisecond)

			cancelFunc()
			err := <-errCh
			require.NoError(t, err)

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
			go a.srv.Handler.ServeHTTP(resp, req)

			args := &structs.ServiceDefinition{
				Name: "monitor",
				Port: 8000,
				Check: structs.CheckType{
					TTL: 15 * time.Second,
				},
			}

			registerReq, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			if _, err := a.srv.AgentRegisterService(nil, registerReq); err != nil {
				t.Fatalf("err: %v", err)
			}

			// Wait until we have received some type of logging output
			require.Eventually(t, func() bool {
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
			errCh := make(chan error)
			go func() {
				_, err := a.srv.AgentMonitor(resp, req)
				errCh <- err
			}()

			args := &structs.ServiceDefinition{
				Name: "monitor",
				Port: 8000,
				Check: structs.CheckType{
					TTL: 15 * time.Second,
				},
			}

			registerReq, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			if _, err := a.srv.AgentRegisterService(nil, registerReq); err != nil {
				t.Fatalf("err: %v", err)
			}

			// Wait until we have received some type of logging output
			require.Eventually(t, func() bool {
				return len(resp.Body.Bytes()) > 0
			}, 3*time.Second, 100*time.Millisecond)

			cancelFunc()
			err := <-errCh
			require.NoError(t, err)

			// Each line is output as a separate JSON object, we grab the first and
			// make sure it can be unmarshalled.
			firstLine := bytes.Split(resp.Body.Bytes(), []byte("\n"))[0]
			var output map[string]interface{}
			if err := json.Unmarshal(firstLine, &output); err != nil {
				t.Fatalf("err: %v", err)
			}
		})
	})

	// hopefully catch any potential regression in serf/memberlist logging setup.
	t.Run("serf shutdown logging", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/monitor?loglevel=debug", nil)
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		req = req.WithContext(cancelCtx)

		resp := httptest.NewRecorder()
		errCh := make(chan error)
		go func() {
			_, err := a.srv.AgentMonitor(resp, req)
			errCh <- err
		}()

		require.NoError(t, a.Shutdown())

		// Wait until we have received some type of logging output
		require.Eventually(t, func() bool {
			return len(resp.Body.Bytes()) > 0
		}, 3*time.Second, 100*time.Millisecond)

		cancelFunc()
		err := <-errCh
		require.NoError(t, err)

		got := resp.Body.String()
		want := "serf: Shutdown without a Leave"
		if !strings.Contains(got, want) {
			t.Fatalf("got %q and did not find %q", got, want)
		}
	})
}

func TestAgent_Monitor_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Try without a token.
	req, _ := http.NewRequest("GET", "/v1/agent/monitor", nil)
	if _, err := a.srv.AgentMonitor(nil, req); !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// This proves we call the ACL function, and we've got the other monitor
	// test to prove monitor works, which should be sufficient. The monitor
	// logic is a little complex to set up so isn't worth repeating again
	// here.
}

func TestAgent_TokenTriggersFullSync(t *testing.T) {
	t.Parallel()

	body := func(token string) io.Reader {
		return jsonReader(&api.AgentToken{Token: token})
	}

	createNodePolicy := func(t *testing.T, a *TestAgent, policyName string) *structs.ACLPolicy {
		policy := &structs.ACLPolicy{
			Name:  policyName,
			Rules: `node_prefix "" { policy = "write" }`,
		}

		req, err := http.NewRequest("PUT", "/v1/acl/policy?token=root", jsonBody(policy))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLPolicyCreate(resp, req)
		require.NoError(t, err)

		policy, ok := obj.(*structs.ACLPolicy)
		require.True(t, ok)
		return policy
	}

	createNodeToken := func(t *testing.T, a *TestAgent, policyName string) *structs.ACLToken {
		createNodePolicy(t, a, policyName)

		token := &structs.ACLToken{
			Description: "test",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{Name: policyName},
			},
		}

		req, err := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(token))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLTokenCreate(resp, req)
		require.NoError(t, err)

		token, ok := obj.(*structs.ACLToken)
		require.True(t, ok)
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
			url := fmt.Sprintf("/v1/agent/token/%s?token=root", tt.path)

			a := NewTestAgent(t, TestACLConfig()+`
				acl {
					tokens {
						default = ""
						agent = ""
						agent_master = ""
						replication = ""
					}
				}
			`)
			defer a.Shutdown()
			testrpc.WaitForLeader(t, a.RPC, "dc1")

			// create node policy and token
			token := createNodeToken(t, a, "test")

			req, err := http.NewRequest("PUT", url, body(token.SecretID))
			require.NoError(t, err)

			resp := httptest.NewRecorder()
			_, err = a.srv.AgentToken(resp, req)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, resp.Code)
			require.Equal(t, token.SecretID, tt.tokenGetFn(a.tokens))

			testrpc.WaitForTestAgent(t, a.RPC, "dc1",
				testrpc.WithToken("root"),
				testrpc.WaitForAntiEntropySync())
		})
	}
}

func TestAgent_Token(t *testing.T) {
	t.Parallel()

	// The behavior of this handler when ACLs are disabled is vetted over
	// in TestACL_Disabled_Response since there's already good infra set
	// up over there to test this, and it calls the common function.
	a := NewTestAgent(t, TestACLConfig()+`
		acl {
			tokens {
				default = ""
				agent = ""
				agent_master = ""
				replication = ""
			}
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	type tokens struct {
		user         string
		userSource   tokenStore.TokenSource
		agent        string
		agentSource  tokenStore.TokenSource
		master       string
		masterSource tokenStore.TokenSource
		repl         string
		replSource   tokenStore.TokenSource
	}

	resetTokens := func(init tokens) {
		a.tokens.UpdateUserToken(init.user, init.userSource)
		a.tokens.UpdateAgentToken(init.agent, init.agentSource)
		a.tokens.UpdateAgentMasterToken(init.master, init.masterSource)
		a.tokens.UpdateReplicationToken(init.repl, init.replSource)
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
	}{
		{
			name:   "bad token name",
			method: "PUT",
			url:    "nope?token=root",
			body:   body("X"),
			code:   http.StatusNotFound,
		},
		{
			name:   "bad JSON",
			method: "PUT",
			url:    "acl_token?token=root",
			body:   badJSON(),
			code:   http.StatusBadRequest,
		},
		{
			name:      "set user legacy",
			method:    "PUT",
			url:       "acl_token?token=root",
			body:      body("U"),
			code:      http.StatusOK,
			raw:       tokens{user: "U", userSource: tokenStore.TokenSourceAPI},
			effective: tokens{user: "U", agent: "U"},
		},
		{
			name:      "set default",
			method:    "PUT",
			url:       "default?token=root",
			body:      body("U"),
			code:      http.StatusOK,
			raw:       tokens{user: "U", userSource: tokenStore.TokenSourceAPI},
			effective: tokens{user: "U", agent: "U"},
		},
		{
			name:      "set agent legacy",
			method:    "PUT",
			url:       "acl_agent_token?token=root",
			body:      body("A"),
			code:      http.StatusOK,
			init:      tokens{user: "U", agent: "U"},
			raw:       tokens{user: "U", agent: "A", agentSource: tokenStore.TokenSourceAPI},
			effective: tokens{user: "U", agent: "A"},
		},
		{
			name:      "set agent",
			method:    "PUT",
			url:       "agent?token=root",
			body:      body("A"),
			code:      http.StatusOK,
			init:      tokens{user: "U", agent: "U"},
			raw:       tokens{user: "U", agent: "A", agentSource: tokenStore.TokenSourceAPI},
			effective: tokens{user: "U", agent: "A"},
		},
		{
			name:      "set master legacy",
			method:    "PUT",
			url:       "acl_agent_master_token?token=root",
			body:      body("M"),
			code:      http.StatusOK,
			raw:       tokens{master: "M", masterSource: tokenStore.TokenSourceAPI},
			effective: tokens{master: "M"},
		},
		{
			name:      "set master ",
			method:    "PUT",
			url:       "agent_master?token=root",
			body:      body("M"),
			code:      http.StatusOK,
			raw:       tokens{master: "M", masterSource: tokenStore.TokenSourceAPI},
			effective: tokens{master: "M"},
		},
		{
			name:      "set repl legacy",
			method:    "PUT",
			url:       "acl_replication_token?token=root",
			body:      body("R"),
			code:      http.StatusOK,
			raw:       tokens{repl: "R", replSource: tokenStore.TokenSourceAPI},
			effective: tokens{repl: "R"},
		},
		{
			name:      "set repl",
			method:    "PUT",
			url:       "replication?token=root",
			body:      body("R"),
			code:      http.StatusOK,
			raw:       tokens{repl: "R", replSource: tokenStore.TokenSourceAPI},
			effective: tokens{repl: "R"},
		},
		{
			name:   "clear user legacy",
			method: "PUT",
			url:    "acl_token?token=root",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{user: "U"},
			raw:    tokens{userSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear default",
			method: "PUT",
			url:    "default?token=root",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{user: "U"},
			raw:    tokens{userSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear agent legacy",
			method: "PUT",
			url:    "acl_agent_token?token=root",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{agent: "A"},
			raw:    tokens{agentSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear agent",
			method: "PUT",
			url:    "agent?token=root",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{agent: "A"},
			raw:    tokens{agentSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear master legacy",
			method: "PUT",
			url:    "acl_agent_master_token?token=root",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{master: "M"},
			raw:    tokens{masterSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear master",
			method: "PUT",
			url:    "agent_master?token=root",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{master: "M"},
			raw:    tokens{masterSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear repl legacy",
			method: "PUT",
			url:    "acl_replication_token?token=root",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{repl: "R"},
			raw:    tokens{replSource: tokenStore.TokenSourceAPI},
		},
		{
			name:   "clear repl",
			method: "PUT",
			url:    "replication?token=root",
			body:   body(""),
			code:   http.StatusOK,
			init:   tokens{repl: "R"},
			raw:    tokens{replSource: tokenStore.TokenSourceAPI},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTokens(tt.init)
			url := fmt.Sprintf("/v1/agent/token/%s", tt.url)
			resp := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, url, tt.body)
			_, err := a.srv.AgentToken(resp, req)
			require.NoError(t, err)
			require.Equal(t, tt.code, resp.Code)
			require.Equal(t, tt.effective.user, a.tokens.UserToken())
			require.Equal(t, tt.effective.agent, a.tokens.AgentToken())
			require.Equal(t, tt.effective.master, a.tokens.AgentMasterToken())
			require.Equal(t, tt.effective.repl, a.tokens.ReplicationToken())

			tok, src := a.tokens.UserTokenAndSource()
			require.Equal(t, tt.raw.user, tok)
			require.Equal(t, tt.raw.userSource, src)

			tok, src = a.tokens.AgentTokenAndSource()
			require.Equal(t, tt.raw.agent, tok)
			require.Equal(t, tt.raw.agentSource, src)

			tok, src = a.tokens.AgentMasterTokenAndSource()
			require.Equal(t, tt.raw.master, tok)
			require.Equal(t, tt.raw.masterSource, src)

			tok, src = a.tokens.ReplicationTokenAndSource()
			require.Equal(t, tt.raw.repl, tok)
			require.Equal(t, tt.raw.replSource, src)
		})
	}

	// This one returns an error that is interpreted by the HTTP wrapper, so
	// doesn't fit into our table above.
	t.Run("permission denied", func(t *testing.T) {
		resetTokens(tokens{})
		req, _ := http.NewRequest("PUT", "/v1/agent/token/acl_token", body("X"))
		_, err := a.srv.AgentToken(nil, req)
		require.True(t, acl.IsErrPermissionDenied(err))
		require.Equal(t, "", a.tokens.UserToken())
	})
}

func TestAgentConnectCARoots_empty(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t, "connect { enabled = false }")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectCARoots(resp, req)
	require.Error(err)
	require.Contains(err.Error(), "Connect must be enabled")
}

func TestAgentConnectCARoots_list(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Set some CAs. Note that NewTestAgent already bootstraps one CA so this just
	// adds a second and makes it active.
	ca2 := connect.TestCAConfigSet(t, a, nil)

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCARoots(resp, req)
	require.NoError(err)

	value := obj.(structs.IndexedCARoots)
	assert.Equal(value.ActiveRootID, ca2.ID)
	// Would like to assert that it's the same as the TestAgent domain but the
	// only way to access that state via this package is by RPC to the server
	// implementation running in TestAgent which is more or less a tautology.
	assert.NotEmpty(value.TrustDomain)
	assert.Len(value.Roots, 2)

	// We should never have the secret information
	for _, r := range value.Roots {
		assert.Equal("", r.SigningCert)
		assert.Equal("", r.SigningKey)
	}

	assert.Equal("MISS", resp.Header().Get("X-Cache"))

	// Test caching
	{
		// List it again
		resp2 := httptest.NewRecorder()
		obj2, err := a.srv.AgentConnectCARoots(resp2, req)
		require.NoError(err)
		assert.Equal(obj, obj2)

		// Should cache hit this time and not make request
		assert.Equal("HIT", resp2.Header().Get("X-Cache"))
	}

	// Test that caching is updated in the background
	{
		// Set a new CA
		ca := connect.TestCAConfigSet(t, a, nil)

		retry.Run(t, func(r *retry.R) {
			// List it again
			resp := httptest.NewRecorder()
			obj, err := a.srv.AgentConnectCARoots(resp, req)
			r.Check(err)

			value := obj.(structs.IndexedCARoots)
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
	t.Parallel()

	require := require.New(t)
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

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.Error(err)
	require.True(acl.IsErrPermissionDenied(err))
}

func TestAgentConnectCALeafCert_aclServiceWrite(t *testing.T) {
	t.Parallel()

	require := require.New(t)
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

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Create an ACL with service:write for our service
	var token string
	{
		args := map[string]interface{}{
			"Name":  "User Token",
			"Type":  "client",
			"Rules": `service "test" { policy = "write" }`,
		}
		req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		aclResp := obj.(aclCreateResponse)
		token = aclResp.ID
	}

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?token="+token, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.NoError(err)

	// Get the issued cert
	_, ok := obj.(*structs.IssuedCert)
	require.True(ok)
}

func TestAgentConnectCALeafCert_aclServiceReadDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)
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

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Create an ACL with service:read for our service
	var token string
	{
		args := map[string]interface{}{
			"Name":  "User Token",
			"Type":  "client",
			"Rules": `service "test" { policy = "read" }`,
		}
		req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		aclResp := obj.(aclCreateResponse)
		token = aclResp.ID
	}

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?token="+token, nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.Error(err)
	require.True(acl.IsErrPermissionDenied(err))
}

func TestAgentConnectCALeafCert_good(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t, "")
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
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		if !assert.Equal(200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.NoError(err)
	require.Equal("MISS", resp.Header().Get("X-Cache"))

	// Get the issued cert
	issued, ok := obj.(*structs.IssuedCert)
	assert.True(ok)

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, ca1)

	// Verify blocking index
	assert.True(issued.ModifyIndex > 0)
	assert.Equal(fmt.Sprintf("%d", issued.ModifyIndex),
		resp.Header().Get("X-Consul-Index"))

	// Test caching
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		obj2, err := a.srv.AgentConnectCALeafCert(resp, req)
		require.NoError(err)
		require.Equal(obj, obj2)

		// Should cache hit this time and not make request
		require.Equal("HIT", resp.Header().Get("X-Cache"))
	}

	// Test that caching is updated in the background
	{
		// Set a new CA
		ca := connect.TestCAConfigSet(t, a, nil)

		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			// Try and sign again (note no index/wait arg since cache should update in
			// background even if we aren't actively blocking)
			obj, err := a.srv.AgentConnectCALeafCert(resp, req)
			r.Check(err)

			issued2 := obj.(*structs.IssuedCert)
			if issued.CertPEM == issued2.CertPEM {
				r.Fatalf("leaf has not updated")
			}

			// Got a new leaf. Sanity check it's a whole new key as well as different
			// cert.
			if issued.PrivateKeyPEM == issued2.PrivateKeyPEM {
				r.Fatalf("new leaf has same private key as before")
			}

			// Verify that the cert is signed by the new CA
			requireLeafValidUnderCA(t, issued2, ca)

			// Should be a cache hit! The data should've updated in the cache
			// in the background so this should've been fetched directly from
			// the cache.
			if resp.Header().Get("X-Cache") != "HIT" {
				r.Fatalf("should be a cache hit")
			}
		})
	}
}

// Test we can request a leaf cert for a service we have permission for
// but is not local to this agent.
func TestAgentConnectCALeafCert_goodNotLocal(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t, "")
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
		_, err := a.srv.CatalogRegister(resp, req)
		require.NoError(err)
		if !assert.Equal(200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.NoError(err)
	require.Equal("MISS", resp.Header().Get("X-Cache"))

	// Get the issued cert
	issued, ok := obj.(*structs.IssuedCert)
	assert.True(ok)

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, ca1)

	// Verify blocking index
	assert.True(issued.ModifyIndex > 0)
	assert.Equal(fmt.Sprintf("%d", issued.ModifyIndex),
		resp.Header().Get("X-Consul-Index"))

	// Test caching
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		obj2, err := a.srv.AgentConnectCALeafCert(resp, req)
		require.NoError(err)
		require.Equal(obj, obj2)

		// Should cache hit this time and not make request
		require.Equal("HIT", resp.Header().Get("X-Cache"))
	}

	// Test Blocking - see https://github.com/hashicorp/consul/issues/4462
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		blockingReq, _ := http.NewRequest("GET", fmt.Sprintf("/v1/agent/connect/ca/leaf/test?wait=125ms&index=%d", issued.ModifyIndex), nil)
		doneCh := make(chan struct{})
		go func() {
			a.srv.AgentConnectCALeafCert(resp, blockingReq)
			close(doneCh)
		}()

		select {
		case <-time.After(500 * time.Millisecond):
			require.FailNow("Shouldn't block for this long - not respecting wait parameter in the query")

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
			obj, err := a.srv.AgentConnectCALeafCert(resp, req)
			r.Check(err)

			issued2 := obj.(*structs.IssuedCert)
			if issued.CertPEM == issued2.CertPEM {
				r.Fatalf("leaf has not updated")
			}

			// Got a new leaf. Sanity check it's a whole new key as well as different
			// cert.
			if issued.PrivateKeyPEM == issued2.PrivateKeyPEM {
				r.Fatalf("new leaf has same private key as before")
			}

			// Verify that the cert is signed by the new CA
			requireLeafValidUnderCA(t, issued2, ca)

			// Should be a cache hit! The data should've updated in the cache
			// in the background so this should've been fetched directly from
			// the cache.
			if resp.Header().Get("X-Cache") != "HIT" {
				r.Fatalf("should be a cache hit")
			}
		})
	}
}

func TestAgentConnectCALeafCert_secondaryDC_good(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)

	a1 := StartTestAgent(t, TestAgent{Name: "dc1", HCL: `
		datacenter = "dc1"
		primary_datacenter = "dc1"
	`})
	defer a1.Shutdown()
	testrpc.WaitForTestAgent(t, a1.RPC, "dc1")

	a2 := StartTestAgent(t, TestAgent{Name: "dc2", HCL: `
		datacenter = "dc2"
		primary_datacenter = "dc1"
	`})
	defer a2.Shutdown()
	testrpc.WaitForTestAgent(t, a2.RPC, "dc2")

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	_, err := a2.JoinWAN([]string{addr})
	require.NoError(err)

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
		_, err := a2.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		if !assert.Equal(200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	obj, err := a2.srv.AgentConnectCALeafCert(resp, req)
	require.NoError(err)
	require.Equal("MISS", resp.Header().Get("X-Cache"))

	// Get the issued cert
	issued, ok := obj.(*structs.IssuedCert)
	assert.True(ok)

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, dc1_ca1)

	// Verify blocking index
	assert.True(issued.ModifyIndex > 0)
	assert.Equal(fmt.Sprintf("%d", issued.ModifyIndex),
		resp.Header().Get("X-Consul-Index"))

	// Test caching
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		obj2, err := a2.srv.AgentConnectCALeafCert(resp, req)
		require.NoError(err)
		require.Equal(obj, obj2)

		// Should cache hit this time and not make request
		require.Equal("HIT", resp.Header().Get("X-Cache"))
	}

	// Test that we aren't churning leaves for no reason at idle.
	{
		ch := make(chan error, 1)
		go func() {
			req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?index="+strconv.Itoa(int(issued.ModifyIndex)), nil)
			resp := httptest.NewRecorder()
			obj, err := a2.srv.AgentConnectCALeafCert(resp, req)
			if err != nil {
				ch <- err
			} else {
				issued2 := obj.(*structs.IssuedCert)
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
		obj, err := a2.srv.AgentConnectCALeafCert(resp, req)
		r.Check(err)

		issued2 := obj.(*structs.IssuedCert)
		if issued.CertPEM == issued2.CertPEM {
			r.Fatalf("leaf has not updated")
		}

		// Got a new leaf. Sanity check it's a whole new key as well as different
		// cert.
		if issued.PrivateKeyPEM == issued2.PrivateKeyPEM {
			r.Fatalf("new leaf has same private key as before")
		}

		// Verify that the cert is signed by the new CA
		requireLeafValidUnderCA(t, issued2, dc1_ca2)

		// Should be a cache hit! The data should've updated in the cache
		// in the background so this should've been fetched directly from
		// the cache.
		if resp.Header().Get("X-Cache") != "HIT" {
			r.Fatalf("should be a cache hit")
		}
	})
}

func waitForActiveCARoot(t *testing.T, srv *HTTPServer, expect *structs.CARoot) {
	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/roots", nil)
		resp := httptest.NewRecorder()
		obj, err := srv.AgentConnectCARoots(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		roots, ok := obj.(structs.IndexedCARoots)
		if !ok {
			r.Fatalf("response is wrong type %T", obj)
		}

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

func requireLeafValidUnderCA(t *testing.T, issued *structs.IssuedCert, ca *structs.CARoot) {
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

func makeTelemetryDefaults(targetID string) lib.TelemetryConfig {
	return lib.TelemetryConfig{
		FilterDefault: true,
		MetricsPrefix: "consul.proxy." + targetID,
	}
}

func TestAgentConnectAuthorize_badBody(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	args := []string{}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	require.Error(err)
	assert.Nil(respRaw)
	// Note that BadRequestError is handled outside the endpoint handler so we
	// still see a 200 if we check here.
	assert.Contains(err.Error(), "decode failed")
}

func TestAgentConnectAuthorize_noTarget(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	args := &structs.ConnectAuthorizeRequest{}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	require.Error(err)
	assert.Nil(respRaw)
	// Note that BadRequestError is handled outside the endpoint handler so we
	// still see a 200 if we check here.
	assert.Contains(err.Error(), "Target service must be specified")
}

// Client ID is not in the valid URI format
func TestAgentConnectAuthorize_idInvalidFormat(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	args := &structs.ConnectAuthorizeRequest{
		Target:        "web",
		ClientCertURI: "tubes",
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	require.Error(err)
	assert.Nil(respRaw)
	// Note that BadRequestError is handled outside the endpoint handler so we
	// still see a 200 if we check here.
	assert.Contains(err.Error(), "ClientCertURI not a valid Connect identifier")
}

// Client ID is a valid URI but its not a service URI
func TestAgentConnectAuthorize_idNotService(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	args := &structs.ConnectAuthorizeRequest{
		Target:        "web",
		ClientCertURI: "spiffe://1234.consul",
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	require.Error(err)
	assert.Nil(respRaw)
	// Note that BadRequestError is handled outside the endpoint handler so we
	// still see a 200 if we check here.
	assert.Contains(err.Error(), "ClientCertURI not a valid Service identifier")
}

// Test when there is an intention allowing the connection
func TestAgentConnectAuthorize_allow(t *testing.T) {
	t.Parallel()

	require := require.New(t)
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

		require.Nil(a.RPC("Intention.Apply", &req, &ixnId))
	}

	args := &structs.ConnectAuthorizeRequest{
		Target:        target,
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	require.Nil(err)
	require.Equal(200, resp.Code)
	require.Equal("MISS", resp.Header().Get("X-Cache"))

	obj := respRaw.(*connectAuthorizeResp)
	require.True(obj.Authorized)
	require.Contains(obj.Reason, "Matched")

	// Make the request again
	{
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		require.Nil(err)
		require.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		require.True(obj.Authorized)
		require.Contains(obj.Reason, "Matched")

		// That should've been a cache hit.
		require.Equal("HIT", resp.Header().Get("X-Cache"))
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

		require.Nil(a.RPC("Intention.Apply", &req, &ixnId))
	}

	// Short sleep lets the cache background refresh happen
	time.Sleep(100 * time.Millisecond)

	// Make the request again
	{
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		require.Nil(err)
		require.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		require.False(obj.Authorized)
		require.Contains(obj.Reason, "Matched")

		// That should've been a cache hit, too, since it updated in the
		// background.
		require.Equal("HIT", resp.Header().Get("X-Cache"))
	}
}

// Test when there is an intention denying the connection
func TestAgentConnectAuthorize_deny(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
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
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}

	args := &structs.ConnectAuthorizeRequest{
		Target:        target,
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(200, resp.Code)

	obj := respRaw.(*connectAuthorizeResp)
	assert.False(obj.Authorized)
	assert.Contains(obj.Reason, "Matched")
}

// Test when there is an intention allowing service with a different trust
// domain. We allow this because migration between trust domains shouldn't cause
// an outage even if we have stale info about current trusted domains. It's safe
// because the CA root is either unique to this cluster and not used to sign
// anything external, or path validation can be used to ensure that the CA can
// only issue certs that are valid for the specific cluster trust domain at x509
// level which is enforced by TLS handshake.
func TestAgentConnectAuthorize_allowTrustDomain(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
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
		require.NoError(a.RPC("Intention.Apply", &req, &reply))
	}

	{
		args := &structs.ConnectAuthorizeRequest{
			Target:        target,
			ClientCertURI: "spiffe://fake-domain.consul/ns/default/dc/dc1/svc/web",
		}
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		require.NoError(err)
		assert.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		require.True(obj.Authorized)
		require.Contains(obj.Reason, "Matched")
	}
}

func TestAgentConnectAuthorize_denyWildcard(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
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
		require.NoError(a.RPC("Intention.Apply", &req, &reply))
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
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}

	// Web should be allowed
	{
		args := &structs.ConnectAuthorizeRequest{
			Target:        target,
			ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
		}
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		require.NoError(err)
		assert.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		assert.True(obj.Authorized)
		assert.Contains(obj.Reason, "Matched")
	}

	// API should be denied
	{
		args := &structs.ConnectAuthorizeRequest{
			Target:        target,
			ClientCertURI: connect.TestSpiffeIDService(t, "api").URI().String(),
		}
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		require.NoError(err)
		assert.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		assert.False(obj.Authorized)
		assert.Contains(obj.Reason, "Matched")
	}
}

// Test that authorize fails without service:write for the target service.
func TestAgentConnectAuthorize_serviceWrite(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Create an ACL
	var token string
	{
		args := map[string]interface{}{
			"Name":  "User Token",
			"Type":  "client",
			"Rules": `service "foo" { policy = "read" }`,
		}
		req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		aclResp := obj.(aclCreateResponse)
		token = aclResp.ID
	}

	args := &structs.ConnectAuthorizeRequest{
		Target:        "foo",
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST",
		"/v1/agent/connect/authorize?token="+token, jsonReader(args))
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.True(acl.IsErrPermissionDenied(err))
}

// Test when no intentions match w/ a default deny policy
func TestAgentConnectAuthorize_defaultDeny(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.ConnectAuthorizeRequest{
		Target:        "foo",
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize?token=root", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(200, resp.Code)

	obj := respRaw.(*connectAuthorizeResp)
	assert.False(obj.Authorized)
	assert.Contains(obj.Reason, "Default behavior")
}

// Test when no intentions match w/ a default allow policy
func TestAgentConnectAuthorize_defaultAllow(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dc1 := "dc1"
	a := NewTestAgent(t, `
		acl_datacenter = "`+dc1+`"
		acl_default_policy = "allow"
		acl_master_token = "root"
		acl_agent_token = "root"
		acl_agent_master_token = "towel"
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, dc1)

	args := &structs.ConnectAuthorizeRequest{
		Target:        "foo",
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize?token=root", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(200, resp.Code)
	assert.NotNil(respRaw)

	obj := respRaw.(*connectAuthorizeResp)
	assert.True(obj.Authorized)
	assert.Contains(obj.Reason, "Default behavior")
}

func TestAgent_Host(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dc1 := "dc1"
	a := NewTestAgent(t, `
	acl_datacenter = "`+dc1+`"
	acl_default_policy = "allow"
	acl_master_token = "master"
	acl_agent_token = "agent"
	acl_agent_master_token = "towel"
	acl_enforce_version_8 = true
`)
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/host?token=master", nil)
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentHost(resp, req)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.Code)
	assert.NotNil(respRaw)

	obj := respRaw.(*debug.HostInfo)
	assert.NotNil(obj.CollectionTime)
	assert.Empty(obj.Errors)
}

func TestAgent_HostBadACL(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dc1 := "dc1"
	a := NewTestAgent(t, `
	acl_datacenter = "`+dc1+`"
	acl_default_policy = "deny"
	acl_master_token = "root"
	acl_agent_token = "agent"
	acl_agent_master_token = "towel"
	acl_enforce_version_8 = true
`)
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	req, _ := http.NewRequest("GET", "/v1/agent/host?token=agent", nil)
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentHost(resp, req)
	assert.EqualError(err, "ACL not found")
	assert.Equal(http.StatusOK, resp.Code)
	assert.Nil(respRaw)
}

// Thie tests that a proxy with an ExposeConfig is returned as expected.
func TestAgent_Services_ExposeConfig(t *testing.T) {
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
	a.State.AddService(srv1, "")

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	obj, err := a.srv.AgentServices(nil, req)
	require.NoError(t, err)
	val := obj.(map[string]*api.AgentService)
	require.Len(t, val, 1)
	actual := val["proxy-id"]
	require.NotNil(t, actual)
	require.Equal(t, api.ServiceKindConnectProxy, actual.Kind)
	require.Equal(t, srv1.Proxy.ToAPI(), actual.Proxy)
}
