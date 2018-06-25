package file

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// This doesn't really test the "atomic" part of this function. It really
// tests that it just writes the file properly. I would love to test this
// better but I'm not sure how. -mitchellh
func TestWriteAtomic(t *testing.T) {
	require := require.New(t)
	td, err := ioutil.TempDir("", "lib-file")
	require.NoError(err)
	defer os.RemoveAll(td)

	// Create a subdir that doesn't exist to test that it is created
	path := filepath.Join(td, "subdir", "file")

	// Write
	expected := []byte("hello")
	require.NoError(WriteAtomic(path, expected))

	// Read and verify
	actual, err := ioutil.ReadFile(path)
	require.NoError(err)
	require.Equal(expected, actual)
}
