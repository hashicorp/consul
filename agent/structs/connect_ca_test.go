// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCAConfiguration_GetCommonConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *CAConfiguration
		want    *CommonCAProviderConfig
		wantErr bool
	}{
		{
			name: "basic defaults",
			cfg: &CAConfiguration{
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "4320h",
					"CSRMaxPerSecond":     "50",
				},
			},
			want: &CommonCAProviderConfig{
				LeafCertTTL:         72 * time.Hour,
				IntermediateCertTTL: 4320 * time.Hour,
				CSRMaxPerSecond:     50,
			},
		},
		{
			// Note that this is currently what is actually stored in MemDB, I think
			// due to a trip through msgpack somewhere but I'm not really sure why
			// since the defaults are applied on the server and so should probably use
			// direct RPC that bypasses encoding? Either way this case is important
			// because it reflects the actual data as it's stored in state which is
			// what matters in real life.
			name: "basic defaults after encoding fun",
			cfg: &CAConfiguration{
				Config: map[string]interface{}{
					"LeafCertTTL":         []uint8("72h"),
					"IntermediateCertTTL": []uint8("4320h"),
				},
			},
			want: &CommonCAProviderConfig{
				LeafCertTTL:         72 * time.Hour,
				IntermediateCertTTL: 4320 * time.Hour,
				CSRMaxPerSecond:     50, // The default value
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.GetCommonConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("CAConfiguration.GetCommonConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCAProviderConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *CommonCAProviderConfig
		wantErr bool
		wantMsg string
	}{
		{
			name:    "defaults",
			cfg:     &CommonCAProviderConfig{},
			wantErr: true,
			wantMsg: "leaf cert TTL must be greater or equal than 1h0m0s",
		},
		{
			name: "intermediate cert ttl too short",
			cfg: &CommonCAProviderConfig{
				LeafCertTTL:         2 * time.Hour,
				IntermediateCertTTL: 4 * time.Hour,
				RootCertTTL:         5 * time.Hour,
			},
			wantErr: true,
			wantMsg: "Intermediate Cert TTL must be greater or equal than 3 * LeafCertTTL (>=6h0m0s).",
		},
		{
			name: "intermediate cert ttl too short",
			cfg: &CommonCAProviderConfig{
				LeafCertTTL:         5 * time.Hour,
				IntermediateCertTTL: 15*time.Hour - 1,
				RootCertTTL:         15 * time.Hour,
			},
			wantErr: true,
			wantMsg: "Intermediate Cert TTL must be greater or equal than 3 * LeafCertTTL (>=15h0m0s).",
		},
		{
			name: "good intermediate and leaf cert TTL, missing key type",
			cfg: &CommonCAProviderConfig{
				LeafCertTTL:         1 * time.Hour,
				IntermediateCertTTL: 4 * time.Hour,
				RootCertTTL:         5 * time.Hour,
			},
			wantErr: true,
			wantMsg: "private key type must be either 'ec' or 'rsa'",
		},
		{
			name: "good intermediate/leaf cert TTL/key type, missing bits",
			cfg: &CommonCAProviderConfig{
				LeafCertTTL:         1 * time.Hour,
				IntermediateCertTTL: 4 * time.Hour,
				RootCertTTL:         5 * time.Hour,
				PrivateKeyType:      "ec",
			},
			wantErr: true,
			wantMsg: "EC key length must be one of (224, 256, 384, 521) bits",
		},
		{
			name: "good intermediate/leaf cert TTL/key type/bits",
			cfg: &CommonCAProviderConfig{
				LeafCertTTL:         1 * time.Hour,
				IntermediateCertTTL: 4 * time.Hour,
				RootCertTTL:         5 * time.Hour,
				PrivateKeyType:      "ec",
				PrivateKeyBits:      256,
			},
			wantErr: false,
		},
		{
			name: "good root cert/ intermediate TTLs",
			cfg: &CommonCAProviderConfig{
				LeafCertTTL:         1 * time.Hour,
				IntermediateCertTTL: 4 * time.Hour,
				RootCertTTL:         5 * time.Hour,
				PrivateKeyType:      "ec",
				PrivateKeyBits:      256,
			},
			wantErr: false,
			wantMsg: "",
		},
		{
			name: "bad root cert/ intermediate TTLs",
			cfg: &CommonCAProviderConfig{
				LeafCertTTL:         1 * time.Hour,
				IntermediateCertTTL: 4 * time.Hour,
				RootCertTTL:         3 * time.Hour,
				PrivateKeyType:      "ec",
				PrivateKeyBits:      256,
			},
			wantErr: true,
			wantMsg: "root cert TTL is set and is not greater than intermediate cert ttl. root cert ttl: 3h0m0s, intermediate cert ttl: 4h0m0s",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				require.False(t, tt.wantErr)
				return
			}
			require.Equal(t, err.Error(), tt.wantMsg)
		})
	}
}
