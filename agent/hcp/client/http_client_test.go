// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient(t *testing.T) {
	mockCfg := config.MockCloudCfg{}
	mockHCPCfg, err := mockCfg.HCPConfig()
	require.NoError(t, err)

	client := NewHTTPClient(mockHCPCfg.APITLSConfig(), mockHCPCfg)
	require.NotNil(t, client)

	var req *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
	}))
	_, err = client.Get(srv.URL)
	require.NoError(t, err)
	require.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
}
