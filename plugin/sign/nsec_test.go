package sign

import (
	"os"
	"testing"

	"github.com/coredns/coredns/plugin/file"
)

func TestNames(t *testing.T) {
	f, err := os.Open("testdata/db.miek.nl_ns")
	if err != nil {
		t.Error(err)
	}
	z, err := file.Parse(f, "db.miek.nl_ns", "miek.nl", 0)
	if err != nil {
		t.Error(err)
	}

	names := names("miek.nl.", z)
	expected := []string{"miek.nl.", "child.miek.nl.", "www.miek.nl."}
	for i := range names {
		if names[i] != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], names[i])
		}
	}
}
