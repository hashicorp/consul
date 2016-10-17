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

	z := &Zones{}

	z.Walk(ldr)

	// example.org and example.com should exist
	if x := len(z.Z["example.org."].All()); x != 4 {
		t.Fatalf("expected 4 RRs, got %d", x)
	}
	if x := len(z.Z["example.com."].All()); x != 4 {
		t.Fatalf("expected 4 RRs, got %d", x)
	}

	// Now remove one file, rescan and see if it's gone.
	if err := os.Remove(path.Join(tempdir, "db.example.com")); err != nil {
		t.Fatal(err)
	}

	z.Walk(ldr)
}
