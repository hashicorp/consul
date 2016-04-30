package dns

import (
	"path/filepath"
	"testing"
)

func TestStorage(t *testing.T) {
	storage = Storage("./le_test")

	if expected, actual := filepath.Join("le_test", "zones"), storage.Zones(); actual != expected {
		t.Errorf("Expected Zones() to return '%s' but got '%s'", expected, actual)
	}
	if expected, actual := filepath.Join("le_test", "zones", "test.com"), storage.Zone("test.com"); actual != expected {
		t.Errorf("Expected Site() to return '%s' but got '%s'", expected, actual)
	}
	if expected, actual := filepath.Join("le_test", "zones", "test.com", "db.test.com"), storage.SecondaryZoneFile("test.com"); actual != expected {
		t.Errorf("Expected SecondaryZoneFile() to return '%s' but got '%s'", expected, actual)
	}
}
