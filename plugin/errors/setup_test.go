package errors

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestErrorsParse(t *testing.T) {
	tests := []struct {
		inputErrorsRules string
		shouldErr        bool
	}{
		{`errors`, false},
		{`errors stdout`, false},
		{`errors errors.txt`, true},
		{`errors visible`, true},
		{`errors { log visible }`, true},
		{`errors
		errors `, true},
		{`errors a b`, true},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputErrorsRules)
		_, err := errorsParse(c)

		if err == nil && test.shouldErr {
			t.Errorf("Test %d didn't error, but it should have", i)
		} else if err != nil && !test.shouldErr {
			t.Errorf("Test %d errored, but it shouldn't have; got '%v'", i, err)
		}
	}
}
