package lib

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func makeFullTelemetryConfig(t *testing.T) TelemetryConfig {
	var (
		strSliceVal = []string{"foo"}
		strVal      = "foo"
		intVal      = int64(1 * time.Second)
	)

	cfg := TelemetryConfig{}
	cfgP := reflect.ValueOf(&cfg)
	cfgV := cfgP.Elem()
	for i := 0; i < cfgV.NumField(); i++ {
		f := cfgV.Field(i)
		if !f.IsValid() || !f.CanSet() {
			continue
		}
		// Set non-zero values for all fields. We only implement kinds that exist
		// now for brevity but will fail the test if a new field type is added since
		// this is likely not implemented in MergeDefaults either.
		switch f.Kind() {
		case reflect.Slice:
			if f.Type() != reflect.TypeOf(strSliceVal) {
				t.Fatalf("unknown slice type in TelemetryConfig." +
					" You need to update MergeDefaults and this test code.")
			}
			f.Set(reflect.ValueOf(strSliceVal))
		case reflect.Int, reflect.Int64: // time.Duration == int64
			f.SetInt(intVal)
		case reflect.String:
			f.SetString(strVal)
		case reflect.Bool:
			f.SetBool(true)
		default:
			t.Fatalf("unknown field type in TelemetryConfig" +
				" You need to update MergeDefaults and this test code.")
		}
	}
	return cfg
}

func TestTelemetryConfig_MergeDefaults(t *testing.T) {
	tests := []struct {
		name     string
		cfg      TelemetryConfig
		defaults TelemetryConfig
		want     TelemetryConfig
	}{
		{
			name: "basic merge",
			cfg: TelemetryConfig{
				StatsiteAddr: "stats.it:4321",
			},
			defaults: TelemetryConfig{
				StatsdAddr:   "localhost:5678",
				StatsiteAddr: "localhost:1234",
			},
			want: TelemetryConfig{
				StatsdAddr:   "localhost:5678",
				StatsiteAddr: "stats.it:4321",
			},
		},
		{
			// This test uses reflect to build a TelemetryConfig with every value set
			// to ensure that we exercise every possible field type. This means that
			// if new fields are added that are not supported types in the code, this
			// test should either ensure they work or fail to build the test case and
			// fail the test.
			name:     "exhaustive",
			cfg:      TelemetryConfig{},
			defaults: makeFullTelemetryConfig(t),
			want:     makeFullTelemetryConfig(t),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.cfg
			c.MergeDefaults(&tt.defaults)
			require.Equal(t, tt.want, c)
		})
	}
}
