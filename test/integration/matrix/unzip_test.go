package test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnzip(t *testing.T) {
	err := unzip("./testdata/foo.zip", "./testdata/foo")
	require.NoError(t, err)
	require.FileExists(t, "./testdata/foo")
	err = os.Remove("./testdata/foo")
	require.NoError(t, err)
}
