// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

// operator_feature_gate_endpoint_http_test.go contains HTTP-layer tests for
// OperatorFeatureGateList and OperatorFeatureGate that exercise paths not
// covered by the existing four unit tests.  Integration tests that require a
// full agent use NewTestAgent + testrpc.WaitForLeader.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

// ---------------------------------------------------------------------------
// Unit-level tests (no agent required)
// ---------------------------------------------------------------------------

// TestOperatorFeatureGate_EmptyNamePUT confirms that an empty feature name on
// PUT also triggers the BadRequest branch (matches the GET path through the
// same handler).
func TestOperatorFeatureGate_EmptyNamePUT(t *testing.T) {
	h := &HTTPHandlers{}
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/operator/feature/", strings.NewReader(`{"Enabled":true}`))

	_, err := h.OperatorFeatureGate(resp, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "feature gate name is required")
}

// TestOperatorFeatureGate_UnsupportedMethod confirms that a DELETE request
// returns MethodNotAllowedError.
func TestOperatorFeatureGate_UnsupportedMethod(t *testing.T) {
	h := &HTTPHandlers{}
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/operator/feature/api-gateway-upstream-routing", nil)

	_, err := h.OperatorFeatureGate(resp, req)
	require.Error(t, err)
	var methodErr MethodNotAllowedError
	require.ErrorAs(t, err, &methodErr, "expected MethodNotAllowedError")
}

// TestOperatorFeatureGate_PutInvalidCAS confirms that a non-numeric ?cas=
// query value returns BadRequest.
func TestOperatorFeatureGate_PutInvalidCAS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPut,
		"/v1/operator/feature/api-gateway-upstream-routing?cas=notanumber",
		strings.NewReader(`{"Enabled":true}`),
	)

	_, err := a.srv.OperatorFeatureGate(resp, req)
	require.Error(t, err)
	httpErr, ok := err.(HTTPError)
	require.True(t, ok, "expected HTTPError, got %T: %v", err, err)
	require.Equal(t, http.StatusBadRequest, httpErr.StatusCode)
	require.Contains(t, httpErr.Reason, "error parsing cas value")
}

// TestFeatureGateToAPI_AllFields verifies that every field of FeatureGateInfo
// is mapped to the api.FeatureGate struct.
func TestFeatureGateToAPI_AllFields(t *testing.T) {
	info := structs.FeatureGateInfo{
		Name:                 "my-gate",
		Stage:                "experimental",
		MinVersion:           "2.1.0",
		DefaultEnabled:       true,
		BeforeMinimumVersion: "disabled",
		Description:          "A test gate",
		Owner:                "test-team",
		DesiredEnabled:       true,
		EffectiveEnabled:     false,
		Eligible:             false,
		Source:               "operator",
		Reason:               structs.FeatureGateReasonBelowMinimumVersion,
		PolicyIndex:          5,
		StatusIndex:          7,
	}
	got := featureGateToAPI(info)
	require.Equal(t, info.Name, got.Name)
	require.Equal(t, info.Stage, got.Stage)
	require.Equal(t, info.MinVersion, got.MinVersion)
	require.Equal(t, info.DefaultEnabled, got.DefaultEnabled)
	require.Equal(t, info.BeforeMinimumVersion, got.BeforeMinimumVersion)
	require.Equal(t, info.Description, got.Description)
	require.Equal(t, info.Owner, got.Owner)
	require.Equal(t, info.DesiredEnabled, got.DesiredEnabled)
	require.Equal(t, info.EffectiveEnabled, got.EffectiveEnabled)
	require.Equal(t, info.Eligible, got.Eligible)
	require.Equal(t, info.Source, got.Source)
	require.Equal(t, string(info.Reason), got.Reason)
	require.Equal(t, info.PolicyIndex, got.PolicyIndex)
	require.Equal(t, info.StatusIndex, got.StatusIndex)
}

// ---------------------------------------------------------------------------
// Integration tests (full agent)
// ---------------------------------------------------------------------------

