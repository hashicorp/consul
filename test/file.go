package test

import (
	"io/ioutil"
	"os"
	"testing"
)

// TempFile will create a temporary file on disk and returns the name and a cleanup function to remove it later.
func TempFile(t *testing.T, dir, content string) (string, func(), error) {
	f, err := ioutil.TempFile(dir, "go-test-tmpfile")
	if err != nil {
		return "", nil, err
	}
	if err := ioutil.WriteFile(f.Name(), []byte(content), 0644); err != nil {
		return "", nil, err
	}
	rmFunc := func() { os.Remove(f.Name()) }
	return f.Name(), rmFunc, nil
}
