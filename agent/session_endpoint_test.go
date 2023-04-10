// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
	"github.com/stretchr/testify/require"
)

func verifySession(t *testing.T, r *retry.R, a *TestAgent, want structs.Session) {
	t.Helper()

	args := &structs.SessionSpecificRequest{
		Datacenter: "dc1",
		SessionID:  want.ID,
	}
	var out structs.IndexedSessions
	if err := a.RPC(context.Background(), "Session.Get", args, &out); err != nil {
		r.Fatalf("err: %v", err)
	}
	if len(out.Sessions) != 1 {
		r.Fatalf("bad: %#v", out.Sessions)
	}

	// Make a copy so we don't modify the state store copy for an in-mem
	// RPC and zero out the Raft info for the compare.
	got := *(out.Sessions[0])
	got.CreateIndex = 0
	got.ModifyIndex = 0

	if got.ID != want.ID {
		t.Fatalf("bad session ID: expected %s, got %s", want.ID, got.ID)
	}
	if got.Node != want.Node {
		t.Fatalf("bad session Node: expected %s, got %s", want.Node, got.Node)
	}
	if got.Behavior != want.Behavior {
		t.Fatalf("bad session Behavior: expected %s, got %s", want.Behavior, got.Behavior)
	}
	if got.LockDelay != want.LockDelay {
		t.Fatalf("bad session LockDelay: expected %s, got %s", want.LockDelay, got.LockDelay)
	}
	if !reflect.DeepEqual(got.Checks, want.Checks) {
		t.Fatalf("bad session Checks: expected %+v, got %+v", want.Checks, got.Checks)
	}
	if !reflect.DeepEqual(got.NodeChecks, want.NodeChecks) {
		t.Fatalf("bad session NodeChecks: expected %+v, got %+v", want.NodeChecks, got.NodeChecks)
	}
	if !reflect.DeepEqual(got.ServiceChecks, want.ServiceChecks) {
		t.Fatalf("bad session ServiceChecks: expected %+v, got %+v", want.ServiceChecks, got.ServiceChecks)
	}
}

func TestSessionCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create a health check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			CheckID:   "consul",
			Node:      a.Config.NodeName,
			Name:      "consul",
			ServiceID: "consul",
			Status:    api.HealthPassing,
		},
	}

	retry.Run(t, func(r *retry.R) {
		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			r.Fatalf("err: %v", err)
		}

		// Associate session with node and 2 health checks
		body := bytes.NewBuffer(nil)
		enc := json.NewEncoder(body)
		raw := map[string]interface{}{
			"Name":      "my-cool-session",
			"Node":      a.Config.NodeName,
			"Checks":    []types.CheckID{"consul"},
			"LockDelay": "20s",
		}
		require.NoError(r, enc.Encode(raw))

		req, _ := http.NewRequest("PUT", "/v1/session/create", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionCreate(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		want := structs.Session{
			ID:         obj.(sessionCreateResponse).ID,
			Name:       "my-cool-session",
			Node:       a.Config.NodeName,
			Checks:     []types.CheckID{"consul"},
			NodeChecks: []string{string(structs.SerfCheckID)},
			LockDelay:  20 * time.Second,
			Behavior:   structs.SessionKeysRelease,
		}
		verifySession(t, r, a, want)
	})
}

func TestSessionCreate_NodeChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create a health check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			CheckID:   "consul",
			Node:      a.Config.NodeName,
			Name:      "consul",
			ServiceID: "consul",
			Status:    api.HealthPassing,
		},
	}

	retry.Run(t, func(r *retry.R) {
		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			r.Fatalf("err: %v", err)
		}

		// Associate session with node and 2 health checks
		body := bytes.NewBuffer(nil)
		enc := json.NewEncoder(body)
		raw := map[string]interface{}{
			"Name": "my-cool-session",
			"Node": a.Config.NodeName,
			"ServiceChecks": []structs.ServiceCheck{
				{ID: "consul", Namespace: ""},
			},
			"NodeChecks": []types.CheckID{structs.SerfCheckID},
			"LockDelay":  "20s",
		}
		require.NoError(r, enc.Encode(raw))

		req, _ := http.NewRequest("PUT", "/v1/session/create", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionCreate(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		want := structs.Session{
			ID:            obj.(sessionCreateResponse).ID,
			Name:          "my-cool-session",
			Node:          a.Config.NodeName,
			NodeChecks:    []string{string(structs.SerfCheckID)},
			ServiceChecks: []structs.ServiceCheck{{ID: "consul", Namespace: ""}},
			LockDelay:     20 * time.Second,
			Behavior:      structs.SessionKeysRelease,
		}
		verifySession(t, r, a, want)
	})
}

