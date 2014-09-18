package testutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAgent_InitKeyring(t *testing.T) {
	key := "tbLJg26ZJyJ9pK3qhc9jig=="

	dir, err := ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(dir)
	keyFile := filepath.Join(dir, "test/keyring")

	if err := InitKeyring(keyFile, key); err != nil {
		t.Fatalf("err: %s", err)
	}

	fi, err := os.Stat(filepath.Dir(keyFile))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !fi.IsDir() {
		t.Fatalf("bad: %#v", fi)
	}

	data, err := ioutil.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := `["tbLJg26ZJyJ9pK3qhc9jig=="]`
	if string(data) != expected {
		t.Fatalf("bad: %#v", string(data))
	}
}
