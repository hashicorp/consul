package auto

import "testing"

func TestRewriteToExpand(t *testing.T) {
	tests := []struct {
		in       string
		expected string
	}{
		{in: "", expected: ""},
		{in: "{1}", expected: "${1}"},
		{in: "{1", expected: "${1"},
	}
	for i, tc := range tests {
		got := rewriteToExpand(tc.in)
		if got != tc.expected {
			t.Errorf("Test %d: Expected error %v, but got %v", i, tc.expected, got)
		}
	}
}
