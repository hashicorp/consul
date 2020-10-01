package uiserver

import (
	"bytes"
	"errors"
	"os"
	"time"
)

// bufferedFile implements http.File and allows us to modify a file from disk by
// writing out the new version into a buffer and then serving file reads from
// that.
type bufferedFile struct {
	buf  *bytes.Reader
	info os.FileInfo
}

func newBufferedFile(buf []byte, info os.FileInfo) *bufferedFile {
	return &bufferedFile{
		buf:  bytes.NewReader(buf),
		info: info,
	}
}

func (t *bufferedFile) Read(p []byte) (n int, err error) {
	return t.buf.Read(p)
}

func (t *bufferedFile) Seek(offset int64, whence int) (int64, error) {
	return t.buf.Seek(offset, whence)
}

func (t *bufferedFile) Close() error {
	return nil
}

func (t *bufferedFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("not a directory")
}

func (t *bufferedFile) Stat() (os.FileInfo, error) {
	return t, nil
}

func (t *bufferedFile) Name() string {
	return t.info.Name()
}

func (t *bufferedFile) Size() int64 {
	return int64(t.buf.Len())
}

func (t *bufferedFile) Mode() os.FileMode {
	return t.info.Mode()
}

func (t *bufferedFile) ModTime() time.Time {
	return t.info.ModTime()
}

func (t *bufferedFile) IsDir() bool {
	return false
}

func (t *bufferedFile) Sys() interface{} {
	return nil
}
