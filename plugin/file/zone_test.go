package file

import "testing"

func TestNameFromRight(t *testing.T) {
	z := NewZone("example.org.", "stdin")

	tests := []struct {
		in       string
		labels   int
		shot     bool
		expected string
	}{
		{"example.org.", 0, false, "example.org."},
		{"a.example.org.", 0, false, "example.org."},
		{"a.example.org.", 1, false, "a.example.org."},
		{"a.example.org.", 2, true, "a.example.org."},
		{"a.b.example.org.", 2, false, "a.b.example.org."},
	}

	for i, tc := range tests {
		got, shot := z.nameFromRight(tc.in, tc.labels)
		if got != tc.expected {
			t.Errorf("Test %d: expected %s, got %s", i, tc.expected, got)
		}
		if shot != tc.shot {
			t.Errorf("Test %d: expected shot to be %t, got %t", i, tc.shot, shot)
		}
	}
}
