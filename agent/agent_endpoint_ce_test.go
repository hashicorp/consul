// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package agent

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/testrpc"
)

func TestAgent_Self_VersionLacksEnt(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	cases := map[string]struct {
		hcl       string
		expectXDS bool
	}{
		"normal": {
			hcl: "primary_datacenter = \"dc1\"",
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			a := NewTestAgent(t, tc.hcl)
			defer a.Shutdown()

			testrpc.WaitForTestAgent(t, a.RPC, "dc1")
			req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
			resp := httptest.NewRecorder()
			a.srv.h.ServeHTTP(resp, req)

			dec := json.NewDecoder(resp.Body)
			var out map[string]map[string]interface{}
			require.NoError(t, dec.Decode(&out))
			require.NotContains(t, out["Config"]["Version"], "ent")
		})
	}
}
