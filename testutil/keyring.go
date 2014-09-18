package testutil

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InitKeyring will create a keyring file at a given path.
func InitKeyring(path, key string) error {
	if _, err := base64.StdEncoding.DecodeString(key); err != nil {
		return fmt.Errorf("Invalid key: %s", err)
	}

	keys := []string{key}
	keyringBytes, err := json.Marshal(keys)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("File already exists: %s", path)
	}

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer fh.Close()

	if _, err := fh.Write(keyringBytes); err != nil {
		os.Remove(path)
		return err
	}

	return nil
}
