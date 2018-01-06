package plugin

import "testing"

func TestFallthrough(t *testing.T) {
	if Fallthrough(nil, "foo.com.") {
		t.Errorf("Expected false, got true for nil fallthrough")
	}

	if !Fallthrough(&[]string{}, "foo.net.") {
		t.Errorf("Expected true, got false for all zone fallthrough")
	}

	if Fallthrough(&[]string{"foo.com", "bar.com"}, "foo.net") {
		t.Errorf("Expected false, got true for non-matching fallthrough zone")
	}

	if !Fallthrough(&[]string{"foo.com.", "bar.com."}, "bar.com.") {
		t.Errorf("Expected true, got false for matching fallthrough zone")
	}
}
