// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		// Create a server that is 24 hours old.
		md := &ServerMetadata{
			LastSeenUnix: now.Add(-24 * time.Hour).Unix(),
		}

		ok := md.IsLastSeenStale(1 * time.Hour)
		assert.True(t, ok)
	})

	t.Run("TestIsLastSeenStaleFalse", func(t *testing.T) {
		// Create a server that is 24 hours old.
		md := &ServerMetadata{
			LastSeenUnix: now.Add(-1 * time.Hour).Unix(),
		}

		ok := md.IsLastSeenStale(24 * time.Hour)
		assert.False(t, ok)
	})
}

func TestWriteServerMetadata(t *testing.T) {
	t.Run("TestWriteError", func(t *testing.T) {
		m := &mockServerMetadataWriter{
			writeErr: errors.New("write error"),
		}

		err := WriteServerMetadata(m)
		assert.Error(t, err)
	})

	t.Run("TestOK", func(t *testing.T) {
		b := new(bytes.Buffer)

		err := WriteServerMetadata(b)
		assert.NoError(t, err)
		assert.True(t, b.Len() > 0)
	})
}
