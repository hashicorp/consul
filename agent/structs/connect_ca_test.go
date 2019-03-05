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
