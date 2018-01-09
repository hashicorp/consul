package fall

import "testing"

func TestEqual(t *testing.T) {
	var z F
	f := F{Zones: []string{"example.com."}}
	g := F{Zones: []string{"example.net."}}
	h := F{Zones: []string{"example.com."}}

	if !f.Equal(h) {
		t.Errorf("%v should equal %v", f, h)
	}

	if z.Equal(f) {
		t.Errorf("%v should not be equal to %v", z, f)
	}

	if f.Equal(g) {
		t.Errorf("%v should not be equal to %v", f, g)
	}
}

func TestZero(t *testing.T) {
	var f F
	if !f.Equal(Zero) {
		t.Errorf("F should be zero")
	}
}

func TestSetZonesFromArgs(t *testing.T) {
	var f F
	f.SetZonesFromArgs([]string{})
	if !f.Equal(Root) {
		t.Errorf("F should have the root zone")
	}

	f.SetZonesFromArgs([]string{"example.com", "example.net."})
	expected := F{Zones: []string{"example.com.", "example.net."}}
	if !f.Equal(expected) {
		t.Errorf("F should be %v but is %v", expected, f)
	}
}

func TestFallthrough(t *testing.T) {
	var fall F
	if fall.Through("foo.com.") {
		t.Errorf("Expected false, got true for zero fallthrough")
	}

	fall.SetZonesFromArgs([]string{})
	if !fall.Through("foo.net.") {
		t.Errorf("Expected true, got false for all zone fallthrough")
	}

	fall.SetZonesFromArgs([]string{"foo.com", "bar.com"})

	if fall.Through("foo.net.") {
		t.Errorf("Expected false, got true for non-matching fallthrough zone")
	}

	if !fall.Through("bar.com.") {
		t.Errorf("Expected true, got false for matching fallthrough zone")
	}
}
