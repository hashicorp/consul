// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestHTTP_Peering_GenerateToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

	t.Run("No Body", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/token", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "The peering arguments must be provided in the body")
	})

	t.Run("Body Invalid", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader([]byte("abc")))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "Body decoding failed:")
	})

	t.Run("No Name", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/token",
			bytes.NewReader([]byte(`{}`)))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "PeerName is required")
	})

	// TODO(peering): add more failure cases

	t.Run("Success", func(t *testing.T) {
		body := &pbpeering.GenerateTokenRequest{
			PeerName: "peering-a",
		}

		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		var r pbpeering.GenerateTokenResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&r))

		tokenJSON, err := base64.StdEncoding.DecodeString(r.PeeringToken)
		require.NoError(t, err)

		var token structs.PeeringToken
		require.NoError(t, json.Unmarshal(tokenJSON, &token))

		require.NotNil(t, token.CA)
		require.Equal(t, []string{fmt.Sprintf("127.0.0.1:%d", a.config.GRPCTLSPort)}, token.ServerAddresses)
		require.Equal(t, "server.dc1.peering.11111111-2222-3333-4444-555555555555.consul", token.ServerName)

		// The PeerID in the token is randomly generated so we don't assert on its value.
		require.NotEmpty(t, token.PeerID)
	})

	t.Run("Success with external address", func(t *testing.T) {
		externalAddress := "32.1.2.3"
		body := &pbpeering.GenerateTokenRequest{
			PeerName:                "peering-a",
			ServerExternalAddresses: []string{externalAddress},
		}

		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		var r pbpeering.GenerateTokenResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&r))

		tokenJSON, err := base64.StdEncoding.DecodeString(r.PeeringToken)
		require.NoError(t, err)

		var token structs.PeeringToken
		require.NoError(t, json.Unmarshal(tokenJSON, &token))

		require.NotNil(t, token.CA)
		require.Equal(t, []string{externalAddress}, token.ManualServerAddresses)
		require.Equal(t, []string{fmt.Sprintf("127.0.0.1:%d", a.config.GRPCTLSPort)}, token.ServerAddresses)
		require.Equal(t, "server.dc1.peering.11111111-2222-3333-4444-555555555555.consul", token.ServerName)

		// The PeerID in the token is randomly generated so we don't assert on its value.
		require.NotEmpty(t, token.PeerID)
	})
}

// Test for GenerateToken calls at various points in a peer's lifecycle
func TestHTTP_Peering_GenerateToken_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

	body := &pbpeering.GenerateTokenRequest{
		PeerName: "peering-a",
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	getPeering := func(t *testing.T) *api.Peering {
		t.Helper()
		// Check state of peering
		req, err := http.NewRequest("GET", "/v1/peering/peering-a", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		var p *api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&p))
		return p
	}

	{
		// Call once
		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())
		// Assertions tested in TestHTTP_Peering_GenerateToken
	}

	if !t.Run("generate token called again", func(t *testing.T) {
		before := getPeering(t)
		require.Equal(t, api.PeeringStatePending, before.State)

		// Call again
		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		after := getPeering(t)
		assert.NotEqual(t, before.ModifyIndex, after.ModifyIndex)
		// blank out modify index so we can compare rest of struct
		before.ModifyIndex, after.ModifyIndex = 0, 0
		assert.Equal(t, before, after)

	}) {
		t.FailNow()
	}

}

func TestHTTP_Peering_Establish(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

	t.Run("No Body", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/establish", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "The peering arguments must be provided in the body")
	})

	t.Run("Body Invalid", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/establish", bytes.NewReader([]byte("abc")))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "Body decoding failed:")
	})

	t.Run("No Name", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/establish",
			bytes.NewReader([]byte(`{}`)))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "PeerName is required")
	})

	t.Run("No Token", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/v1/peering/establish",
			bytes.NewReader([]byte(`{"PeerName": "peer1-usw1"}`)))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "PeeringToken is required")
	})

	t.Run("Success", func(t *testing.T) {
		a2 := NewTestAgent(t, `datacenter = "dc2"`)
		testrpc.WaitForTestAgent(t, a2.RPC, "dc2")

		bodyBytes, err := json.Marshal(&pbpeering.GenerateTokenRequest{
			PeerName: "foo",
		})
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		var r pbpeering.GenerateTokenResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&r))

		b, err := json.Marshal(&pbpeering.EstablishRequest{
			PeerName:     "zip",
			PeeringToken: r.PeeringToken,
			Meta:         map[string]string{"foo": "bar"},
		})
		require.NoError(t, err)

		retry.Run(t, func(r *retry.R) {
			req, err = http.NewRequest("POST", "/v1/peering/establish", bytes.NewReader(b))
			require.NoError(r, err)

			resp = httptest.NewRecorder()
			a2.srv.h.ServeHTTP(resp, req)
			require.Equal(r, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

			// success response does not currently return a value so {} is correct
			require.Equal(r, "{}", resp.Body.String())
		})
	})
}

