package helpers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

func LoadDataSource(data string, testStdin io.Reader) (string, error) {
	var stdin io.Reader = os.Stdin
	if testStdin != nil {
		stdin = testStdin
	}

	// Handle empty quoted shell parameters
	if len(data) == 0 {
		return "", nil
	}

	switch data[0] {
	case '@':
		data, err := ioutil.ReadFile(data[1:])
		if err != nil {
			return "", fmt.Errorf("Failed to read file: %s", err)
		} else {
			return string(data), nil
		}
	case '-':
		if len(data) > 1 {
			return data, nil
		}
		var b bytes.Buffer
		if _, err := io.Copy(&b, stdin); err != nil {
			return "", fmt.Errorf("Failed to read stdin: %s", err)
		}
		return b.String(), nil
	default:
		return data, nil
	}
}
