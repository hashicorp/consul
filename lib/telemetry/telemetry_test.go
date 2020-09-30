package telemetry

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func makeFullTelemetryConfig(t *testing.T) Config {
	var (
		strSliceVal = []string{"foo"}
		strVal      = "foo"
		intVal      = int64(1 * time.Second)
	)

	cfg := Config{}
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
				t.Fatalf("unknown slice type in Config." +
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
			t.Fatalf("unknown field type in Config" +
				" You need to update MergeDefaults and this test code.")
		}
	}
	return cfg
}

// note(kit): We should able to check for the presence of specific sinks, but the fact that we hard error
//  out if any sinks fail to build means that require.NoError() is sufficient... even if it feels imprecise.
func TestTelemetry_Sinks(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "Inmem only",
			cfg:  Config{},
		},
		{
			name: "statsite",
			cfg: Config{
				StatsiteAddr: "localhost:9999",
			},
		},
		{
			name: "statsd",
			cfg: Config{
				StatsdAddr: "localhost:9999",
			},
		},
		{
			name: "dogstatsd",
			cfg: Config{
				DogstatsdAddr: "localhost:9999",
				DogstatsdTags: []string{"how", "now", "brown", "cow"},
			},
		},
		{
			name: "Prometheus",
			cfg: Config{
				PrometheusRetentionTime: 50,
			},
		},
		{
			name: "Circonus",
			cfg: Config{
				CirconusAPIApp:                     "",
				CirconusAPIToken:                   "test",
				CirconusAPIURL:                     "localhost:9999",
				CirconusBrokerID:                   "",
				CirconusBrokerSelectTag:            "select",
				CirconusCheckDisplayName:           "name",
				CirconusCheckForceMetricActivation: "",
				CirconusCheckID:                    "",
				CirconusCheckInstanceID:            "BAZU-999U",
				CirconusCheckSearchTag:             "foop",
				CirconusCheckTags:                  "barp",
				CirconusSubmissionInterval:         "30s",
				CirconusSubmissionURL:              "localhost:5555",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Init(tt.cfg)
			require.NoError(t, err)
		})
	}
}

func TestTelemetryConfig_MergeDefaults(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		defaults Config
		want     Config
	}{
		{
			name: "basic merge",
			cfg: Config{
				StatsiteAddr: "stats.it:4321",
			},
			defaults: Config{
				StatsdAddr:   "localhost:5678",
				StatsiteAddr: "localhost:1234",
			},
			want: Config{
				StatsdAddr:   "localhost:5678",
				StatsiteAddr: "stats.it:4321",
			},
		},
		{
			// This test uses reflect to build a Config with every value set
			// to ensure that we exercise every possible field type. This means that
			// if new fields are added that are not supported types in the code, this
			// test should either ensure they work or fail to build the test case and
			// fail the test.
			name:     "exhaustive",
			cfg:      Config{},
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
