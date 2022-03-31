package lib

import "testing"

func TestGlobbedStringsMatch(t *testing.T) {
	tests := []struct {
		item   string
		val    string
		expect bool
	}{
		{"", "", true},
		{"*", "*", true},
		{"**", "**", true},
		{"*t", "t", true},
		{"*t", "test", true},
		{"t*", "test", true},
		{"*test", "test", true},
		{"*test", "a test", true},
		{"test", "a test", false},
		{"*test", "tests", false},
		{"test*", "test", true},
		{"test*", "testsss", true},
		{"test**", "testsss", false},
		{"test**", "test*", true},
		{"**test", "*test", true},
		{"TEST", "test", false},
		{"test", "test", true},
	}

	for _, tt := range tests {
		actual := GlobbedStringsMatch(tt.item, tt.val)

		if actual != tt.expect {
			t.Fatalf("Bad testcase %#v, expected %t, got %t", tt, tt.expect, actual)
		}
	}
}
