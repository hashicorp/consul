package parse

import (
	"testing"

	"github.com/caddyserver/caddy"
)

func TestTransfer(t *testing.T) {
	tests := []struct {
		inputFileRules string
		shouldErr      bool
		secondary      bool
		expectedTo     []string
		expectedFrom   []string
	}{
		// OK transfer to
		{
			`to 127.0.0.1`,
			false, false, []string{"127.0.0.1:53"}, []string{},
		},
		// OK transfer tos
		{
			`to 127.0.0.1 127.0.0.2`,
			false, false, []string{"127.0.0.1:53", "127.0.0.2:53"}, []string{},
		},
		// OK transfer from
		{
			`from 127.0.0.1`,
			false, true, []string{}, []string{"127.0.0.1:53"},
		},
		// OK transfer froms
		{
			`from 127.0.0.1 127.0.0.2`,
			false, true, []string{}, []string{"127.0.0.1:53", "127.0.0.2:53"},
		},
		// OK transfer tos/froms
		{
			`to 127.0.0.1 127.0.0.2
			from 127.0.0.1 127.0.0.2`,
			false, true, []string{"127.0.0.1:53", "127.0.0.2:53"}, []string{"127.0.0.1:53", "127.0.0.2:53"},
		},
		// Bad transfer from, secondary false
		{
			`from 127.0.0.1`,
			true, false, []string{}, []string{},
		},
		// Bad transfer from garbage
		{
			`from !@#$%^&*()`,
			true, true, []string{}, []string{},
		},
		// Bad transfer from no args
		{
			`from`,
			true, false, []string{}, []string{},
		},
		// Bad transfer from *
		{
			`from *`,
			true, true, []string{}, []string{},
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputFileRules)
		tos, froms, err := Transfer(c, test.secondary)

		if err == nil && test.shouldErr {
			t.Fatalf("Test %d expected errors, but got no error %+v %+v", i, err, test)
		} else if err != nil && !test.shouldErr {
			t.Fatalf("Test %d expected no errors, but got '%v'", i, err)
		}

		if test.expectedTo != nil {
			for j, got := range tos {
				if got != test.expectedTo[j] {
					t.Fatalf("Test %d expected %v, got %v", i, test.expectedTo[j], got)
				}
			}
		}
		if test.expectedFrom != nil {
			for j, got := range froms {
				if got != test.expectedFrom[j] {
					t.Fatalf("Test %d expected %v, got %v", i, test.expectedFrom[j], got)
				}
			}
		}

	}

}