func TestSessionCreate_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create a health check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			CheckID:   "consul",
			Node:      a.Config.NodeName,
			Name:      "consul",
			ServiceID: "consul",
			Status:    api.HealthPassing,
		},
	}
	retry.Run(t, func(r *retry.R) {
		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			r.Fatalf("err: %v", err)
		}

		// Associate session with node and 2 health checks, and make it delete on session destroy
		body := bytes.NewBuffer(nil)
		enc := json.NewEncoder(body)
		raw := map[string]interface{}{
			"Name":       "my-cool-session",
			"Node":       a.Config.NodeName,
			"Checks":     []types.CheckID{"consul"},
			"NodeChecks": []string{string(structs.SerfCheckID)},
			"LockDelay":  "20s",
			"Behavior":   structs.SessionKeysDelete,
		}
		require.NoError(r, enc.Encode(raw))

		req, _ := http.NewRequest("PUT", "/v1/session/create", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionCreate(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		want := structs.Session{
			ID:         obj.(sessionCreateResponse).ID,
			Name:       "my-cool-session",
			Node:       a.Config.NodeName,
			Checks:     []types.CheckID{"consul"},
			NodeChecks: []string{string(structs.SerfCheckID)},
			LockDelay:  20 * time.Second,
			Behavior:   structs.SessionKeysDelete,
		}
		verifySession(t, r, a, want)
	})
}

func TestSessionCreate_DefaultCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	raw := map[string]interface{}{
		"Name":      "my-cool-session",
		"Node":      a.Config.NodeName,
		"LockDelay": "20s",
	}

	retry.Run(t, func(r *retry.R) {
		body := bytes.NewBuffer(nil)
		enc := json.NewEncoder(body)
		require.NoError(r, enc.Encode(raw))
		req, _ := http.NewRequest("PUT", "/v1/session/create", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionCreate(resp, req)
		require.NoError(r, err)
		require.Equal(r, resp.Code, http.StatusOK)

		want := structs.Session{
			ID:         obj.(sessionCreateResponse).ID,
			Name:       "my-cool-session",
			Node:       a.Config.NodeName,
			NodeChecks: []string{string(structs.SerfCheckID)},
			LockDelay:  20 * time.Second,
			Behavior:   structs.SessionKeysRelease,
		}
		verifySession(t, r, a, want)
	})
}

func TestSessionCreate_NoCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	raw := map[string]interface{}{
		"Name":      "my-cool-session",
		"Node":      a.Config.NodeName,
		"LockDelay": "20s",
	}

	t.Run("no check fields should yield default serfHealth", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			body := bytes.NewBuffer(nil)
			enc := json.NewEncoder(body)
			require.NoError(r, enc.Encode(raw))

			req, _ := http.NewRequest("PUT", "/v1/session/create", body)
			resp := httptest.NewRecorder()
			obj, err := a.srv.SessionCreate(resp, req)
			require.NoError(r, err)
			require.Equal(r, resp.Code, http.StatusOK, resp.Body.String())

			want := structs.Session{
				ID:         obj.(sessionCreateResponse).ID,
				Name:       "my-cool-session",
				Node:       a.Config.NodeName,
				NodeChecks: []string{string(structs.SerfCheckID)},
				LockDelay:  20 * time.Second,
				Behavior:   structs.SessionKeysRelease,
			}
			verifySession(t, r, a, want)
		})
	})

	t.Run("overwrite nodechecks to associate with no checks", func(t *testing.T) {
		raw := map[string]interface{}{
			"Name":       "my-cool-session",
			"Node":       a.Config.NodeName,
			"NodeChecks": []string{},
			"LockDelay":  "20s",
		}

		retry.Run(t, func(r *retry.R) {
			body := bytes.NewBuffer(nil)
			enc := json.NewEncoder(body)
			require.NoError(r, enc.Encode(raw))
			req, _ := http.NewRequest("PUT", "/v1/session/create", body)
			resp := httptest.NewRecorder()
			obj, err := a.srv.SessionCreate(resp, req)
			require.NoError(r, err)
			require.Equal(r, resp.Code, http.StatusOK)

			want := structs.Session{
				ID:         obj.(sessionCreateResponse).ID,
				Name:       "my-cool-session",
				Node:       a.Config.NodeName,
				NodeChecks: []string{},
				LockDelay:  20 * time.Second,
				Behavior:   structs.SessionKeysRelease,
			}
			verifySession(t, r, a, want)
		})
	})

	t.Run("overwrite checks to associate with no checks", func(t *testing.T) {
		raw := map[string]interface{}{
			"Name":      "my-cool-session",
			"Node":      a.Config.NodeName,
			"Checks":    []string{},
			"LockDelay": "20s",
		}

		retry.Run(t, func(r *retry.R) {
			body := bytes.NewBuffer(nil)
			enc := json.NewEncoder(body)
			require.NoError(r, enc.Encode(raw))

			req, _ := http.NewRequest("PUT", "/v1/session/create", body)
			resp := httptest.NewRecorder()
			obj, err := a.srv.SessionCreate(resp, req)
			require.NoError(r, err)
			require.Equal(r, resp.Code, http.StatusOK)

			want := structs.Session{
				ID:         obj.(sessionCreateResponse).ID,
				Name:       "my-cool-session",
				Node:       a.Config.NodeName,
				NodeChecks: []string{},
				Checks:     []types.CheckID{},
				LockDelay:  20 * time.Second,
				Behavior:   structs.SessionKeysRelease,
			}
			verifySession(t, r, a, want)
		})
	})
}

