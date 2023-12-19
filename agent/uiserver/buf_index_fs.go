// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package uiserver

import (
	"io/fs"
)

// bufIndexFS is an implementation of fs.FS that intercepts requests for
// the index.html file and returns a pre-rendered file from memory.
type bufIndexFS struct {
	fs       fs.FS
	bufIndex fs.File
}

func (fs *bufIndexFS) Open(name string) (fs.File, error) {
	if name == "index.html" {
		return fs.bufIndex, nil
	}
	return fs.fs.Open(name)
}
