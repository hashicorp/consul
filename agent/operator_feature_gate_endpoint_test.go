// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
)

func newFeatureGateHTTPHandlers(t *testing.T, rpc func(*structs.FeatureGateQueryRequest, *structs.FeatureGateQueryResponse)) *HTTPHandlers {
	t.Helper()
	delegate := &delegateMock{}
	delegate.On("RPC", "Operator.FeatureGateGet", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			rpc(args.Get(1).(*structs.FeatureGateQueryRequest), args.Get(2).(*structs.FeatureGateQueryResponse))
		}).Return(nil)
	t.Cleanup(func() { delegate.AssertExpectations(t) })
	agent := &Agent{
		config:   &config.RuntimeConfig{Datacenter: "dc1"},
		tokens:   new(token.Store),
		delegate: delegate,
	}
	initializeFeatureGateHTTPTestAgent(t, agent)
	return &HTTPHandlers{agent: agent}
}

func TestOperatorFeatureGateList_Uninitialized(t *testing.T) {
	h := newFeatureGateHTTPHandlers(t, func(_ *structs.FeatureGateQueryRequest, reply *structs.FeatureGateQueryResponse) {
		reply.Uninitialized = true
		reply.Features = []structs.FeatureGateInfo{}
	})
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/operator/features", nil)

	got, err := h.OperatorFeatureGateList(resp, req)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Equal(t, "true", resp.Header().Get("X-Consul-Feature-Gates-Uninitialized"))
}

func TestOperatorFeatureGateGet_Uninitialized(t *testing.T) {
	h := newFeatureGateHTTPHandlers(t, func(args *structs.FeatureGateQueryRequest, reply *structs.FeatureGateQueryResponse) {
		require.Equal(t, "some-feature", args.Name)
		reply.Uninitialized = true
	})
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/operator/feature/some-feature", nil)

	_, err := h.OperatorFeatureGate(resp, req)
	var httpErr HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusServiceUnavailable, httpErr.StatusCode)
}

func TestOperatorFeatureGateSet_UninitializedReturnsServiceUnavailable(t *testing.T) {
	for name, rpcErr := range map[string]error{
		"uninitialized policy":     fmt.Errorf("rpc error making call: %w", structs.ErrFeatureGatePolicyUninitialized),
		"leader without gate RPCs": fmt.Errorf("rpc error making call: rpc: can't find method Operator.FeatureGateSet"),
	} {
		t.Run(name, func(t *testing.T) {
			delegate := &delegateMock{}
			delegate.On("RPC", "Operator.FeatureGateSet", mock.Anything, mock.Anything).Return(rpcErr)
			t.Cleanup(func() { delegate.AssertExpectations(t) })
			agent := &Agent{
				config:   &config.RuntimeConfig{Datacenter: "dc1"},
				tokens:   new(token.Store),
				delegate: delegate,
			}
			initializeFeatureGateHTTPTestAgent(t, agent)
			h := &HTTPHandlers{agent: agent}

			req := httptest.NewRequest(http.MethodPut, "/v1/operator/feature/some-feature", strings.NewReader(`{"Enabled":true}`))
			_, err := h.OperatorFeatureGate(httptest.NewRecorder(), req)
			var httpErr HTTPError
			require.ErrorAs(t, err, &httpErr)
			require.Equal(t, http.StatusServiceUnavailable, httpErr.StatusCode)
		})
	}
}

func TestOperatorFeatureGate_InvalidName(t *testing.T) {
	h := &HTTPHandlers{}
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/operator/feature/", nil)

	_, err := h.OperatorFeatureGate(resp, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "feature gate name is required")
}

func TestOperatorFeatureGate_PutParsingError(t *testing.T) {
	h := &HTTPHandlers{}
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/operator/feature/some-feature", strings.NewReader(`{"enabled":`))

	_, err := h.OperatorFeatureGate(resp, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "error parsing feature gate setting")
}

func TestFeatureGateToAPI(t *testing.T) {
	info := structs.FeatureGateInfo{Name: "demo", DesiredEnabled: true, EffectiveEnabled: false, Reason: structs.FeatureGateReasonOperatorEnabled}
	apiFeature := featureGateToAPI(info)
	require.Equal(t, "demo", apiFeature.Name)
	require.Equal(t, string(structs.FeatureGateReasonOperatorEnabled), apiFeature.Reason)
}
