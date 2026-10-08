// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAPI_ConfigEntries_TerminatingGatewayCredential catches API JSON read and
// write paths that omit the non-secret credential policy.
func TestAPI_ConfigEntries_TerminatingGatewayCredential(t *testing.T) {
	mapi, client := setupMockAPI(t)
	entry := &TerminatingGatewayConfigEntry{
		Kind:      TerminatingGateway,
		Name:      "camp-egress",
		Namespace: "inference",
		Partition: "payments",
		CredentialInjection: &GatewayCredentialInjection{
			UDSPath:        "/run/camp-auth/processor.sock",
			MessageTimeout: "250ms",
		},
		Services: []LinkedService{{
			Name:      "openai-a",
			Namespace: "inference",
			CAFile:    "/etc/ssl/certs/ca-certificates.crt",
			SNI:       "api.openai.com",
			Credential: &GatewayServiceCredential{
				Mode:      "inject",
				BindingID: "openai-a",
			},
		}},
	}

	body, err := json.Marshal(entry)
	require.NoError(t, err)
	body = append(body, '\n')
	mapi.static(http.MethodPut, "/v1/config", body).Return(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("true"))
		require.NoError(t, err)
	})
	mapi.static(http.MethodGet, "/v1/config/terminating-gateway/camp-egress", nil).Return(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, entry.Namespace, r.URL.Query().Get("ns"))
		require.Equal(t, entry.Partition, r.URL.Query().Get("partition"))
		require.NoError(t, json.NewEncoder(w).Encode(entry))
	})

	_, _, err = client.ConfigEntries().Set(entry, nil)
	require.NoError(t, err)

	read, _, err := client.ConfigEntries().Get(TerminatingGateway, "camp-egress", &QueryOptions{
		Namespace: entry.Namespace,
		Partition: entry.Partition,
	})
	require.NoError(t, err)
	got, ok := read.(*TerminatingGatewayConfigEntry)
	require.True(t, ok)
	require.Equal(t, entry.Namespace, got.Namespace)
	require.Equal(t, entry.Partition, got.Partition)
	require.Equal(t, entry.CredentialInjection, got.CredentialInjection)
	require.Equal(t, entry.Services, got.Services)
}
