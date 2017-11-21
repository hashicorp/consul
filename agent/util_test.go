package agent

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil"
	"github.com/pascaldekloe/goe/verify"
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
	if err := setFilePermissions(path, "%", "", ""); err == nil {
		t.Fatalf("should fail")
	}

	// Bad GID fails
	if err := setFilePermissions(path, "", "%", ""); err == nil {
		t.Fatalf("should fail")
	}

	// Bad mode fails
	if err := setFilePermissions(path, "", "", "%"); err == nil {
		t.Fatalf("should fail")
	}

	// Allows omitting user/group/mode
	if err := setFilePermissions(path, "", "", ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Doesn't change mode if not given
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := setFilePermissions(path, "", "", ""); err != nil {
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
	if err := setFilePermissions(path, "", "", "0777"); err != nil {
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

func TestDurationFixer(t *testing.T) {
	obj := map[string]interface{}{
		"key1": []map[string]interface{}{
			{
				"subkey1": "10s",
			},
			{
				"subkey2": "5d",
			},
		},
		"key2": map[string]interface{}{
			"subkey3": "30s",
			"subkey4": "20m",
		},
		"key3": "11s",
		"key4": "49h",
	}
	expected := map[string]interface{}{
		"key1": []map[string]interface{}{
			{
				"subkey1": 10 * time.Second,
			},
			{
				"subkey2": "5d",
			},
		},
		"key2": map[string]interface{}{
			"subkey3": "30s",
			"subkey4": 20 * time.Minute,
		},
		"key3": "11s",
		"key4": 49 * time.Hour,
	}

	fixer := NewDurationFixer("key4", "subkey1", "subkey4")
	if err := fixer.FixupDurations(obj); err != nil {
		t.Fatal(err)
	}

	// Ensure we only processed the intended fieldnames
	verify.Values(t, "", obj, expected)
}
