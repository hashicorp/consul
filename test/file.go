package test

import (
	"io/ioutil"
	"os"
)

// TempFile will create a temporary file on disk and returns the name and a cleanup function to remove it later.
func TempFile(dir, content string) (string, func(), error) {
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
