// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"encoding/json"
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
			require.Equal(t, "/v1/operator/features", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode([]api.FeatureGate{{Name: featureName}}))
		}))
		defer srv.Close()

		ui := cli.NewMockUi()
		code := New(ui).Run([]string{"-http-addr=" + srv.URL, "-format=json"})
		require.Equal(t, 0, code, "stderr: %s", ui.ErrorWriter.String())
		var got []api.FeatureGate
		require.NoError(t, json.Unmarshal([]byte(ui.OutputWriter.String()), &got))
		require.Equal(t, featureName, got[0].Name)
	})

	t.Run("uninitialized empty response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode([]api.FeatureGate{}))
		}))
		defer srv.Close()

		ui := cli.NewMockUi()
		code := New(ui).Run([]string{"-http-addr=" + srv.URL})
		require.Equal(t, 0, code)
		require.Contains(t, ui.ErrorWriter.String(), "No feature gates found")
	})

	t.Run("api error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"Errors":["unavailable"]}`, http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		ui := cli.NewMockUi()
		code := New(ui).Run([]string{"-http-addr=" + srv.URL})
		require.Equal(t, 1, code)
		require.Contains(t, ui.ErrorWriter.String(), "Error querying feature gates")
	})

	t.Run("rejects positional arguments", func(t *testing.T) {
		ui := cli.NewMockUi()
		require.Equal(t, 1, New(ui).Run([]string{"unexpected"}))
		require.Contains(t, ui.ErrorWriter.String(), "takes no positional arguments")
	})
}

func TestCmd_HelpSynopsis(t *testing.T) {
	c := New(cli.NewMockUi())
	require.NotEmpty(t, c.Help())
	require.NotEmpty(t, c.Synopsis())
	require.NotContains(t, c.Help(), "\t")
}
