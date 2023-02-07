package config

import (
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAddFlags_WithParse tests whether command line flags are properly parsed
// into the Flags/File structure. It contains an example for every type
// that is parsed. It does not test the conversion into the final
// runtime configuration. See TestConfig for that.
func TestAddFlags_WithParse(t *testing.T) {
	tests := []struct {
		args     []string
		expected LoadOpts
		extra    []string
	}{
		{},
		{
			args:     []string{`-bind`, `a`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{BindAddr: pString("a")}}},
		},
		{
			args:     []string{`-bootstrap`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Bootstrap: pBool(true)}}},
		},
		{
			args:     []string{`-bootstrap=true`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Bootstrap: pBool(true)}}},
		},
		{
			args:     []string{`-bootstrap=false`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Bootstrap: pBool(false)}}},
		},
		{
			args:     []string{`-config-file`, `a`, `-config-dir`, `b`, `-config-file`, `c`, `-config-dir`, `d`},
			expected: LoadOpts{ConfigFiles: []string{"a", "b", "c", "d"}},
		},
		{
			args:     []string{`-datacenter`, `a`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Datacenter: pString("a")}}},
		},
		{
			args:     []string{`-dns-port`, `1`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Ports: Ports{DNS: pInt(1)}}}},
		},
		{
			args:     []string{`-grpc-port`, `1`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Ports: Ports{GRPC: pInt(1)}}}},
		},
		{
			args:     []string{`-http-port`, `1`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Ports: Ports{HTTP: pInt(1)}}}},
		},
		{
			args:     []string{`-https-port`, `1`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Ports: Ports{HTTPS: pInt(1)}}}},
		},
		{
			args:     []string{`-serf-lan-port`, `1`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Ports: Ports{SerfLAN: pInt(1)}}}},
		},
		{
			args:     []string{`-serf-wan-port`, `1`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Ports: Ports{SerfWAN: pInt(1)}}}},
		},
		{
			args:     []string{`-server-port`, `1`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Ports: Ports{Server: pInt(1)}}}},
		},
		{
			args:     []string{`-join`, `a`, `-join`, `b`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{DeprecatedConfig: DeprecatedConfig{StartJoinAddrsLAN: []string{"a", "b"}}}},
		},
		{
			args:     []string{`-node-meta`, `a:b`, `-node-meta`, `c:d`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{NodeMeta: map[string]string{"a": "b", "c": "d"}}}},
		},
		{
			args:     []string{`-bootstrap`, `true`},
			expected: LoadOpts{FlagValues: FlagValuesTarget{Config: Config{Bootstrap: pBool(true)}}},
			extra:    []string{"true"},
		},
		{
			args: []string{`-primary-gateway`, `foo.local`, `-primary-gateway`, `bar.local`},
			expected: LoadOpts{
				FlagValues: FlagValuesTarget{
					Config: Config{
						PrimaryGateways: []string{"foo.local", "bar.local"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			flags := LoadOpts{}
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			AddFlags(fs, &flags)

			err := fs.Parse(tt.args)
			require.NoError(t, err)

			// Normalize the expected value because require.Equal considers
			// empty slices/maps and nil slices/maps to be different.
			if tt.extra == nil && fs.Args() != nil {
				tt.extra = []string{}
			}
			if len(tt.expected.FlagValues.NodeMeta) == 0 {
				tt.expected.FlagValues.NodeMeta = map[string]string{}
			}
			require.Equal(t, tt.extra, fs.Args())
			require.Equal(t, tt.expected, flags)
		})
	}
}
