// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_OperatorFeatureGateList(t *testing.T) {
	mapi, client := setupMockAPI(t)
	want := []FeatureGate{{Name: "gate-one", PolicyIndex: 10, StatusIndex: 11}}
	mapi.static(http.MethodGet, "/v1/operator/features", nil).Return(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "dc2", r.URL.Query().Get("dc"))
		require.Contains(t, r.URL.Query(), "stale")
		w.Header().Set("X-Consul-Index", "11")
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(want))
	}).Once()

	got, meta, err := client.Operator().FeatureGateList(&QueryOptions{Datacenter: "dc2", AllowStale: true})
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, uint64(11), meta.LastIndex)
}

func TestAPI_OperatorFeatureGateGet(t *testing.T) {
	mapi, client := setupMockAPI(t)
	want := FeatureGate{Name: "gate/with space", DesiredEnabled: true, EffectiveEnabled: true}
	mapi.static(http.MethodGet, "/v1/operator/feature/gate%2Fwith%20space", nil).Return(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Consul-Index", "21")
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(want))
	}).Once()

	got, meta, err := client.Operator().FeatureGateGet(want.Name, nil)
	require.NoError(t, err)
	require.Equal(t, &want, got)
	require.Equal(t, uint64(21), meta.LastIndex)
}

func TestAPI_OperatorFeatureGateSet(t *testing.T) {
	mapi, client := setupMockAPI(t)
	want := FeatureGateSetResponse{Applied: true, Feature: FeatureGate{Name: "gate-one", Source: "operator"}}
	body := []byte("{\"Enabled\":true}\n")
	mapi.static(http.MethodPut, "/v1/operator/feature/gate-one", body).Return(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "42", r.URL.Query().Get("cas"))
		require.Equal(t, "dc2", r.URL.Query().Get("dc"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(want))
	}).Once()

	got, err := client.Operator().FeatureGateSet("gate-one", true, 42, &WriteOptions{Datacenter: "dc2"})
	require.NoError(t, err)
	require.Equal(t, &want, got)
}

func TestAPI_OperatorFeatureGateSet_OmitsZeroCAS(t *testing.T) {
	mapi, client := setupMockAPI(t)
	body := []byte("{\"Enabled\":false}\n")
	mapi.static(http.MethodPut, "/v1/operator/feature/gate-one", body).Return(func(w http.ResponseWriter, r *http.Request) {
		require.NotContains(t, r.URL.Query(), "cas")
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(FeatureGateSetResponse{}))
	}).Once()

	_, err := client.Operator().FeatureGateSet("gate-one", false, 0, nil)
	require.NoError(t, err)
}
