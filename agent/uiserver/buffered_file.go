package uiserver

import (
	"bytes"
	"io/fs"
	"os"
	"time"
)

// bufferedFile implements fs.File and allows us to modify a file from disk by
// writing out the new version into a buffer and then serving file reads from
// that.
type bufferedFile struct {
	buf  *bytes.Buffer
	info fs.FileInfo
}

var _ fs.FileInfo = (*bufferedFile)(nil)

func newBufferedFile(buf *bytes.Buffer, info fs.FileInfo) *bufferedFile {
	return &bufferedFile{
		buf:  buf,
		info: info,
	}
}

func (b *bufferedFile) Stat() (fs.FileInfo, error) {
	return b, nil
}

func (b *bufferedFile) Read(bytes []byte) (int, error) {
	return b.buf.Read(bytes)
}

func (b *bufferedFile) Close() error {
	return nil
}

func (b *bufferedFile) Name() string {
	return b.info.Name()
}

func (b *bufferedFile) Size() int64 {
	return int64(b.buf.Len())
}

func (b *bufferedFile) Mode() os.FileMode {
	return b.info.Mode()
}

func (b *bufferedFile) ModTime() time.Time {
	return b.info.ModTime()
}

func (b *bufferedFile) IsDir() bool {
	return false
}

func (b *bufferedFile) Sys() any {
	return nil
}
