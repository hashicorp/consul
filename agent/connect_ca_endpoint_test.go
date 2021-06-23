package agent

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"io/ioutil"
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	r := require.New(t)
	a := NewTestAgent(t, "connect { enabled = false }")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.ConnectCARoots(resp, req)
	r.Error(err)
	r.Contains(err.Error(), "Connect must be enabled")
}

func TestConnectCARoots_list(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	assertion := assert.New(t)
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
	assertion.NoError(err)

	value := obj.(structs.IndexedCARoots)
	assertion.Equal(value.ActiveRootID, ca2.ID)
	assertion.Len(value.Roots, 2)

	// We should never have the secret information
	for _, r := range value.Roots {
		assertion.Equal("", r.SigningCert)
		assertion.Equal("", r.SigningKey)
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
					"RotationPeriod": "1h",
					"IntermediateCertTTL": "288h"
				}
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"RotationPeriod":      "1h",
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
					"RotationPeriod": "1h",
					"IntermediateCertTTL": "288h"
				}
			}`,
			wantErr: false,
			wantCfg: structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: connect.TestClusterID,
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"RotationPeriod":      "1h",
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
					"RotationPeriod": "1h",
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
					"RotationPeriod":      "1h",
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
					"RotationPeriod": "1h",
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
					"RotationPeriod":      "1h",
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
					"RotationPeriod": "1h",
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
					"RotationPeriod":      "1h",
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
			r := require.New(t)
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
					r.Error(err)
					return
				}
				r.NoError(err)
			}
			// The config should be updated now.
			{
				req, _ := http.NewRequest("GET", "/v1/connect/ca/configuration", nil)
				resp := httptest.NewRecorder()
				obj, err := a.srv.ConnectCAConfiguration(resp, req)
				r.NoError(err)

				got := obj.(structs.CAConfiguration)
				// Reset Raft indexes to make it non flaky
				got.CreateIndex = 0
				got.ModifyIndex = 0
				r.Equal(tc.wantCfg, got)
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

	data, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(data))
	// expecting the root cert from dc1 and an intermediate in dc2
	require.Len(t, pool.Subjects(), 2)
}

func Test_writeCA(t *testing.T) {
	type args struct {
		roots []*structs.CARoot
	}
	const ca = "-----BEGIN CERTIFICATE-----\nMIIB3TCCAYKgAwIBAgIIHk4Xdb5VAukwCgYIKoZIzj0EAwIwFDESMBAGA1UEAxMJ\nVGVzdCBDQSAxMB4XDTIxMDYxNjE1MjExOFoXDTMxMDYxNjE1MjExOFowFDESMBAG\nA1UEAxMJVGVzdCBDQSAxMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEQ268w92S\n3z1FUBjDA85hYLAFvHWEj1SyhByG5xH1i0Agbj3MQDEYEWvKpLa1IWAi6thowmg+\npAvOjKlFrdN3saOBvTCBujAOBgNVHQ8BAf8EBAMCAYYwDwYDVR0TAQH/BAUwAwEB\n/zApBgNVHQ4EIgQg1GebhqO6UZ0UDbFS4J3T4fZLu1fpOlQq+GAzjG48mJIwKwYD\nVR0jBCQwIoAg1GebhqO6UZ0UDbFS4J3T4fZLu1fpOlQq+GAzjG48mJIwPwYDVR0R\nBDgwNoY0c3BpZmZlOi8vMTExMTExMTEtMjIyMi0zMzMzLTQ0NDQtNTU1NTU1NTU1\nNTU1LmNvbnN1bDAKBggqhkjOPQQDAgNJADBGAiEAhPvUug/B5NhEFZzivsKt5Xtr\ncEyoASTqwCnKCWdNy3cCIQDgX3Stt+VlgaT4YcpLbl/GzezMVgt9F3Z6dKKHf0E9\ntA==\n-----END CERTIFICATE-----"
	const caWithNewLine = ca + "\n"
	expected := fmt.Sprintf("%s\n%s\n%s\n", ca, ca, ca)
	expectedRoot := fmt.Sprintf("%s\n", ca)
	tests := []struct {
		name     string
		args     args
		wantResp string
		wantErr  bool
	}{
		{"empty", args{roots: []*structs.CARoot{}}, "", false},
		{"root + 2 intermediates with newline", args{roots: []*structs.CARoot{{RootCert: caWithNewLine, IntermediateCerts: []string{caWithNewLine, caWithNewLine}}}}, expected, false},
		{"root + 2 intermediates no newline", args{roots: []*structs.CARoot{{RootCert: ca, IntermediateCerts: []string{ca, ca}}}}, expected, false},
		{"root + 2 intermediates mixed", args{roots: []*structs.CARoot{{RootCert: caWithNewLine, IntermediateCerts: []string{caWithNewLine, ca}}}}, expected, false},
		{"root only", args{roots: []*structs.CARoot{{RootCert: caWithNewLine}}}, expectedRoot, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &bytes.Buffer{}
			err := writeCA(resp, tt.args.roots)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeCA() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotResp := resp.String(); gotResp != tt.wantResp {
				t.Errorf("writeCA() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}
