// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package set

// set_run_test.go covers cmd.Run execution paths: successful set,
// CAS failure (Applied=false), invalid value, API failure, and output
// formatting.  These tests use an httptest.Server to mock the Consul
// HTTP API so they can run without a full consul cluster.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

// newMockServer starts an httptest.Server that serves the minimal set of
// Consul API endpoints needed by the set command.
//
//   - GET /v1/agent/self  – required by api.Client for DC discovery
//   - PUT /v1/operator/feature/{name} – the actual feature-gate set endpoint
func newMockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	// The api.Client pings /v1/agent/self on certain code paths; stub it.
	mux.HandleFunc("/v1/agent/self", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Minimal response that satisfies the client.
		fmt.Fprintf(w, `{"Config":{"Datacenter":"dc1","NodeName":"mock","Server":true},"Member":{"Name":"mock","Addr":"127.0.0.1","Port":8300,"Tags":{},"Status":1,"ProtocolMin":1,"ProtocolMax":3,"ProtocolCur":2,"DelegateMin":1,"DelegateMax":5,"DelegateCur":4}}`)
	})
	mux.HandleFunc("/v1/operator/feature/", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// featureGateSetResponse builds a JSON-encoded FeatureGateSetResponse.
func featureGateSetResponse(applied bool, name string, policyIndex uint64) string {
	resp := api.FeatureGateSetResponse{
		Applied: applied,
		Feature: api.FeatureGate{
			Name:        name,
			Stage:       "experimental",
			MinVersion:  "2.1.0",
			PolicyIndex: policyIndex,
			StatusIndex: policyIndex + 1,
			Source:      "operator",
			Reason:      "operator-enabled",
		},
	}
	out, _ := json.Marshal(resp)
	return string(out)
}

// ---------------------------------------------------------------------------
// Successful set (pretty output)
// ---------------------------------------------------------------------------

func TestCmd_Run_Success(t *testing.T) {
	const featureName = "api-gateway-upstream-routing"

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, featureName))

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, featureGateSetResponse(true, featureName, 10))
	})

	ui := cli.NewMockUi()
	c := New(ui)
	code := c.Run([]string{
		"-http-addr=" + srv.URL,
		featureName,
		"enabled",
	})
	require.Equal(t, 0, code, "stderr: %s", ui.ErrorWriter.String())
	require.Contains(t, ui.OutputWriter.String(), featureName)
}

// ---------------------------------------------------------------------------
// Successful set (JSON output)
// ---------------------------------------------------------------------------

func TestCmd_Run_SuccessJSON(t *testing.T) {
	const featureName = "api-gateway-upstream-routing"

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, featureGateSetResponse(true, featureName, 5))
	})

	ui := cli.NewMockUi()
	c := New(ui)
	code := c.Run([]string{
		"-http-addr=" + srv.URL,
		"-format=json",
		featureName,
		"enabled",
	})
	require.Equal(t, 0, code, "stderr: %s", ui.ErrorWriter.String())

	// Output must be valid JSON containing the feature name.
	out := ui.OutputWriter.String()
	var features []api.FeatureGate
	require.NoError(t, json.Unmarshal([]byte(out), &features))
	require.Len(t, features, 1)
	require.Equal(t, featureName, features[0].Name)
}

// ---------------------------------------------------------------------------
// CAS failure: Applied=false returns exit code 1
// ---------------------------------------------------------------------------

func TestCmd_Run_CASFailure(t *testing.T) {
	const featureName = "api-gateway-upstream-routing"

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// CAS check: verify the ?cas query param is forwarded.
		require.Equal(t, "42", r.URL.Query().Get("cas"))
		w.Header().Set("Content-Type", "application/json")
		// Applied=false simulates a stale policy index.
		fmt.Fprint(w, featureGateSetResponse(false, featureName, 40))
	})

	ui := cli.NewMockUi()
	c := New(ui)
	code := c.Run([]string{
		"-http-addr=" + srv.URL,
		"-cas=42",
		featureName,
		"enabled",
	})
	require.Equal(t, 1, code, "CAS failure should return exit code 1")
	require.Contains(t, ui.ErrorWriter.String(), "not updated")
}

// ---------------------------------------------------------------------------
// Invalid value (neither enabled/disabled nor true/false)
// ---------------------------------------------------------------------------

func TestCmd_Run_InvalidValue(t *testing.T) {
	ui := cli.NewMockUi()
	c := New(ui)
	code := c.Run([]string{"api-gateway-upstream-routing", "yes"})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), "Invalid enabled value")
}

// ---------------------------------------------------------------------------
// Missing positional arguments
// ---------------------------------------------------------------------------

func TestCmd_Run_MissingArgs(t *testing.T) {
	ui := cli.NewMockUi()
	c := New(ui)
	code := c.Run([]string{"api-gateway-upstream-routing"})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), "feature name and enabled|disabled value are required")
}

func TestCmd_Run_NoArgs(t *testing.T) {
	ui := cli.NewMockUi()
	c := New(ui)
	code := c.Run([]string{})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), "feature name and enabled|disabled value are required")
}

// ---------------------------------------------------------------------------
// Invalid -cas value
// ---------------------------------------------------------------------------

func TestCmd_Run_InvalidCASValue(t *testing.T) {
	ui := cli.NewMockUi()
	c := New(ui)
	// -cas is a string flag; providing "abc" is only detected after flag parse.
	code := c.Run([]string{"-cas=abc", "api-gateway-upstream-routing", "enabled"})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), "Invalid -cas value")
}

func TestCmd_Run_NullCASIsIgnored(t *testing.T) {
	const featureName = "api-gateway-upstream-routing"

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// "null" is the jq expansion for a missing field; must not set ?cas.
		require.Empty(t, r.URL.Query().Get("cas"), "null CAS should not be forwarded")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, featureGateSetResponse(true, featureName, 1))
	})

	ui := cli.NewMockUi()
	c := New(ui)
	code := c.Run([]string{"-http-addr=" + srv.URL, "-cas=null", featureName, "enabled"})
	require.Equal(t, 0, code, "stderr: %s", ui.ErrorWriter.String())
}

// ---------------------------------------------------------------------------
// API failure (non-200 from server)
// ---------------------------------------------------------------------------

func TestCmd_Run_APIFailure(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"Errors":["unknown feature gate"]}`, http.StatusNotFound)
	})

	ui := cli.NewMockUi()
	c := New(ui)
	code := c.Run([]string{"-http-addr=" + srv.URL, "does-not-exist", "enabled"})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), "Error updating feature gate")
}

// ---------------------------------------------------------------------------
// Help and Synopsis
// ---------------------------------------------------------------------------

func TestCmd_HelpSynopsis(t *testing.T) {
	ui := cli.NewMockUi()
	c := New(ui)
	require.NotEmpty(t, c.Help())
	require.NotEmpty(t, c.Synopsis())
	// Help must not contain hard-tab characters (Consul convention).
	require.NotContains(t, c.Help(), "\t")
}

// ---------------------------------------------------------------------------
// Flags interleaved with positional arguments
// ---------------------------------------------------------------------------

func TestCmd_Run_FlagsBetweenPositionals(t *testing.T) {
	const featureName = "api-gateway-upstream-routing"

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "99", r.URL.Query().Get("cas"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, featureGateSetResponse(true, featureName, 99))
	})

	ui := cli.NewMockUi()
	c := New(ui)
	// Flag -cas between the two positional args.
	code := c.Run([]string{"-http-addr=" + srv.URL, featureName, "-cas=99", "enabled"})
	require.Equal(t, 0, code, "stderr: %s", ui.ErrorWriter.String())
}
