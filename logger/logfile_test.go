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
	testBytes    = 10
)

func TestLogFile_timeRotation(t *testing.T) {
	t.Parallel()
	tempDir := testutil.TempDir(t, "LogWriterTime")
	defer os.Remove(tempDir)
	logFile := LogFile{fileName: testFileName, logPath: tempDir, duration: testDuration}
	logFile.Write([]byte("Hello World"))
	time.Sleep(2 * time.Second)
	logFile.Write([]byte("Second File"))
	want := 2
	if got, _ := ioutil.ReadDir(tempDir); len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}

func TestLogFile_byteRotation(t *testing.T) {
	t.Parallel()
	tempDir := testutil.TempDir(t, "LogWriterBytes")
	defer os.Remove(tempDir)
	logFile := LogFile{fileName: testFileName, logPath: tempDir, MaxBytes: testBytes, duration: 24 * time.Hour}
	logFile.Write([]byte("Hello World"))
	logFile.Write([]byte("Second File"))
	want := 2
	tempFiles, _ := ioutil.ReadDir(tempDir)
	if got := tempFiles; len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}
