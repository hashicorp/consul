package metrics

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestPrometheus(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
	}{
		{`prometheus`, false},
		{`prometheus {}`, false},   // TODO(miek): should be true
		{`prometheus /foo`, false}, // TODO(miek): should be true
		{`prometheus localhost:53`, false},
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
