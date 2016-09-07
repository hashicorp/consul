package middleware

import "testing"

func TestZoneMatches(t *testing.T) {
	child := "example.org."
	zones := Zones([]string{"org.", "."})
	actual := zones.Matches(child)
	if actual != "org." {
		t.Errorf("Expected %v, got %v", "org.", actual)
	}

	child = "bla.example.org."
	zones = Zones([]string{"bla.example.org.", "org.", "."})
	actual = zones.Matches(child)

	if actual != "bla.example.org." {
		t.Errorf("Expected %v, got %v", "org.", actual)
	}
}

func TestZoneNormalize(t *testing.T) {
	zones := Zones([]string{"example.org", "Example.ORG.", "example.org."})
	expected := "example.org."
	zones.Normalize()

	for _, actual := range zones {
		if actual != expected {
			t.Errorf("Expected %v, got %v\n", expected, actual)
		}
	}
}

func TestNameMatches(t *testing.T) {
	matches := []struct {
		child    string
		parent   string
		expected bool
	}{
		{".", ".", true},
		{"example.org.", ".", true},
		{"example.org.", "example.org.", true},
		{"example.org.", "org.", true},
		{"org.", "example.org.", false},
	}

	for _, m := range matches {
		actual := Name(m.parent).Matches(m.child)
		if actual != m.expected {
			t.Errorf("Expected %v for %s/%s, got %v", m.expected, m.parent, m.child, actual)
		}

	}
}

func TestNameNormalize(t *testing.T) {
	names := []string{
		"example.org", "example.org.",
		"Example.ORG.", "example.org."}

	for i := 0; i < len(names); i += 2 {
		ts := names[i]
		expected := names[i+1]
		actual := Name(ts).Normalize()
		if expected != actual {
			t.Errorf("Expected %v, got %v\n", expected, actual)
		}
	}
}

func TestHostNormalize(t *testing.T) {
	hosts := []string{".:53", ".", "example.org:53", "example.org.", "example.org.:53", "example.org."}

	for i := 0; i < len(hosts); i += 2 {
		ts := hosts[i]
		expected := hosts[i+1]
		actual := Host(ts).Normalize()
		if expected != actual {
			t.Errorf("Expected %v, got %v\n", expected, actual)
		}
	}
}

func TestAddrNormalize(t *testing.T) {
	addrs := []string{".:53", ".:53", "example.org", "example.org:53", "example.org.:1053", "example.org.:1053"}

	for i := 0; i < len(addrs); i += 2 {
		ts := addrs[i]
		expected := addrs[i+1]
		actual := Addr(ts).Normalize()
		if expected != actual {
			t.Errorf("Expected %v, got %v\n", expected, actual)
		}
	}

}
