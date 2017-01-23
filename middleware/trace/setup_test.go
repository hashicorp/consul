package trace

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestTraceParse(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		endpoint      string
	}{
		// oks
		{`trace`, false, "http://localhost:9411/api/v1/spans"},
		{`trace localhost:1234`, false, "http://localhost:1234/api/v1/spans"},
		{`trace http://localhost:1234/somewhere/else`, false, "http://localhost:1234/somewhere/else"},
		{`trace zipkin localhost:1234`, false, "http://localhost:1234/api/v1/spans"},
		{`trace zipkin http://localhost:1234/somewhere/else`, false, "http://localhost:1234/somewhere/else"},
		// fails
		{`trace footype localhost:4321`, true, ""},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		m, err := traceParse(c)
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

		if test.endpoint != m.Endpoint {
			t.Errorf("Test %v: Expected endpoint %s but found: %s", i, test.endpoint, m.Endpoint)
		}
	}
}
