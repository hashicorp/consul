// +build linux

package lumberjack

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestMaintainMode(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir("TestMaintainMode", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	isNil(err, t)
	f.Close()

	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	isNil(err, t)
	info2, err := os.Stat(filename2)
	isNil(err, t)
	equals(mode, info.Mode(), t)
	equals(mode, info2.Mode(), t)
}

func TestMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	os_Chown = fakeFS.Chown
	os_Stat = fakeFS.Stat
	defer func() {
		os_Chown = os.Chown
		os_Stat = os.Stat
	}()
	currentTime = fakeTime
	dir := makeTempDir("TestMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	isNil(err, t)
	f.Close()

	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	equals(555, fakeFS.files[filename].uid, t)
	equals(666, fakeFS.files[filename].gid, t)
}

func TestCompressMaintainMode(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestCompressMaintainMode", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	isNil(err, t)
	f.Close()

	l := &Logger{
		Compress: true,
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// mode.
	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	isNil(err, t)
	info2, err := os.Stat(filename2+compressSuffix)
	isNil(err, t)
	equals(mode, info.Mode(), t)
	equals(mode, info2.Mode(), t)
}

func TestCompressMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	os_Chown = fakeFS.Chown
	os_Stat = fakeFS.Stat
	defer func() {
		os_Chown = os.Chown
		os_Stat = os.Stat
	}()
	currentTime = fakeTime
	dir := makeTempDir("TestCompressMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	isNil(err, t)
	f.Close()

	l := &Logger{
		Compress:   true,
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// owner.
	filename2 := backupFile(dir)
	equals(555, fakeFS.files[filename2+compressSuffix].uid, t)
	equals(666, fakeFS.files[filename2+compressSuffix].gid, t)
}

type fakeFile struct {
	uid int
	gid int
}

type fakeFS struct {
	files map[string]fakeFile
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: make(map[string]fakeFile)}
}

func (fs *fakeFS) Chown(name string, uid, gid int) error {
	fs.files[name] = fakeFile{uid: uid, gid: gid}
	return nil
}

func (fs *fakeFS) Stat(name string) (os.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	stat.Uid = 555
	stat.Gid = 666
	return info, nil
}
