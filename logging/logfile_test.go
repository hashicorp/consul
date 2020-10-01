package logging

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
)

const (
	testFileName = "Consul.log"
	testDuration = 50 * time.Millisecond
	testBytes    = 10
)

func TestLogFile_timeRotation(t *testing.T) {
	tempDir := testutil.TempDir(t, "LogWriterTime")
	logFile := LogFile{
		fileName: testFileName,
		logPath:  tempDir,
		duration: testDuration,
	}
	logFile.Write([]byte("Hello World"))
	time.Sleep(3 * testDuration)
	logFile.Write([]byte("Second File"))
	want := 2
	if got, _ := ioutil.ReadDir(tempDir); len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}

func TestLogFile_openNew(t *testing.T) {
	tempDir := testutil.TempDir(t, "LogWriterOpen")
	logFile := LogFile{fileName: testFileName, logPath: tempDir, duration: testDuration}
	if err := logFile.openNew(); err != nil {
		t.Errorf("Expected open file %s, got an error (%s)", testFileName, err)
	}

	if _, err := ioutil.ReadFile(logFile.FileInfo.Name()); err != nil {
		t.Errorf("Expected readable file %s, got an error (%s)", logFile.FileInfo.Name(), err)
	}
}

func TestLogFile_byteRotation(t *testing.T) {
	tempDir := testutil.TempDir(t, "LogWriterBytes")
	logFile := LogFile{
		fileName: testFileName,
		logPath:  tempDir,
		MaxBytes: testBytes,
		duration: 24 * time.Hour,
	}
	logFile.Write([]byte("Hello World"))
	logFile.Write([]byte("Second File"))
	want := 2
	tempFiles, _ := ioutil.ReadDir(tempDir)
	if got := tempFiles; len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}

func TestLogFile_deleteArchives(t *testing.T) {
	tempDir := testutil.TempDir(t, "LogWriteDeleteArchives")
	logFile := LogFile{
		fileName: testFileName,
		logPath:  tempDir,
		MaxBytes: testBytes,
		duration: 24 * time.Hour,
		MaxFiles: 1,
	}
	logFile.Write([]byte("[INFO] Hello World"))
	logFile.Write([]byte("[INFO] Second File"))
	logFile.Write([]byte("[INFO] Third File"))
	want := 2
	tempFiles, _ := ioutil.ReadDir(tempDir)
	if got := tempFiles; len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
		return
	}
	for _, tempFile := range tempFiles {
		var bytes []byte
		var err error
		path := filepath.Join(tempDir, tempFile.Name())
		if bytes, err = ioutil.ReadFile(path); err != nil {
			t.Errorf(err.Error())
			return
		}
		contents := string(bytes)

		if contents == "[INFO] Hello World" {
			t.Errorf("Should have deleted the eldest log file")
			return
		}
	}
}

func TestLogFile_deleteArchivesDisabled(t *testing.T) {
	tempDir := testutil.TempDir(t, t.Name())
	logFile := LogFile{
		fileName: testFileName,
		logPath:  tempDir,
		MaxBytes: testBytes,
		duration: 24 * time.Hour,
		MaxFiles: 0,
	}
	logFile.Write([]byte("[INFO] Hello World"))
	logFile.Write([]byte("[INFO] Second File"))
	logFile.Write([]byte("[INFO] Third File"))
	want := 3
	tempFiles, _ := ioutil.ReadDir(tempDir)
	if got := tempFiles; len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
		return
	}
}

func TestLogFile_rotationDisabled(t *testing.T) {
	tempDir := testutil.TempDir(t, t.Name())
	logFile := LogFile{
		fileName: testFileName,
		logPath:  tempDir,
		MaxBytes: testBytes,
		duration: 24 * time.Hour,
		MaxFiles: -1,
	}
	logFile.Write([]byte("[INFO] Hello World"))
	logFile.Write([]byte("[INFO] Second File"))
	logFile.Write([]byte("[INFO] Third File"))
	want := 1
	tempFiles, _ := ioutil.ReadDir(tempDir)
	if got := tempFiles; len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
		return
	}
}
