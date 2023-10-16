package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPHandlers_ACLLegacy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	type testCase struct {
		method string
		path   string
	}

	run := func(t *testing.T, tc testCase) {
		req, err := http.NewRequest(tc.method, tc.path, nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()

		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusGone, resp.Code)
		require.Contains(t, resp.Body.String(), "the legacy ACL system was removed")
	}

	var testCases = []testCase{
		{method: http.MethodPut, path: "/v1/acl/create"},
		{method: http.MethodPut, path: "/v1/acl/update"},
		{method: http.MethodPut, path: "/v1/acl/destroy/ID"},
		{method: http.MethodGet, path: "/v1/acl/info/ID"},
		{method: http.MethodPut, path: "/v1/acl/clone/ID"},
		{method: http.MethodGet, path: "/v1/acl/list"},
	}

	for _, tc := range testCases {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			run(t, tc)
		})
	}
}
