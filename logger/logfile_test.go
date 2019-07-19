package logger

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/logutils"
)

const (
	testFileName = "Consul.log"
	testDuration = 2 * time.Second
	testBytes    = 10
)

func TestLogFile_timeRotation(t *testing.T) {
	t.Parallel()
	tempDir := testutil.TempDir(t, "LogWriterTime")
	defer os.Remove(tempDir)
	filt := LevelFilter()
	logFile := LogFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		duration:  testDuration,
	}
	logFile.Write([]byte("Hello World"))
	time.Sleep(2 * time.Second)
	logFile.Write([]byte("Second File"))
	want := 2
	if got, _ := ioutil.ReadDir(tempDir); len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}

func TestLogFile_openNew(t *testing.T) {
	t.Parallel()
	tempDir := testutil.TempDir(t, "LogWriterOpen")
	defer os.Remove(tempDir)
	logFile := LogFile{fileName: testFileName, logPath: tempDir, duration: testDuration}
	if err := logFile.openNew(); err != nil {
		t.Errorf("Expected open file %s, got an error (%s)", testFileName, err)
	}

	if _, err := ioutil.ReadFile(logFile.FileInfo.Name()); err != nil {
		t.Errorf("Expected readable file %s, got an error (%s)", logFile.FileInfo.Name(), err)
	}
}

func TestLogFile_byteRotation(t *testing.T) {
	t.Parallel()
	tempDir := testutil.TempDir(t, "LogWriterBytes")
	defer os.Remove(tempDir)
	filt := LevelFilter()
	filt.MinLevel = logutils.LogLevel("INFO")
	logFile := LogFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		MaxBytes:  testBytes,
		duration:  24 * time.Hour,
	}
	logFile.Write([]byte("Hello World"))
	logFile.Write([]byte("Second File"))
	want := 2
	tempFiles, _ := ioutil.ReadDir(tempDir)
	if got := tempFiles; len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}

func TestLogFile_logLevelFiltering(t *testing.T) {
	t.Parallel()
	tempDir := testutil.TempDir(t, "LogWriterTime")
	defer os.Remove(tempDir)
	filt := LevelFilter()
	logFile := LogFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		MaxBytes:  testBytes,
		duration:  testDuration,
	}
	logFile.Write([]byte("[INFO] This is an info message"))
	logFile.Write([]byte("[DEBUG] This is a debug message"))
	logFile.Write([]byte("[ERR] This is an error message"))
	want := 2
	if got, _ := ioutil.ReadDir(tempDir); len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}

func TestLogFile_deleteArchives(t *testing.T) {
	t.Parallel()
	tempDir := testutil.TempDir(t, "LogWriteDeleteArchives")
	defer os.Remove(tempDir)
	filt := LevelFilter()
	filt.MinLevel = logutils.LogLevel("INFO")
	logFile := LogFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		MaxBytes:  testBytes,
		duration:  24 * time.Hour,
		MaxFiles:  1,
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
	t.Parallel()
	tempDir := testutil.TempDir(t, "LogWriteDeleteArchivesDisabled")
	defer os.Remove(tempDir)
	filt := LevelFilter()
	filt.MinLevel = logutils.LogLevel("INFO")
	logFile := LogFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		MaxBytes:  testBytes,
		duration:  24 * time.Hour,
		MaxFiles:  0,
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
