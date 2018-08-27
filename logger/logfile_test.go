package logger

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil"
)

const (
	testFileName = "Consul.log"
	testDuration = 2 * time.Second
	testBytes    = 15
)

func TestLogFile_timeRotation(t *testing.T) {
	tempDir := testutil.TempDir(t, "LogWriterTime")
	defer os.Remove(tempDir)
	logFile := LogFile{fileName: testFileName, logPath: tempDir, duration: testDuration}
	logFile.Write([]byte("Hello World"))
	time.Sleep(2 * time.Second)
	logFile.Write([]byte("Second File"))
	want := logFile.logRotated
	if got, _ := ioutil.ReadDir(tempDir); int64(len(got)) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}

func TestLogFile_byteRotation(t *testing.T) {
	tempDir := testutil.TempDir(t, "LogWriterBytes")
	defer os.Remove(tempDir)
	logFile := LogFile{fileName: testFileName, logPath: tempDir, MaxBytes: testBytes, duration: 24 * time.Hour}
	logFile.Write([]byte("Hello World Peace"))
	logFile.Write([]byte("Second File"))
	want := logFile.logRotated
	tempFiles, _ := ioutil.ReadDir(tempDir)
	if got := tempFiles; int64(len(got)) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}