// TestOperatorFeatureGateList_Integration verifies the full GET /features path
// against a live single-node cluster.
func TestOperatorFeatureGateList_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	req := httptest.NewRequest(http.MethodGet, "/v1/operator/features", nil)
	resp := httptest.NewRecorder()

	// Wait until the feature-gate policy is initialized (the handler returns
	// an empty slice with Uninitialized header until then).
	var features []api.FeatureGate
	require.Eventually(t, func() bool {
		resp = httptest.NewRecorder()
		raw, err := a.srv.OperatorFeatureGateList(resp, req)
		if err != nil {
			return false
		}
		if raw == nil {
			return false
		}
		fgs, ok := raw.([]api.FeatureGate)
		if !ok {
			return false
		}
		features = fgs
		return resp.Header().Get("X-Consul-Feature-Gates-Uninitialized") == "" && len(features) > 0
	}, 5*time.Second, 50*time.Millisecond, "feature-gate list should become non-empty once initialized")

	require.NotEmpty(t, features)
	// Every feature must have a non-empty name.
	for _, f := range features {
		require.NotEmpty(t, f.Name)
	}
}

// TestOperatorFeatureGate_GetSingleFeature verifies the GET /feature/{name} path.
func TestOperatorFeatureGate_GetSingleFeature(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	featureName := featuregate.APIGatewayUpstreamRouting.String()
	url := fmt.Sprintf("/v1/operator/feature/%s", featureName)

	var got api.FeatureGate
	require.Eventually(t, func() bool {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		resp := httptest.NewRecorder()
		raw, err := a.srv.OperatorFeatureGate(resp, req)
		if err != nil {
			return false
		}
		fg, ok := raw.(api.FeatureGate)
		if !ok {
			return false
		}
		got = fg
		return true
	}, 5*time.Second, 50*time.Millisecond)

	require.Equal(t, featureName, got.Name)
}

// TestOperatorFeatureGate_GetUnknownFeatureReturnsError verifies that an
// unknown feature name returns an error (the RPC error propagates through
// the HTTP handler as a plain error, not an HTTPError).
func TestOperatorFeatureGate_GetUnknownFeatureReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Wait for init first so we know the error is about the unknown name.
	require.Eventually(t, func() bool {
		req := httptest.NewRequest(http.MethodGet, "/v1/operator/features", nil)
		resp := httptest.NewRecorder()
		raw, err := a.srv.OperatorFeatureGateList(resp, req)
		if err != nil || raw == nil {
			return false
		}
		fgs, ok := raw.([]api.FeatureGate)
		return ok && len(fgs) > 0
	}, 5*time.Second, 50*time.Millisecond, "wait for feature gate list to be initialized")

	req := httptest.NewRequest(http.MethodGet, "/v1/operator/feature/no-such-feature", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.OperatorFeatureGate(resp, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown feature gate")
}

// TestOperatorFeatureGate_PutSuccessful verifies the PUT /feature/{name} path
// returns Applied=true and the updated feature state.
func TestOperatorFeatureGate_PutSuccessful(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	featureName := featuregate.APIGatewayUpstreamRouting.String()

	// Wait for initialization.
	require.Eventually(t, func() bool {
		req := httptest.NewRequest(http.MethodGet, "/v1/operator/features", nil)
		resp := httptest.NewRecorder()
		raw, err := a.srv.OperatorFeatureGateList(resp, req)
		if err != nil || raw == nil {
			return false
		}
		fgs, ok := raw.([]api.FeatureGate)
		return ok && len(fgs) > 0
	}, 5*time.Second, 50*time.Millisecond)

	body, _ := json.Marshal(api.FeatureGateSetRequest{Enabled: true})
	url := fmt.Sprintf("/v1/operator/feature/%s", featureName)
	req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	resp := httptest.NewRecorder()

	raw, err := a.srv.OperatorFeatureGate(resp, req)
	require.NoError(t, err)
	setResp, ok := raw.(api.FeatureGateSetResponse)
	require.True(t, ok)
	require.True(t, setResp.Applied)
	require.Equal(t, featureName, setResp.Feature.Name)
}
