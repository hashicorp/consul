// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

const ServerMetadataFile = "server_metadata.json"

// ServerMetadata ...
type ServerMetadata struct {
	LastSeenUnix int64 `json:"last_seen_unix"`
}

func (md *ServerMetadata) CheckLastSeen(d time.Duration) error {
	lastSeen := time.Unix(md.LastSeenUnix, 0)
	maxAge := time.Now().Add(-d)

	if lastSeen.Before(maxAge) {
		return fmt.Errorf("server is older than specified %s max age", d)
	}

	return nil
}

func OpenServerMetadata(filename string) (io.WriteCloser, error) {
	return os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
}

func ReadServerMetadata(filename string) (*ServerMetadata, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		// Return early if it doesn't as this indicates the server is starting for the first time.
		if err == os.ErrNotExist {
			return nil, nil
		}
		return nil, err
	}

	var md ServerMetadata
	if err := json.Unmarshal(b, &md); err != nil {
		return nil, err
	}

	return &md, nil
}

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
