package test

import (
	"io/ioutil"
	"os"
	"testing"
)

// Zone will create a temporary file on disk and returns the name and
// cleanup function to remove it later.
func Zone(t *testing.T, dir, zonefile string) (string, func(), error) {
	f, err := ioutil.TempFile(dir, "go-test-zone")
	if err != nil {
		return "", nil, err
	}
	if err := ioutil.WriteFile(f.Name(), []byte(zonefile), 0644); err != nil {
		return "", nil, err
	}
	rmFunc := func() { os.Remove(f.Name()) }
	return f.Name(), rmFunc, nil
}