func makeTestSession(t *testing.T, srv *HTTPHandlers) string {
	t.Helper()
	url := "/v1/session/create"
	req, _ := http.NewRequest("PUT", url, nil)
	resp := httptest.NewRecorder()
	obj, err := srv.SessionCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	sessResp := obj.(sessionCreateResponse)
	return sessResp.ID
}

func makeTestSessionDelete(t *testing.T, srv *HTTPHandlers) string {
	t.Helper()
	// Create Session with delete behavior
	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"Behavior": "delete",
	}
	require.NoError(t, enc.Encode(raw))

	url := "/v1/session/create"
	req, _ := http.NewRequest("PUT", url, body)
	resp := httptest.NewRecorder()
	obj, err := srv.SessionCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	sessResp := obj.(sessionCreateResponse)
	return sessResp.ID
}

func makeTestSessionTTL(t *testing.T, srv *HTTPHandlers, ttl string) string {
	t.Helper()
	// Create Session with TTL
	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"TTL": ttl,
	}
	require.NoError(t, enc.Encode(raw))

	url := "/v1/session/create"
	req, _ := http.NewRequest("PUT", url, body)
	resp := httptest.NewRecorder()
	obj, err := srv.SessionCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	sessResp := obj.(sessionCreateResponse)
	return sessResp.ID
}

