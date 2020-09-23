package uiserver

import "net/http"

// redirectFS is an http.FS that serves the index.html file for any path that is
// not found on the underlying FS.
//
// TODO: it seems better to actually 404 bad paths or at least redirect them
// rather than pretend index.html is everywhere but this is behavior changing
// so I don't want to take it on as part of this refactor.
type redirectFS struct {
	fs http.FileSystem
}

func (fs *redirectFS) Open(name string) (http.File, error) {
	file, err := fs.fs.Open(name)
	if err != nil {
		file, err = fs.fs.Open("/index.html")
	}
	return file, err
}
