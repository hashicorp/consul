package uiserver

import (
	"io"
	"io/fs"
)

// bufferedFile implements fs.File and allows us to modify a file from disk by
// writing out the new version into a buffer and then serving file reads from
// that.
type bufferedFile struct {
	buf  io.Reader
	info fs.FileInfo
}

func (b *bufferedFile) Stat() (fs.FileInfo, error) {
	return b.info, nil
}

func (b *bufferedFile) Read(bytes []byte) (int, error) {
	return b.buf.Read(bytes)
}

func (b *bufferedFile) Close() error {
	return nil
}

func newBufferedFile(buf io.Reader, info fs.FileInfo) *bufferedFile {
	return &bufferedFile{
		buf:  buf,
		info: info,
	}
}
