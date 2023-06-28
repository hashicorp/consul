// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ca

import (
	"bytes"
	"testing"
	"time"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestStructs_CAConfiguration_MsgpackEncodeDecode(t *testing.T) {
	type testcase struct {
		in           *structs.CAConfiguration
		expectConfig interface{} // provider specific
		parseFunc    func(*testing.T, map[string]interface{}) interface{}
	}

	commonBaseMap := map[string]interface{}{
		"LeafCertTTL":      "30h",
		"SkipValidate":     true,
		"CSRMaxPerSecond":  5.25,
		"CSRMaxConcurrent": int64(55),
		"PrivateKeyType":   "rsa",
		"PrivateKeyBits":   int64(4096),
	}
	expectCommonBase := &structs.CommonCAProviderConfig{
		LeafCertTTL:         30 * time.Hour,
		IntermediateCertTTL: 90 * time.Hour,
		SkipValidate:        true,
		CSRMaxPerSecond:     5.25,
		CSRMaxConcurrent:    55,
		PrivateKeyType:      "rsa",
		PrivateKeyBits:      4096,
		RootCertTTL:         10 * 24 * 365 * time.Hour,
	}

	cases := map[string]testcase{
		structs.ConsulCAProvider: {
			in: &structs.CAConfiguration{
				ClusterID: "abc",
				Provider:  structs.ConsulCAProvider,
				State: map[string]string{
					"foo": "bar",
				},
				ForceWithoutCrossSigning: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 99,
				},
				Config: map[string]interface{}{
					"PrivateKey":          "key",
					"RootCert":            "cert",
					"RotationPeriod":      "5m", // old unused field
					"IntermediateCertTTL": "90h",
					"DisableCrossSigning": true,
				},
			},
			expectConfig: &structs.ConsulCAProviderConfig{
				CommonCAProviderConfig: *expectCommonBase,
				PrivateKey:             "key",
				RootCert:               "cert",
				DisableCrossSigning:    true,
			},
			parseFunc: func(t *testing.T, raw map[string]interface{}) interface{} {
				config, err := ParseConsulCAConfig(raw)
				require.NoError(t, err)
				return config
			},
		},
		structs.VaultCAProvider: {
			in: &structs.CAConfiguration{
				ClusterID: "abc",
				Provider:  structs.VaultCAProvider,
				State: map[string]string{
					"foo": "bar",
				},
				ForceWithoutCrossSigning: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 99,
				},
				Config: map[string]interface{}{
					"Address":             "addr",
					"Token":               "token",
					"RootPKIPath":         "root-pki/",
					"IntermediatePKIPath": "im-pki/",
					"IntermediateCertTTL": "90h",
					"CAFile":              "ca-file",
					"CAPath":              "ca-path",
					"CertFile":            "cert-file",
					"KeyFile":             "key-file",
					"TLSServerName":       "server-name",
					"TLSSkipVerify":       true,
				},
			},
			expectConfig: &structs.VaultCAProviderConfig{
				CommonCAProviderConfig: *expectCommonBase,
				Address:                "addr",
				Token:                  "token",
				RootPKIPath:            "root-pki/",
				IntermediatePKIPath:    "im-pki/",
				CAFile:                 "ca-file",
				CAPath:                 "ca-path",
				CertFile:               "cert-file",
				KeyFile:                "key-file",
				TLSServerName:          "server-name",
				TLSSkipVerify:          true,
			},
			parseFunc: func(t *testing.T, raw map[string]interface{}) interface{} {
				config, err := ParseVaultCAConfig(raw, true)
				require.NoError(t, err)
				return config
			},
		},
		structs.AWSCAProvider: {
			in: &structs.CAConfiguration{
				ClusterID: "abc",
				Provider:  structs.AWSCAProvider,
				State: map[string]string{
					"foo": "bar",
				},
				ForceWithoutCrossSigning: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 99,
				},
				Config: map[string]interface{}{
					"ExistingARN":         "arn://foo",
					"DeleteOnExit":        true,
					"IntermediateCertTTL": "90h",
				},
			},
			expectConfig: &structs.AWSCAProviderConfig{
				CommonCAProviderConfig: *expectCommonBase,
				ExistingARN:            "arn://foo",
				DeleteOnExit:           true,
			},
			parseFunc: func(t *testing.T, raw map[string]interface{}) interface{} {
				config, err := ParseAWSCAConfig(raw)
				require.NoError(t, err)
				return config
			},
		},
	}
	// underlay common ca config stuff
	for _, tc := range cases {
		for k, v := range commonBaseMap {
			if _, ok := tc.in.Config[k]; !ok {
				tc.in.Config[k] = v
			}
		}
	}

	var (
		// This is the common configuration pre-1.7.0
		handle1 = structs.TestingOldPre1dot7MsgpackHandle
		// This is the common configuration post-1.7.0
		handle2 = structs.MsgpackHandle
	)

	decoderCase := func(t *testing.T, tc testcase, encHandle, decHandle *codec.MsgpackHandle) {
		t.Helper()

		var buf bytes.Buffer
		enc := codec.NewEncoder(&buf, encHandle)
		require.NoError(t, enc.Encode(tc.in))

		out := &structs.CAConfiguration{}
		dec := codec.NewDecoder(&buf, decHandle)
		require.NoError(t, dec.Decode(out))

		config := tc.parseFunc(t, out.Config)

		out.Config = tc.in.Config // no longer care about how this field decoded
		require.Equal(t, tc.in, out)
		require.Equal(t, tc.expectConfig, config)
		// TODO: verify json?
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Run("old encoder and old decoder", func(t *testing.T) {
				decoderCase(t, tc, handle1, handle1)
			})
			t.Run("old encoder and new decoder", func(t *testing.T) {
				decoderCase(t, tc, handle1, handle2)
			})
			t.Run("new encoder and old decoder", func(t *testing.T) {
				decoderCase(t, tc, handle2, handle1)
			})
			t.Run("new encoder and new decoder", func(t *testing.T) {
				decoderCase(t, tc, handle2, handle2)
			})
		})
	}
}
