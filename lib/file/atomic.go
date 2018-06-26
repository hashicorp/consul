package file

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-uuid"
)

// WriteAtomic writes the given contents to a temporary file in the same
// directory, does an fsync and then renames the file to its real path
func WriteAtomic(path string, contents []byte) error {
	uuid, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}
	tempPath := fmt.Sprintf("%s-%s.tmp", path, uuid)

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	fh, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := fh.Write(contents); err != nil {
		fh.Close()
		os.Remove(tempPath)
		return err
	}
	if err := fh.Sync(); err != nil {
		fh.Close()
		os.Remove(tempPath)
		return err
	}
	if err := fh.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return err
	}
	return nil
}
