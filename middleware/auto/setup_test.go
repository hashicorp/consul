package auto

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestAutoParse(t *testing.T) {
	tests := []struct {
		inputFileRules    string
		shouldErr         bool
		expectedDirectory string
		expectedTempl     string
		expectedRe        string
		expectedTo        string
	}{
		{
			`auto example.org {
				directory /tmp
				transfer to 127.0.0.1
			}`,
			false, "/tmp", "${1}", `db\.(.*)`, "127.0.0.1:53",
		},
		{
			`auto {
				directory /tmp
			}`,
			false, "/tmp", "${1}", `db\.(.*)`, "",
		},
		{
			`auto {
				directory /tmp (.*) bliep
			}`,
			false, "/tmp", "bliep", `(.*)`, "",
		},
		// errors
		{
			`auto example.org {
				directory
			}`,
			true, "", "${1}", `db\.(.*)`, "",
		},
		{
			`auto example.org {
				directory /tmp * {1}
			}`,
			true, "", "${1}", ``, "",
		},
		{
			`auto example.org {
				directory /tmp .* {1}
			}`,
			true, "", "${1}", ``, "",
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputFileRules)
		a, err := autoParse(c)

		if err == nil && test.shouldErr {
			t.Fatalf("Test %d expected errors, but got no error", i)
		} else if err != nil && !test.shouldErr {
			t.Fatalf("Test %d expected no errors, but got '%v'", i, err)
		} else if !test.shouldErr {
			if a.loader.directory != test.expectedDirectory {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedDirectory, a.loader.directory)
			}
			if a.loader.template != test.expectedTempl {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedTempl, a.loader.template)
			}
			if a.loader.re.String() != test.expectedRe {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedRe, a.loader.re)
			}
			if test.expectedTo != "" && a.loader.transferTo[0] != test.expectedTo {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedTo, a.loader.transferTo[0])
			}
		}
	}
}
