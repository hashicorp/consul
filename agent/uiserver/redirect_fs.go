// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package uiserver

import (
	"io/fs"
)

// redirectFS is an fs.FS that serves the index.html file for any path that is
// not found on the underlying FS.
//
// TODO: it seems better to actually 404 bad paths or at least redirect them
// rather than pretend index.html is everywhere but this is behavior changing
// so I don't want to take it on as part of this refactor.
type redirectFS struct {
	fs fs.FS
}

func (fs *redirectFS) Open(name string) (fs.File, error) {
	file, err := fs.fs.Open(name)
	if err != nil {
		file, err = fs.fs.Open("index.html")
	}
	return file, err
}
