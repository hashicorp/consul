package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/pascaldekloe/goe/verify"
)

// TestParseFlags tests whether command line flags are properly parsed
// into the Flags/File structure. It contains an example for every type
// that is parsed. It does not test the conversion into the final
// runtime configuration. See TestConfig for that.
func TestParseFlags(t *testing.T) {
	tests := []struct {
		args  []string
		flags Flags
		err   error
	}{
		{},
		{
			args:  []string{`-bind`, `a`},
			flags: Flags{Config: Config{BindAddr: pString("a")}},
		},
		{
			args:  []string{`-bootstrap`},
			flags: Flags{Config: Config{Bootstrap: pBool(true)}},
		},
		{
			args:  []string{`-bootstrap=true`},
			flags: Flags{Config: Config{Bootstrap: pBool(true)}},
		},
		{
			args:  []string{`-bootstrap=false`},
			flags: Flags{Config: Config{Bootstrap: pBool(false)}},
		},
		{
			args:  []string{`-bootstrap`, `true`},
			flags: Flags{Config: Config{Bootstrap: pBool(true)}},
		},
		{
			args:  []string{`-config-file`, `a`, `-config-dir`, `b`, `-config-file`, `c`, `-config-dir`, `d`},
			flags: Flags{ConfigFiles: []string{"a", "b", "c", "d"}},
		},
		{
			args:  []string{`-datacenter`, `a`},
			flags: Flags{Config: Config{Datacenter: pString("a")}},
		},
		{
			args:  []string{`-dns-port`, `1`},
			flags: Flags{Config: Config{Ports: Ports{DNS: pInt(1)}}},
		},
		{
			args:  []string{`-join`, `a`, `-join`, `b`},
			flags: Flags{Config: Config{StartJoinAddrsLAN: []string{"a", "b"}}},
		},
		{
			args:  []string{`-node-meta`, `a:b`, `-node-meta`, `c:d`},
			flags: Flags{Config: Config{NodeMeta: map[string]string{"a": "b", "c": "d"}}},
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			flags, err := ParseFlags(tt.args)
			if got, want := err, tt.err; !reflect.DeepEqual(got, want) {
				t.Fatalf("got error %v want %v", got, want)
			}
			if !verify.Values(t, "flag", flags, tt.flags) {
				t.FailNow()
			}
		})
	}
}
