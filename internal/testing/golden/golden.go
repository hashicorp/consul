// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package golden

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update *bool

func init() {
	update = flag.Bool("update", false, "update golden files")
}

type options struct {
	baseDirectory    string
	goldenFileSuffix string
	disableUpdate    bool
}

func defaultOptions() *options {
	return &options{
		baseDirectory:    "testdata",
		goldenFileSuffix: ".golden",
	}
}

type Option func(*options)

// WithBaseDirectory overrides the default base directory, ./testdata, to account
// for more complex test data organization
func WithBaseDirectory(baseDirectory string) Option {
	return func(o *options) {
		o.baseDirectory = baseDirectory
	}
}

// WithGoldenFileSuffix allows overriding the default suffix, .golden, that
// is used for golden file updating.
func WithGoldenFileSuffix(goldenFileSuffix string) Option {
	return func(o *options) {
		o.goldenFileSuffix = goldenFileSuffix
	}
}

// WithDisableUpdate disables updating of the golden file even
// when the -update cli flag has been used. This is mainly only
// useful for tests that want to retrieve the golden file contents
// of another test to use as their test input.
func WithDisableUpdate() Option {
	return func(o *options) {
		o.disableUpdate = true
	}
}

func (o *options) filepath(filename string) string {
	return filepath.Join(o.baseDirectory, fmt.Sprintf("%s%s", filename, o.goldenFileSuffix))
}

// ShouldUpdate returns whether golden file tests should rewrite their data
func ShouldUpdate() bool {
	return *update
}

// Filepath resolves the file path for the given filename and options.
func Filepath(filename string, opts ...Option) string {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	return options.filepath(filename)
}

// Get reads the expected value from the file at filename and returns the value.
// filename is relative to the ./testdata directory.
//
// If the `-update` flag is used with `go test`, the golden file will be updated
// to the value of actual.
func Get(t *testing.T, actual, filename string, opts ...Option) string {
	t.Helper()
	return string(GetBytes(t, actual, filename, opts...))
}

// GetAndVerify reads the expected value from file at filename and will also verify
// the equality of the actual and expected values.
func GetAndVerify(t *testing.T, actual, filename string, opts ...Option) string {
	t.Helper()

	return string(GetBytesAndVerify(t, actual, filename, opts...))
}

// GetBytes reads the expected value from the file at filename and returns the
// value as a byte array. filename is relative to the ./testdata directory.
//
// If the `-update` flag is used with `go test`, the golden file will be updated
// to the value of actual.
func GetBytes(t *testing.T, actual, filename string, opts ...Option) []byte {
	t.Helper()

	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	path := options.filepath(filename)
	if *update && !options.disableUpdate {
		WriteContentsToFilePath(t, actual, path)
	}

	return GetBytesAtFilePath(t, path)
}

// GetBytesAndVerify reads the expected value from file at filename using a call to GetBytes
// and will also verify the equality of the actual and expected values.
func GetBytesAndVerify(t *testing.T, actual, filename string, opts ...Option) []byte {
	t.Helper()

	expected := GetBytes(t, actual, filename, opts...)
	require.Equal(t, expected, []byte(actual))
	return expected
}

// WriteBytesAtFilePath writes some expected value to the file specified
// by the fpath parameter. Any necessary intermediate directories will be
// created and the permissions of the file will be set to 0644.
func WriteContentsToFilePath(t *testing.T, actual, fpath string) {
	t.Helper()

	if dir := filepath.Dir(fpath); dir != "." {
		require.NoError(t, os.MkdirAll(dir, 0755))
	}
	err := os.WriteFile(fpath, []byte(actual), 0644)
	require.NoError(t, err)
}

// GetAtFilePath reads the expected value from the file at filepath and returns the
// value as a string
func GetAtFilePath(t *testing.T, filepath string) string {
	t.Helper()
	return string(GetBytesAtFilePath(t, filepath))
}

// GetByteAtFilePath reads the expected value from the file at filepath and returns the
// value as a byte array.
func GetBytesAtFilePath(t *testing.T, filepath string) []byte {
	t.Helper()

	expected, err := os.ReadFile(filepath)
	require.NoError(t, err)
	return expected
}
