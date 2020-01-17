package agent

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/testrpc"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
)

func TestConnectCARoots_empty(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t, t.Name(), "connect { enabled = false }")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.ConnectCARoots(resp, req)
	require.Error(err)
	require.Contains(err.Error(), "Connect must be enabled")
}

func TestConnectCARoots_list(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Set some CAs. Note that NewTestAgent already bootstraps one CA so this just
	// adds a second and makes it active.
	ca2 := connect.TestCAConfigSet(t, a, nil)

	// List
	req, _ := http.NewRequest("GET", "/v1/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.ConnectCARoots(resp, req)
	assert.NoError(err)

	value := obj.(structs.IndexedCARoots)
	assert.Equal(value.ActiveRootID, ca2.ID)
	assert.Len(value.Roots, 2)

	// We should never have the secret information
	for _, r := range value.Roots {
		assert.Equal("", r.SigningCert)
		assert.Equal("", r.SigningKey)
	}
}

func TestConnectCAConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		initialState string
		body         string
		wantErr      bool
		wantCfg      structs.CAConfiguration
	}{
		{
			name: "basic",
			body: `
			{
				"Provider": "consul",
				"Config": {
					"LeafCertTTL": "72h",
					"RotationPeriod": "1h"
				}
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":    "72h",
					"RotationPeriod": "1h",
				},
			},
		},
		{
			name: "basic with IntermediateCertTTL",
			body: `
			{
				"Provider": "consul",
				"Config": {
					"LeafCertTTL": "72h",
					"RotationPeriod": "1h",
					"IntermediateCertTTL": "2h"
				}
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"RotationPeriod":      "1h",
					"IntermediateCertTTL": "2h",
				},
			},
		},
		{
			name: "force without cross sign CamelCase",
			body: `
			{
				"Provider": "consul",
				"Config": {
					"LeafCertTTL": "72h",
					"RotationPeriod": "1h"
				},
				"ForceWithoutCrossSigning": true
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":    "72h",
					"RotationPeriod": "1h",
				},
				ForceWithoutCrossSigning: true,
			},
		},
		{
			name: "force without cross sign snake_case",
			// Note that config is still CamelCase. We don't currently support snake
			// case config in the API only in config files for this. Arguably that's a
			// bug but it's unrelated to the force options being tested here so we'll
			// only test the new behaviour here rather than scope creep to refactoring
			// all the CA config handling.
			body: `
			{
				"provider": "consul",
				"config": {
					"LeafCertTTL": "72h",
					"RotationPeriod": "1h"
				},
				"force_without_cross_signing": true
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":    "72h",
					"RotationPeriod": "1h",
				},
				ForceWithoutCrossSigning: true,
			},
		},
		{
			name: "setting state fails",
			body: `
			{
				"Provider": "consul",
				"State": {
					"foo": "bar"
				}
			}`,
			wantErr: true,
		},
		{
			name:         "updating config with same state",
			initialState: `foo = "bar"`,
			body: `
			{
				"Provider": "consul",
				"config": {
					"LeafCertTTL": "72h",
					"RotationPeriod": "1h"
				},
				"State": {
					"foo": "bar"
				}
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":    "72h",
					"RotationPeriod": "1h",
				},
				State: map[string]string{
					"foo": "bar",
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)
			hcl := ""
			if tc.initialState != "" {
				hcl = `
				connect {
					enabled = true
					ca_provider = "consul"
					ca_config {
						test_state {
							` + tc.initialState + `
						}
					}
				}`
			}
			a := NewTestAgent(t, t.Name(), hcl)
			defer a.Shutdown()
			testrpc.WaitForTestAgent(t, a.RPC, "dc1")

			// Set the config.
			{
				body := bytes.NewBuffer([]byte(tc.body))
				req, _ := http.NewRequest("PUT", "/v1/connect/ca/configuration", body)
				resp := httptest.NewRecorder()
				_, err := a.srv.ConnectCAConfiguration(resp, req)
				if tc.wantErr {
					require.Error(err)
					return
				}
				require.NoError(err)
			}
			// The config should be updated now.
			{
				req, _ := http.NewRequest("GET", "/v1/connect/ca/configuration", nil)
				resp := httptest.NewRecorder()
				obj, err := a.srv.ConnectCAConfiguration(resp, req)
				require.NoError(err)

				got := obj.(structs.CAConfiguration)
				// Reset Raft indexes to make it non flaky
				got.CreateIndex = 0
				got.ModifyIndex = 0
				require.Equal(tc.wantCfg, got)
			}
		})
	}
}
