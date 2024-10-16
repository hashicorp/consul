// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package agent

import (
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthServiceNodes_SamenessGroup_ErrorsOnCE(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1&sameness-group=foo", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.HealthServiceNodes(resp, req)
	require.ErrorContains(t, err, "sameness groups are not supported in consul CE")
}
