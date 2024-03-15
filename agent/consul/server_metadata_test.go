// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockServerMetadataWriter struct {
	writeErr error
}

func (m *mockServerMetadataWriter) Write(p []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}

	return 1, nil
}

func TestServerMetadata(t *testing.T) {
	now := time.Now()

	t.Run("TestIsLastSeenStaleTrue", func(t *testing.T) {
		// Create a server that is 48 hours old.
		md := &ServerMetadata{
			LastSeenUnix: now.Add(-48 * time.Hour).Unix(),
		}

		stale := md.IsLastSeenStale(24 * time.Hour)
		assert.True(t, stale)
	})

	t.Run("TestIsLastSeenStaleFalse", func(t *testing.T) {
		// Create a server that is 1 hour old.
		md := &ServerMetadata{
			LastSeenUnix: now.Add(-1 * time.Hour).Unix(),
		}

		stale := md.IsLastSeenStale(24 * time.Hour)
		assert.False(t, stale)
	})
}

func TestWriteServerMetadata(t *testing.T) {
	t.Run("TestWriteError", func(t *testing.T) {
		m := &mockServerMetadataWriter{
			writeErr: errors.New("write error"),
		}

		err := WriteServerMetadata(m)
		require.Error(t, err)
	})

	t.Run("TestOK", func(t *testing.T) {
		b := new(bytes.Buffer)

		err := WriteServerMetadata(b)
		require.NoError(t, err)
		assert.Greater(t, b.Len(), 0)
	})
}

func TestWriteServerMetadata_MultipleTimes(t *testing.T) {
	file, err := os.CreateTemp("", "server_metadata.json")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	// prepare some data in server_metadata.json
	_, err = OpenServerMetadata(file.Name())
	require.NoError(t, err)
	err = WriteServerMetadata(file)
	require.NoError(t, err)
	stat, err := file.Stat()
	require.NoError(t, err)

	t.Run("reopen not truncate file", func(t *testing.T) {
		_, err = OpenServerMetadata(file.Name())
		require.NoError(t, err)

		// file size unchanged
		stat2, err := file.Stat()
		require.NoError(t, err)
		assert.Equal(t, stat.Size(), stat2.Size())
	})

	t.Run("write updates the file", func(t *testing.T) {
		err = WriteServerMetadata(file)
		require.NoError(t, err)
		stat2, err := file.Stat()
		require.NoError(t, err)
		assert.Equal(t, stat.ModTime(), stat2.ModTime())
	})
}
