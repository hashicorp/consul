// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package get

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

func TestCmd_Run(t *testing.T) {
	const featureName = "api-gateway-upstream-routing"

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/operator/feature/"+featureName, r.URL.Path)
			require.Contains(t, r.URL.Query(), "stale")
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(api.FeatureGate{Name: featureName, EffectiveEnabled: true}))
		}))
		defer srv.Close()

		ui := cli.NewMockUi()
		code := New(ui).Run([]string{"-http-addr=" + srv.URL, "-stale", "-format=json", featureName})
		require.Equal(t, 0, code, "stderr: %s", ui.ErrorWriter.String())
		var got []api.FeatureGate
		require.NoError(t, json.Unmarshal([]byte(ui.OutputWriter.String()), &got))
		require.Len(t, got, 1)
		require.Equal(t, featureName, got[0].Name)
	})

	t.Run("api error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"Errors":["unknown feature gate"]}`, http.StatusNotFound)
		}))
		defer srv.Close()

		ui := cli.NewMockUi()
		code := New(ui).Run([]string{"-http-addr=" + srv.URL, "missing"})
		require.Equal(t, 1, code)
		require.Contains(t, ui.ErrorWriter.String(), "Error querying feature gate")
		require.Contains(t, ui.ErrorWriter.String(), "may not be initialized yet")
	})

	for _, args := range [][]string{nil, {"one", "two"}} {
		name := fmt.Sprintf("%d args", len(args))
		t.Run(name, func(t *testing.T) {
			ui := cli.NewMockUi()
			require.Equal(t, 1, New(ui).Run(args))
			require.Contains(t, ui.ErrorWriter.String(), "Exactly one feature name is required")
		})
	}
}

func TestCmd_HelpSynopsis(t *testing.T) {
	c := New(cli.NewMockUi())
	require.NotEmpty(t, c.Help())
	require.NotEmpty(t, c.Synopsis())
	require.NotContains(t, c.Help(), "\t")
}
