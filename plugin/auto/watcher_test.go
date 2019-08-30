package auto

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestWatcher(t *testing.T) {
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

	// example.org and example.com should exist, we have 3 apex rrs and 1 "real" record. All() returns the non-apex ones.
	if x := len(a.Zones.Z["example.org."].All()); x != 1 {
		t.Fatalf("Expected 1 RRs, got %d", x)
	}
	if x := len(a.Zones.Z["example.com."].All()); x != 1 {
		t.Fatalf("Expected 1 RRs, got %d", x)
	}

	// Now remove one file, rescan and see if it's gone.
	if err := os.Remove(filepath.Join(tempdir, "db.example.com")); err != nil {
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

func TestSymlinks(t *testing.T) {
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

	// Now create a duplicate file in a subdirectory and repoint the symlink
	if err := os.Remove(filepath.Join(tempdir, "db.example.com")); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(tempdir, "..data")
	if err = os.Mkdir(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	newFile := filepath.Join(dataDir, "db.example.com")
	if err = os.Symlink(filepath.Join(tempdir, "db.example.org"), newFile); err != nil {
		t.Fatal(err)
	}

	a.Walk()

	if storedZone, ok := a.Zones.Z["example.com."]; ok {
		storedFile := storedZone.File()
		if storedFile != newFile {
			t.Errorf("Expected %q to reflect new path %q", storedFile, newFile)
		}
	}
}