func TestHTTP_Peering_MethodNotAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert peerings directly to state store.
	// Note that the state store holds reference to the underlying
	// variables; do not modify them after writing.
	foo := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "foo",
			State:               pbpeering.PeeringState_ESTABLISHING,
			PeerCAPems:          nil,
			PeerServerName:      "fooservername",
			PeerServerAddresses: []string{"addr1"},
		},
	}
	_, err := a.rpcClientPeering.PeeringWrite(ctx, foo)
	require.NoError(t, err)

	req, err := http.NewRequest("PUT", "/v1/peering/foo", nil)
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusMethodNotAllowed, resp.Code)
}

func TestHTTP_Peering_Read(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert peerings directly to state store.
	// Note that the state store holds reference to the underlying
	// variables; do not modify them after writing.
	foo := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "foo",
			State:               pbpeering.PeeringState_ESTABLISHING,
			PeerCAPems:          nil,
			PeerServerName:      "fooservername",
			PeerServerAddresses: []string{"addr1"},
			Meta:                map[string]string{"foo": "bar"},
		},
	}
	_, err := a.rpcClientPeering.PeeringWrite(ctx, foo)
	require.NoError(t, err)
	bar := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "bar",
			State:               pbpeering.PeeringState_ACTIVE,
			PeerCAPems:          nil,
			PeerServerName:      "barservername",
			PeerServerAddresses: []string{"addr1"},
		},
	}
	_, err = a.rpcClientPeering.PeeringWrite(ctx, bar)
	require.NoError(t, err)

	t.Run("return foo", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/peering/foo", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		var apiResp api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiResp))

		require.Equal(t, foo.Peering.Name, apiResp.Name)
		require.Equal(t, foo.Peering.Meta, apiResp.Meta)

		require.Equal(t, 0, len(apiResp.StreamStatus.ImportedServices))
		require.Equal(t, 0, len(apiResp.StreamStatus.ExportedServices))
	})

	t.Run("not found", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/peering/baz", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusNotFound, resp.Code)
		require.Equal(t, "Peering not found for \"baz\"", resp.Body.String())
	})
}

func TestHTTP_Peering_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

	bodyBytes, err := json.Marshal(&pbpeering.GenerateTokenRequest{
		PeerName: "foo",
	})
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/v1/peering/token", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

	t.Run("read existing token before attempting delete", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/peering/foo", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		var apiResp api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiResp))
		require.Equal(t, "foo", apiResp.Name)
	})

	t.Run("delete the existing token we just read", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", "/v1/peering/foo", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
		require.Equal(t, "", resp.Body.String())
	})

	t.Run("now the token is deleted and reads should yield a 404", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, err := http.NewRequest("GET", "/v1/peering/foo", nil)
			require.NoError(r, err)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			require.Equal(r, http.StatusNotFound, resp.Code)
		})
	})

	t.Run("delete a token that does not exist", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", "/v1/peering/baz", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)

		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func TestHTTP_Peering_List(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert peerings directly to state store.
	// Note that the state store holds reference to the underlying
	// variables; do not modify them after writing.
	foo := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "foo",
			State:               pbpeering.PeeringState_ESTABLISHING,
			PeerCAPems:          nil,
			PeerServerName:      "fooservername",
			PeerServerAddresses: []string{"addr1"},
		},
	}
	_, err := a.rpcClientPeering.PeeringWrite(ctx, foo)
	require.NoError(t, err)
	bar := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "bar",
			State:               pbpeering.PeeringState_ACTIVE,
			PeerCAPems:          nil,
			PeerServerName:      "barservername",
			PeerServerAddresses: []string{"addr1"},
		},
	}
	_, err = a.rpcClientPeering.PeeringWrite(ctx, bar)
	require.NoError(t, err)

	t.Run("return all", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/peerings", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		var apiResp []*api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiResp))

		require.Len(t, apiResp, 2)

		for _, p := range apiResp {
			require.Equal(t, 0, len(p.StreamStatus.ImportedServices))
			require.Equal(t, 0, len(p.StreamStatus.ExportedServices))
		}
	})
}
