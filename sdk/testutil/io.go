package testutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

// tmpdir is the base directory for all temporary directories
// and files created with TempDir and TempFile. This could be
// achieved by setting a system environment variable but then
// the test execution would depend on whether or not the
// environment variable is set.
//
// On macOS the temp base directory is quite long and that
// triggers a problem with some tests that bind to UNIX sockets
// where the filename seems to be too long. Using a shorter name
// fixes this and makes the paths more readable.
//
// It also provides a single base directory for cleanup.
var tmpdir = "/tmp/consul-test"

func init() {
	if err := os.MkdirAll(tmpdir, 0755); err != nil {
		fmt.Printf("Cannot create %s. Reverting to /tmp\n", tmpdir)
		tmpdir = "/tmp"
	}
}

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
	d, err := ioutil.TempDir(tmpdir, name)
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
	f, err := ioutil.TempFile(tmpdir, name)
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
