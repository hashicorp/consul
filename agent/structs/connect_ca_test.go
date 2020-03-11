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
					"RotationPeriod":  "2160h",
					"LeafCertTTL":     "72h",
					"CSRMaxPerSecond": "50",
				},
			},
			want: &CommonCAProviderConfig{
				LeafCertTTL:     72 * time.Hour,
				CSRMaxPerSecond: 50,
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
					"RotationPeriod": []uint8("2160h"),
					"LeafCertTTL":    []uint8("72h"),
				},
			},
			want: &CommonCAProviderConfig{
				LeafCertTTL:     72 * time.Hour,
				CSRMaxPerSecond: 50, // The default value
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
		cfg     *ConsulCAProviderConfig
		wantErr bool
		wantMsg string
	}{
		{
			name:    "defaults",
			cfg:     &ConsulCAProviderConfig{},
			wantErr: true,
			wantMsg: "Intermediate Cert TTL must be greater or equal than 3h",
		},
		{
			name: "intermediate cert ttl too short",
			cfg: &ConsulCAProviderConfig{
				CommonCAProviderConfig: CommonCAProviderConfig{LeafCertTTL: 2 * time.Hour},
				IntermediateCertTTL:    4 * time.Hour,
			},
			wantErr: true,
			wantMsg: "Intermediate Cert TTL must be greater or equal than 3 * LeafCertTTL (>=6h0m0s).",
		},
		{
			name: "intermediate cert ttl too short",
			cfg: &ConsulCAProviderConfig{
				CommonCAProviderConfig: CommonCAProviderConfig{LeafCertTTL: 5 * time.Hour},
				IntermediateCertTTL:    15*time.Hour - 1,
			},
			wantErr: true,
			wantMsg: "Intermediate Cert TTL must be greater or equal than 3 * LeafCertTTL (>=15h0m0s).",
		},
		{
			name: "good intermediate and leaf cert TTL",
			cfg: &ConsulCAProviderConfig{
				CommonCAProviderConfig: CommonCAProviderConfig{LeafCertTTL: 1 * time.Hour},
				IntermediateCertTTL:    4 * time.Hour,
			},
			wantErr: false,
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
