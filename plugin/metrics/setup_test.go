package metrics

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestPrometheusParse(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		addr      string
	}{
		// oks
		{`prometheus`, false, "localhost:9153"},
		{`prometheus localhost:53`, false, "localhost:53"},
		// fails
		{`prometheus {}`, true, ""},
		{`prometheus /foo`, true, ""},
		{`prometheus a b c`, true, ""},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		m, err := prometheusParse(c)
		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}

		if test.shouldErr {
			continue
		}

		if test.addr != m.Addr {
			t.Errorf("Test %v: Expected address %s but found: %s", i, test.addr, m.Addr)
		}
	}
}
