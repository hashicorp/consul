package agent

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/testrpc"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

func TestConnectCARoots_empty(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "connect { enabled = false }")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.ConnectCARoots(resp, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Connect must be enabled")
}

func TestConnectCARoots_list(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Set some CAs. Note that NewTestAgent already bootstraps one CA so this just
	// adds a second and makes it active.
	ca2 := connect.TestCAConfigSet(t, a, nil)

	// List
	req, _ := http.NewRequest("GET", "/v1/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.ConnectCARoots(resp, req)
	assert.NoError(t, err)

	value := obj.(structs.IndexedCARoots)
	assert.Equal(t, value.ActiveRootID, ca2.ID)
	assert.Len(t, value.Roots, 2)

	// We should never have the secret information
	for _, r := range value.Roots {
		assert.Equal(t, "", r.SigningCert)
		assert.Equal(t, "", r.SigningKey)
	}
}

func TestConnectCAConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
					"IntermediateCertTTL": "288h"
				}
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "288h",
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
					"IntermediateCertTTL": "288h"
				}
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "288h",
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
					"IntermediateCertTTL": "288h"
				},
				"ForceWithoutCrossSigning": true
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "288h",
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
					"IntermediateCertTTL": "288h"
				},
				"force_without_cross_signing": true
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "288h",
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
					"IntermediateCertTTL": "288h"
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
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "288h",
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
			hcl := ""
			if tc.initialState != "" {
				hcl = `
				connect {
					enabled = true
					ca_provider = "consul"
					ca_config {
						intermediate_cert_ttl = "288h"
						test_state {
							` + tc.initialState + `
						}
					}
				}`
			}
			a := NewTestAgent(t, hcl)
			defer a.Shutdown()
			testrpc.WaitForTestAgent(t, a.RPC, "dc1")

			// Set the config.
			{
				body := bytes.NewBuffer([]byte(tc.body))
				req, _ := http.NewRequest("PUT", "/v1/connect/ca/configuration", body)
				resp := httptest.NewRecorder()
				_, err := a.srv.ConnectCAConfiguration(resp, req)
				if tc.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
			}
			// The config should be updated now.
			{
				req, _ := http.NewRequest("GET", "/v1/connect/ca/configuration", nil)
				resp := httptest.NewRecorder()
				obj, err := a.srv.ConnectCAConfiguration(resp, req)
				require.NoError(t, err)

				got := obj.(structs.CAConfiguration)
				// Reset Raft indexes to make it non flaky
				got.CreateIndex = 0
				got.ModifyIndex = 0
				require.Equal(t, tc.wantCfg, got)
			}
		})
	}
}

func TestConnectCARoots_PEMEncoding(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	primary := NewTestAgent(t, "")
	defer primary.Shutdown()
	testrpc.WaitForActiveCARoot(t, primary.RPC, "dc1", nil)

	secondary := NewTestAgent(t, `
		primary_datacenter = "dc1"
		datacenter = "dc2"
		retry_join_wan = ["`+primary.Config.SerfBindAddrWAN.String()+`"]
	`)
	defer secondary.Shutdown()
	testrpc.WaitForActiveCARoot(t, secondary.RPC, "dc2", nil)

	req, _ := http.NewRequest("GET", "/v1/connect/ca/roots?pem=true", nil)
	recorder := httptest.NewRecorder()
	obj, err := secondary.srv.ConnectCARoots(recorder, req)
	require.NoError(t, err)
	require.Nil(t, obj, "Endpoint returned an object for serialization when it should have returned nil and written to the responses")
	resp := recorder.Result()
	require.Equal(t, resp.Header.Get("Content-Type"), "application/pem-certificate-chain")

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// expecting the root cert from dc1 and an intermediate in dc2
	block, rest := pem.Decode(data)
	_, err = x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	block, _ = pem.Decode(rest)
	_, err = x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
}
