// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// ServerMetadataFile is the name of the file on disk that server metadata
// should be written to.
const ServerMetadataFile = "server_metadata.json"

// ServerMetadata represents specific metadata about a running server.
type ServerMetadata struct {
	// LastSeenUnix is the timestamp a server was last seen, in Unix format.
	LastSeenUnix int64 `json:"last_seen_unix"`
}

// IsLastSeenStale checks whether the last seen timestamp is older than a given duration.
func (md *ServerMetadata) IsLastSeenStale(d time.Duration) bool {
	lastSeen := time.Unix(md.LastSeenUnix, 0)
	maxAge := time.Now().Add(-d)

	return lastSeen.Before(maxAge)
}

// OpenServerMetadata is a helper function for opening the server metadata file
// with the correct permissions.
func OpenServerMetadata(filename string) (io.WriteCloser, error) {
	return os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
}

type ServerMetadataReadFunc func(filename string) (*ServerMetadata, error)

// ReadServerMetadata is a helper function for reading the contents of a server
// metadata file and unmarshaling the data from JSON.
func ReadServerMetadata(filename string) (*ServerMetadata, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var md ServerMetadata
	if err := json.Unmarshal(b, &md); err != nil {
		return nil, err
	}

	return &md, nil
}

// WriteServerMetadata writes server metadata to a file in JSON format.
func WriteServerMetadata(w io.Writer) error {
	md := &ServerMetadata{
		LastSeenUnix: time.Now().Unix(),
	}

	b, err := json.Marshal(md)
	if err != nil {
		return err
	}

	if _, err := w.Write(b); err != nil {
		return err
	}

	return nil
}
