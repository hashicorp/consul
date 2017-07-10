package agent

import (
	"os"
	"runtime"
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/testutil"
)

func TestStringHash(t *testing.T) {
	t.Parallel()
	in := "hello world"
	expected := "5eb63bbbe01eeed093cb22bb8f5acdc3"

	if out := stringHash(in); out != expected {
		t.Fatalf("bad: %s", out)
	}
}

func TestSetFilePermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}
	tempFile := testutil.TempFile(t, "consul")
	path := tempFile.Name()
	defer os.Remove(path)

	// Bad UID fails
	if err := setFilePermissions(path, config.UnixSocketPermissions{Usr: "%"}); err == nil {
		t.Fatalf("should fail")
	}

	// Bad GID fails
	if err := setFilePermissions(path, config.UnixSocketPermissions{Grp: "%"}); err == nil {
		t.Fatalf("should fail")
	}

	// Bad mode fails
	if err := setFilePermissions(path, config.UnixSocketPermissions{Perms: "%"}); err == nil {
		t.Fatalf("should fail")
	}

	// Allows omitting user/group/mode
	if err := setFilePermissions(path, config.UnixSocketPermissions{}); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Doesn't change mode if not given
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := setFilePermissions(path, config.UnixSocketPermissions{}); err != nil {
		t.Fatalf("err: %s", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if fi.Mode().String() != "-rwx------" {
		t.Fatalf("bad: %s", fi.Mode())
	}

	// Changes mode if given
	if err := setFilePermissions(path, config.UnixSocketPermissions{Perms: "0777"}); err != nil {
		t.Fatalf("err: %s", err)
	}
	fi, err = os.Stat(path)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if fi.Mode().String() != "-rwxrwxrwx" {
		t.Fatalf("bad: %s", fi.Mode())
	}
}
