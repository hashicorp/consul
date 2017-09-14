package auto

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"testing"
)

func TestWatcher(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	tempdir, err := createFiles()
	if err != nil {
		if tempdir != "" {
			os.RemoveAll(tempdir)
		}
		t.Fatal(err)
	}
	defer os.RemoveAll(tempdir)

	ldr := loader{
		directory: tempdir,
		re:        regexp.MustCompile(`db\.(.*)`),
		template:  `${1}`,
	}

	a := Auto{
		loader: ldr,
		Zones:  &Zones{},
	}

	a.Walk()

	// example.org and example.com should exist
	if x := len(a.Zones.Z["example.org."].All()); x != 4 {
		t.Fatalf("Expected 4 RRs, got %d", x)
	}
	if x := len(a.Zones.Z["example.com."].All()); x != 4 {
		t.Fatalf("Expected 4 RRs, got %d", x)
	}

	// Now remove one file, rescan and see if it's gone.
	if err := os.Remove(path.Join(tempdir, "db.example.com")); err != nil {
		t.Fatal(err)
	}

	a.Walk()

	if _, ok := a.Zones.Z["example.com."]; ok {
		t.Errorf("Expected %q to be gone.", "example.com.")
	}
	if _, ok := a.Zones.Z["example.org."]; !ok {
		t.Errorf("Expected %q to still be there.", "example.org.")
	}
}
