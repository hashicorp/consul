package helpers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

func loadFromFile(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("Failed to read file: %v", err)
	}
	return string(data), nil
}

func loadFromStdin(testStdin io.Reader) (string, error) {
	var stdin io.Reader = os.Stdin
	if testStdin != nil {
		stdin = testStdin
	}

	var b bytes.Buffer
	if _, err := io.Copy(&b, stdin); err != nil {
		return "", fmt.Errorf("Failed to read stdin: %v", err)
	}
	return b.String(), nil
}

func LoadDataSource(data string, testStdin io.Reader) (string, error) {
	// Handle empty quoted shell parameters
	if len(data) == 0 {
		return "", nil
	}

	switch data[0] {
	case '@':
		return loadFromFile(data[1:])
	case '-':
		if len(data) > 1 {
			return data, nil
		}
		return loadFromStdin(testStdin)
	default:
		return data, nil
	}
}

func LoadDataSourceNoRaw(data string, testStdin io.Reader) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("Failed to load data: must specify a file path or '-' for stdin")
	}

	if data == "-" {
		return loadFromStdin(testStdin)
	}

	return loadFromFile(data)
}
