package fall

import "testing"

func TestIsNil(t *testing.T) {
	var f *F
	if !f.IsNil() {
		t.Errorf("F should be nil")
	}
}

func TestIsZero(t *testing.T) {
	f := New()
	if !f.IsZero() {
		t.Errorf("F should be zero")
	}
}

func TestFallThroughExample(t *testing.T) {
	if !Example.Through("example.org.") {
		t.Errorf("example.org. should fall through")
	}
	if Example.Through("example.net.") {
		t.Errorf("example.net. should not fall through")
	}
}

func TestFallthrough(t *testing.T) {
	var fall *F
	if fall.Through("foo.com.") {
		t.Errorf("Expected false, got true for nil fallthrough")
	}

	fall = New()
	if !fall.Through("foo.net.") {
		t.Errorf("Expected true, got false for all zone fallthrough")
	}

	fall.SetZones([]string{"foo.com", "bar.com"})

	if fall.Through("foo.net.") {
		t.Errorf("Expected false, got true for non-matching fallthrough zone")
	}

	if !fall.Through("bar.com.") {
		t.Errorf("Expected true, got false for matching fallthrough zone")
	}
}
