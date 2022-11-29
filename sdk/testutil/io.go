package testutil

import (
	"os"
	"strings"
	"testing"
)

var noCleanup = strings.ToLower(os.Getenv("TEST_NOCLEANUP")) == "true"

// TempDir creates a temporary directory within tmpdir with the name 'testname-name'.
// If the directory cannot be created t.Fatal is called.
// The directory will be removed when the test ends. Set TEST_NOCLEANUP env var
// to prevent the directory from being removed.
func TempDir(t testing.TB, name string) string {
	if t == nil {
		panic("argument t must be non-nil")
	}
	name = t.Name() + "-" + name
	name = strings.Replace(name, "/", "_", -1)
	d, err := os.MkdirTemp("", name)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	t.Cleanup(func() {
		if noCleanup {
			t.Logf("skipping cleanup because TEST_NOCLEANUP was enabled")
			return
		}
		os.RemoveAll(d)
	})
	return d
}

// TempFile creates a temporary file within tmpdir with the name 'testname-name'.
// If the file cannot be created t.Fatal is called. If a temporary directory
// has been created before consider storing the file inside this directory to
// avoid double cleanup.
// The file will be removed when the test ends.  Set TEST_NOCLEANUP env var
// to prevent the file from being removed.
func TempFile(t testing.TB, name string) *os.File {
	if t == nil {
		panic("argument t must be non-nil")
	}
	name = t.Name() + "-" + name
	name = strings.Replace(name, "/", "_", -1)
	f, err := os.CreateTemp("", name)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	t.Cleanup(func() {
		if noCleanup {
			t.Logf("skipping cleanup because TEST_NOCLEANUP was enabled")
			return
		}
		os.Remove(f.Name())
	})
	return f
}
