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

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

var validCA = `
-----BEGIN CERTIFICATE-----
MIICmDCCAj6gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg
Q0EgNzAeFw0xODA1MjExNjMzMjhaFw0yODA1MTgxNjMzMjhaMBYxFDASBgNVBAMT
C0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAER0qlxjnRcMEr
iSGlH7G7dYU7lzBEmLUSMZkyBbClmyV8+e8WANemjn+PLnCr40If9cmpr7RnC9Qk
GTaLnLiF16OCAXswggF3MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/
MGgGA1UdDgRhBF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1OTpjMjpmYTo0ZTo3
NTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToyNDpiMDowNDpiMzpl
ODo5Nzo1Yjo3ZTBqBgNVHSMEYzBhgF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1
OTpjMjpmYTo0ZTo3NTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToy
NDpiMDowNDpiMzplODo5Nzo1Yjo3ZTA/BgNVHREEODA2hjRzcGlmZmU6Ly8xMjRk
ZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIuY29uc3VsMD0GA1UdHgEB
/wQzMDGgLzAtgisxMjRkZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIu
Y29uc3VsMAoGCCqGSM49BAMCA0gAMEUCIQDzkkI7R+0U12a+zq2EQhP/n2mHmta+
fs2hBxWIELGwTAIgLdO7RRw+z9nnxCIA6kNl//mIQb+PGItespiHZKAz74Q=
-----END CERTIFICATE-----
`

func TestHTTP_Peering_GenerateToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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

		require.Nil(t, token.CA)
		require.Equal(t, []string{fmt.Sprintf("127.0.0.1:%d", a.config.ServerPort)}, token.ServerAddresses)
		require.Equal(t, "server.dc1.consul", token.ServerName)

		// The PeerID in the token is randomly generated so we don't assert on its value.
		require.NotEmpty(t, token.PeerID)
	})
}

func TestHTTP_Peering_Establish(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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

	// TODO(peering): add more failure cases

	t.Run("Success", func(t *testing.T) {
		token := structs.PeeringToken{
			CA:              []string{validCA},
			ServerName:      "server.dc1.consul",
			ServerAddresses: []string{fmt.Sprintf("1.2.3.4:%d", 443)},
			PeerID:          "a0affd3e-f1c8-4bb9-9168-90fd902c441d",
		}
		tokenJSON, _ := json.Marshal(&token)
		tokenB64 := base64.StdEncoding.EncodeToString(tokenJSON)
		body := &pbpeering.EstablishRequest{
			PeerName:     "peering-a",
			PeeringToken: tokenB64,
			Meta:         map[string]string{"foo": "bar"},
		}

		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/v1/peering/establish", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "expected 200, got %d: %v", resp.Code, resp.Body.String())

		// success response does not currently return a value so {} is correct
		require.Equal(t, "{}", resp.Body.String())
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
			State:               pbpeering.PeeringState_INITIAL,
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
			State:               pbpeering.PeeringState_INITIAL,
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	foo := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                "foo",
			State:               pbpeering.PeeringState_INITIAL,
			PeerCAPems:          nil,
			PeerServerName:      "fooservername",
			PeerServerAddresses: []string{"addr1"},
		},
	}
	_, err := a.rpcClientPeering.PeeringWrite(ctx, foo)
	require.NoError(t, err)

	t.Run("read existing token before attempting delete", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/peering/foo", nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)

		var apiResp api.Peering
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiResp))

		require.Equal(t, foo.Peering.Name, apiResp.Name)
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
			State:               pbpeering.PeeringState_INITIAL,
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
	})
}
