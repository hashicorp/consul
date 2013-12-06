package consul

import (
	"io/ioutil"
	"os"
	"testing"
)

func tmpDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir
}

func TestServer_StartStop(t *testing.T) {
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	config := DefaultConfig()
	config.DataDir = dir

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := server.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Idempotent
	if err := server.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}
