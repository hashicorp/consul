package xds

import (
	"errors"
	"os"
)

func getSystemCAFile() (string, error) {
	for _, file := range certFiles {
		_, err := os.Stat(file)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return "", errors.New("could not find any system provided certificate authority")
}
