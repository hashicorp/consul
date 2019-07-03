package errors

import (
	"testing"

	"github.com/caddyserver/caddy"
)

func TestErrorsParse(t *testing.T) {
	tests := []struct {
		inputErrorsRules string
		shouldErr        bool
		optCount         int
	}{
		{`errors`, false, 0},
		{`errors stdout`, false, 0},
		{`errors errors.txt`, true, 0},
		{`errors visible`, true, 0},
		{`errors { log visible }`, true, 0},
		{`errors
		  errors `, true, 0},
		{`errors a b`, true, 0},

		{`errors {
		    consolidate
		  }`, true, 0},
		{`errors {
		    consolidate 1m
		  }`, true, 0},
		{`errors {
		    consolidate 1m .* extra
		  }`, true, 0},
		{`errors {
		    consolidate abc .*
		  }`, true, 0},
		{`errors {
		    consolidate 1 .*
		  }`, true, 0},
		{`errors {
		    consolidate 1m ())
		  }`, true, 0},
		{`errors {
		    consolidate 1m ^exact$
		  }`, false, 1},
		{`errors {
		    consolidate 1m error
		  }`, false, 1},
		{`errors {
		    consolidate 1m "format error"
		  }`, false, 1},
		{`errors {
		    consolidate 1m error1
		    consolidate 5s error2
		  }`, false, 2},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputErrorsRules)
		h, err := errorsParse(c)

		if err == nil && test.shouldErr {
			t.Errorf("Test %d didn't error, but it should have", i)
		} else if err != nil && !test.shouldErr {
			t.Errorf("Test %d errored, but it shouldn't have; got '%v'", i, err)
		} else if h != nil && len(h.patterns) != test.optCount {
			t.Errorf("Test %d: pattern count mismatch, expected %d, got %d",
				i, test.optCount, len(h.patterns))
		}
	}
}
