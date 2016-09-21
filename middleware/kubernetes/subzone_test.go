package kubernetes

import (
	"testing"
)

// List of configured zones to test against
var confZones = []string{
	"a.b.c",
	"d",
}

// Map of zonename :: expected boolean result
var examplesSubzoneConflict = map[string]bool{
	"a.b.c":   true,  // conflicts with zone "a.b.c"
	"b.c":     true,  // conflicts with zone "a.b.c"
	"c":       true,  // conflicts with zone "a.b.c"
	"e":       false, // no conflict
	"a.b.c.e": false, // no conflict
	"a.b.c.d": true,  // conflicts with zone "d"
	"":        false,
}

func TestSubzoneConflict(t *testing.T) {
	for z, expected := range examplesSubzoneConflict {
		actual, conflicts := subzoneConflict(confZones, z)

		if actual != expected {
			t.Errorf("Expected conflict result '%v' for example '%v'. Instead got '%v'. Conflicting zones are: %v", expected, z, actual, conflicts)
		}
	}
}
