// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logging

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestLogFile_Rotation_MaxDuration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tempDir := testutil.TempDir(t, "")
	logFile := LogFile{
		fileName: "consul.log",
		logPath:  tempDir,
		duration: 50 * time.Millisecond,
	}

	logFile.Write([]byte("Hello World"))
	time.Sleep(3 * logFile.duration)
	logFile.Write([]byte("Second File"))
	require.Len(t, listDir(t, tempDir), 2)
}

func TestLogFile_openNew(t *testing.T) {
	logFile := LogFile{
		fileName: "consul.log",
		logPath:  testutil.TempDir(t, ""),
		duration: defaultRotateDuration,
	}
	err := logFile.openNew()
	require.NoError(t, err)

	msg := "[INFO] Something"
	_, err = logFile.Write([]byte(msg))
	require.NoError(t, err)

	content, err := os.ReadFile(logFile.FileInfo.Name())
	require.NoError(t, err)
	require.Contains(t, string(content), msg)
}

func TestLogFile_Rotation_MaxBytes(t *testing.T) {
	tempDir := testutil.TempDir(t, "LogWriterBytes")
	logFile := LogFile{
		fileName: "somefile.log",
		logPath:  tempDir,
		MaxBytes: 10,
		duration: defaultRotateDuration,
	}
	logFile.Write([]byte("Hello World"))
	logFile.Write([]byte("Second File"))
	require.Len(t, listDir(t, tempDir), 2)
}

func TestLogFile_PruneFiles(t *testing.T) {
	tempDir := testutil.TempDir(t, t.Name())
	logFile := LogFile{
		fileName: "consul.log",
		logPath:  tempDir,
		MaxBytes: 10,
		duration: defaultRotateDuration,
		MaxFiles: 1,
	}
	logFile.Write([]byte("[INFO] Hello World"))
	logFile.Write([]byte("[INFO] Second File"))
	logFile.Write([]byte("[INFO] Third File"))

	logFiles := listDir(t, tempDir)
	sort.Strings(logFiles)
	require.Len(t, logFiles, 2)

	content, err := os.ReadFile(filepath.Join(tempDir, logFiles[0]))
	require.NoError(t, err)
	require.Contains(t, string(content), "Second File")

	content, err = os.ReadFile(filepath.Join(tempDir, logFiles[1]))
	require.NoError(t, err)
	require.Contains(t, string(content), "Third File")
}

func TestLogFile_PruneFiles_Disabled(t *testing.T) {
	tempDir := testutil.TempDir(t, t.Name())
	logFile := LogFile{
		fileName: "somename.log",
		logPath:  tempDir,
		MaxBytes: 10,
		duration: defaultRotateDuration,
		MaxFiles: 0,
	}
	logFile.Write([]byte("[INFO] Hello World"))
	logFile.Write([]byte("[INFO] Second File"))
	logFile.Write([]byte("[INFO] Third File"))
	require.Len(t, listDir(t, tempDir), 3)
}

func TestLogFile_FileRotation_Disabled(t *testing.T) {
	tempDir := testutil.TempDir(t, t.Name())
	logFile := LogFile{
		fileName: "consul.log",
		logPath:  tempDir,
		MaxBytes: 10,
		MaxFiles: -1,
	}
	logFile.Write([]byte("[INFO] Hello World"))
	logFile.Write([]byte("[INFO] Second File"))
	logFile.Write([]byte("[INFO] Third File"))
	require.Len(t, listDir(t, tempDir), 1)
}

func listDir(t *testing.T, name string) []string {
	t.Helper()
	fh, err := os.Open(name)
	require.NoError(t, err)
	files, err := fh.Readdirnames(100)
	require.NoError(t, err)
	return files
}