func TestSessionDestroy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	id := makeTestSession(t, a.srv)

	req, _ := http.NewRequest("PUT", "/v1/session/destroy/"+id, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.SessionDestroy(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp := obj.(bool); !resp {
		t.Fatalf("should work")
	}
}

func TestSessionCustomTTL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	ttl := 250 * time.Millisecond
	a := NewTestAgent(t, `
		session_ttl_min = "250ms"
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	retry.Run(t, func(r *retry.R) {
		id := makeTestSessionTTL(t, a.srv, ttl.String())

		req, _ := http.NewRequest("GET", "/v1/session/info/"+id, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionGet(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			r.Fatalf("should work")
		}
		if len(respObj) != 1 {
			r.Fatalf("bad: %v", respObj)
		}
		if respObj[0].TTL != ttl.String() {
			r.Fatalf("Incorrect TTL: %s", respObj[0].TTL)
		}

		time.Sleep(ttl*structs.SessionTTLMultiplier + ttl)

		req, _ = http.NewRequest("GET", "/v1/session/info/"+id, nil)
		resp = httptest.NewRecorder()
		obj, err = a.srv.SessionGet(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		respObj, ok = obj.(structs.Sessions)
		require.True(r, ok, "unexpected type: %T", obj)
		require.Len(r, respObj, 0)
	})
}

func TestSessionTTLRenew(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// t.Parallel() // timing test. no parallel
	ttl := 250 * time.Millisecond
	a := NewTestAgent(t, `
		session_ttl_min = "250ms"
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	id := makeTestSessionTTL(t, a.srv, ttl.String())

	req, _ := http.NewRequest("GET", "/v1/session/info/"+id, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.SessionGet(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	respObj, ok := obj.(structs.Sessions)
	if !ok {
		t.Fatalf("should work")
	}
	if len(respObj) != 1 {
		t.Fatalf("bad: %v", respObj)
	}
	if respObj[0].TTL != ttl.String() {
		t.Fatalf("Incorrect TTL: %s", respObj[0].TTL)
	}

	// Sleep to consume some time before renew
	sleepFor := ttl * structs.SessionTTLMultiplier / 3
	if sleepFor <= 0 {
		t.Fatalf("timing tests need to sleep")
	}
	time.Sleep(sleepFor)

	req, _ = http.NewRequest("PUT", "/v1/session/renew/"+id, nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.SessionRenew(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj == nil {
		t.Fatalf("session '%s' expired before renewal", id)
	}
	respObj, ok = obj.(structs.Sessions)
	if !ok {
		t.Fatalf("should work")
	}
	if len(respObj) != 1 {
		t.Fatalf("bad: %v", respObj)
	}

	// Sleep for ttl * TTL Multiplier
	time.Sleep(ttl * structs.SessionTTLMultiplier)

	req, _ = http.NewRequest("GET", "/v1/session/info/"+id, nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.SessionGet(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	respObj, ok = obj.(structs.Sessions)
	if !ok {
		t.Fatalf("session '%s' should have renewed", id)
	}
	if len(respObj) != 1 {
		t.Fatalf("session '%s' should have renewed", id)
	}

	// now wait for timeout and expect session to get destroyed
	time.Sleep(ttl * structs.SessionTTLMultiplier)

	req, _ = http.NewRequest("GET", "/v1/session/info/"+id, nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.SessionGet(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	respObj, ok = obj.(structs.Sessions)
	if !ok {
		t.Fatalf("session '%s' should have destroyed", id)
	}
	if len(respObj) != 0 {
		t.Fatalf("session '%s' should have destroyed", id)
	}
}

func TestSessionGet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/session/info/adf4238a-882b-9ddc-4a9d-5b6758e4159e", nil)
			resp := httptest.NewRecorder()
			obj, err := a.srv.SessionGet(resp, req)
			if err != nil {
				r.Fatalf("err: %v", err)
			}
			respObj, ok := obj.(structs.Sessions)
			if !ok {
				r.Fatalf("should work")
			}
			if respObj == nil || len(respObj) != 0 {
				r.Fatalf("bad: %v", respObj)
			}
		})
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		id := makeTestSession(t, a.srv)

		req, _ := http.NewRequest("GET", "/v1/session/info/"+id, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if len(respObj) != 1 {
			t.Fatalf("bad: %v", respObj)
		}
	})
}

func TestSessionList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		req, _ := http.NewRequest("GET", "/v1/session/list", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionList(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if respObj == nil || len(respObj) != 0 {
			t.Fatalf("bad: %v", respObj)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		var ids []string
		for i := 0; i < 10; i++ {
			ids = append(ids, makeTestSession(t, a.srv))
		}

		req, _ := http.NewRequest("GET", "/v1/session/list", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionList(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		respIDs := make([]string, 0, len(respObj))
		for _, obj := range respObj {
			respIDs = append(respIDs, obj.ID)
		}
		require.ElementsMatch(t, respIDs, ids)
	})
}

func TestSessionsForNode(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		req, _ := http.NewRequest("GET", "/v1/session/node/"+a.Config.NodeName, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionsForNode(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if respObj == nil || len(respObj) != 0 {
			t.Fatalf("bad: %v", respObj)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		var ids []string
		for i := 0; i < 10; i++ {
			ids = append(ids, makeTestSession(t, a.srv))
		}

		req, _ := http.NewRequest("GET", "/v1/session/node/"+a.Config.NodeName, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.SessionsForNode(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		respIDs := make([]string, 0, len(ids))
		for _, session := range respObj {
			respIDs = append(respIDs, session.ID)
		}
		require.ElementsMatch(t, ids, respIDs)
	})
}

func TestSessionDeleteDestroy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	id := makeTestSessionDelete(t, a.srv)

	// now create a new key for the session and acquire it
	buf := bytes.NewBuffer([]byte("test"))
	req, _ := http.NewRequest("PUT", "/v1/kv/ephemeral?acquire="+id, buf)
	resp := httptest.NewRecorder()
	obj, err := a.srv.KVSEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if res := obj.(bool); !res {
		t.Fatalf("should work")
	}

	// now destroy the session, this should delete the key created above
	req, _ = http.NewRequest("PUT", "/v1/session/destroy/"+id, nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.SessionDestroy(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp := obj.(bool); !resp {
		t.Fatalf("should work")
	}

	// Verify that the key is gone
	req, _ = http.NewRequest("GET", "/v1/kv/ephemeral", nil)
	resp = httptest.NewRecorder()
	obj, _ = a.srv.KVSEndpoint(resp, req)
	res, found := obj.(structs.DirEntries)
	if found || len(res) != 0 {
		t.Fatalf("bad: %v found, should be nothing", res)
	}
}
