// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/testrpc"
)

func newChunkedRequest(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.ContentLength = -1
	req.TransferEncoding = []string{"chunked"}
	return req
}

func TestLimitRequestBody_Chunked(t *testing.T) {
	s := &HTTPHandlers{agent: &Agent{config: &config.RuntimeConfig{HTTPMaxRequestBodySize: 16}}}

	t.Run("body at the limit is delivered in full", func(t *testing.T) {
		body := strings.Repeat("x", 16)
		req := newChunkedRequest(http.MethodPut, "/v1/config", strings.NewReader(body))
		require.True(t, s.limitRequestBody(httptest.NewRecorder(), req))
		got, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, body, string(got))
	})

	t.Run("body one byte over the limit is rejected", func(t *testing.T) {
		req := newChunkedRequest(http.MethodPut, "/v1/config", strings.NewReader(strings.Repeat("x", 17)))
		require.False(t, s.limitRequestBody(httptest.NewRecorder(), req))
		require.Equal(t, http.NoBody, req.Body)
	})

	t.Run("JSON value followed by padding past the limit is rejected", func(t *testing.T) {
		req := newChunkedRequest(http.MethodPut, "/v1/config", strings.NewReader("{}"+strings.Repeat(" ", 15)))
		require.False(t, s.limitRequestBody(httptest.NewRecorder(), req))
	})

	t.Run("read errors are passed through to the handler", func(t *testing.T) {
		readErr := errors.New("client went away")
		req := newChunkedRequest(http.MethodPut, "/v1/config", io.MultiReader(strings.NewReader("{}"), iotest.ErrReader(readErr)))
		require.True(t, s.limitRequestBody(httptest.NewRecorder(), req))
		got, err := io.ReadAll(req.Body)
		require.ErrorIs(t, err, readErr)
		require.Equal(t, "{}", string(got))
	})
}

func TestHTTPRequestBodyLimit_ChunkedTrailingData(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// A single valid JSON value followed by padding: decoding stops after the
	// value, so the limit must apply to the whole body rather than to the bytes
	// the handler happens to read.
	padded := append([]byte(`{}`), bytes.Repeat([]byte(" "), maxRequestBodyBytes)...)
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "anonymous login", method: http.MethodPost, path: "/v1/acl/login"},
		{name: "config map", method: http.MethodPut, path: "/v1/config"},
		{name: "discovery chain map", method: http.MethodPost, path: "/v1/discovery-chain/db"},
		{name: "resource api", method: http.MethodPut, path: "/api/demo/v2/artist/foo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := newChunkedRequest(tc.method, tc.path, bytes.NewReader(padded))
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)
			require.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
		})
	}

	t.Run("chunked body within the limit is delivered to the handler", func(t *testing.T) {
		body := `{"Kind":"service-defaults","Name":"chunked-web","Protocol":"http"}`
		req := newChunkedRequest(http.MethodPut, "/v1/config", strings.NewReader(body))
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

		getResp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(getResp, httptest.NewRequest(http.MethodGet, "/v1/config/service-defaults/chunked-web", nil))
		require.Equal(t, http.StatusOK, getResp.Code, getResp.Body.String())
		require.Contains(t, getResp.Body.String(), `"Protocol":"http"`)
	})
}
