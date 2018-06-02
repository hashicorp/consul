package dnstap

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		file   string
		path   string
		full   bool
		socket bool
		fail   bool
	}{
		{"dnstap dnstap.sock full", "dnstap.sock", true, true, false},
		{"dnstap unix://dnstap.sock", "dnstap.sock", false, true, false},
		{"dnstap tcp://127.0.0.1:6000", "127.0.0.1:6000", false, false, false},
		{"dnstap", "fail", false, true, true},
	}
	for _, c := range tests {
		cad := caddy.NewTestController("dns", c.file)
		conf, err := parseConfig(&cad.Dispenser)
		if c.fail {
			if err == nil {
				t.Errorf("%s: %s", c.file, err)
			}
		} else if err != nil || conf.target != c.path ||
			conf.full != c.full || conf.socket != c.socket {

			t.Errorf("Expected: %+v\nhave: %+v\nerror: %s\n", c, conf, err)
		}
	}
}
