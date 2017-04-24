package pprof

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestPProf(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
	}{
		{`pprof`, false},
		{`pprof 1.2.3.4:1234`, false},
		{`pprof :1234`, false},
		{`pprof {}`, true},
		{`pprof /foo`, true},
		{`pprof {
            a b
        }`, true},
		{`pprof
          pprof`, true},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		err := setup(c)
		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
		}
	}
}
