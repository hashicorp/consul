package uiserver

import (
	"net/http"
	"os"
)

// bufIndexFS is an implementation of http.FS that intercepts requests for
// the index.html file and returns a pre-rendered file from memory.
type bufIndexFS struct {
	fs            http.FileSystem
	indexRendered []byte
	indexInfo     os.FileInfo
}

func (fs *bufIndexFS) Open(name string) (http.File, error) {
	if name == "/index.html" {
		return newBufferedFile(fs.indexRendered, fs.indexInfo), nil
	}
	return fs.fs.Open(name)
}
