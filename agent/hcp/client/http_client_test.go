// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient(t *testing.T) {
	mockCfg := config.MockCloudCfg{}
	mockHCPCfg, err := mockCfg.HCPConfig()
	require.NoError(t, err)

	client := NewHTTPClient(mockHCPCfg.APITLSConfig(), mockHCPCfg, hclog.NewNullLogger())
	require.NotNil(t, client)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
	}))
	client.Get(srv.URL)
}
