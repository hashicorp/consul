package storage

import (
	"os"
	"path"
	"strings"
	"testing"
)

func TestFsPath(t *testing.T) {
	if actual := fsPath(); !strings.HasSuffix(actual, ".coredns") {
		t.Errorf("Expected path to be a .coredns folder, got: %v", actual)
	}

	os.Setenv("COREDNSPATH", "testpath")
	defer os.Setenv("COREDNSPATH", "")
	if actual, expected := fsPath(), "testpath"; actual != expected {
		t.Errorf("Expected path to be %v, got: %v", expected, actual)
	}
}

func TestZone(t *testing.T) {
	for _, ts := range []string{"example.org.", "example.org"} {
		d := CoreDir.Zone(ts)
		actual := path.Base(string(d))
		expected := "D" + ts
		if actual != expected {
			t.Errorf("Expected path to be %v, got %v", actual, expected)
		}
	}
}

func TestZoneRoot(t *testing.T) {
	for _, ts := range []string{"."} {
		d := CoreDir.Zone(ts)
		actual := path.Base(string(d))
		expected := "D" + ts
		if actual != expected {
			t.Errorf("Expected path to be %v, got %v", actual, expected)
		}
	}
}
