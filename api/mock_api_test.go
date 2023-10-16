// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockAPI struct {
	ts *httptest.Server
	t  *testing.T
	mock.Mock
}

func setupMockAPI(t *testing.T) (*mockAPI, *Client) {
	mapi := mockAPI{t: t}
	mapi.Test(t)
	mapi.ts = httptest.NewServer(&mapi)
	t.Cleanup(func() {
		mapi.ts.Close()
		mapi.Mock.AssertExpectations(t)
	})

	cfg := DefaultConfig()
	cfg.Address = mapi.ts.URL

	client, err := NewClient(cfg)
	require.NoError(t, err)
	return &mapi, client
}

func (m *mockAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body interface{}

	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			body = bodyBytes

			var bodyMap map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
				body = bodyMap
			}
		}
	}

	ret := m.Called(r.Method, r.URL.Path, body)

	if replyFn, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request)); ok {
		replyFn(w, r)
		return
	}
}

func (m *mockAPI) static(method string, path string, body interface{}) *mock.Call {
	return m.On("ServeHTTP", method, path, body)
}

func (m *mockAPI) withReply(method, path string, body interface{}, status int, reply interface{}) *mock.Call {
	return m.static(method, path, body).Return(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)

		if reply == nil {
			return
		}

		rdr, ok := reply.(io.Reader)
		if ok {
			io.Copy(w, rdr)
			return
		}

		enc := json.NewEncoder(w)
		require.NoError(m.t, enc.Encode(reply))
	})
}
