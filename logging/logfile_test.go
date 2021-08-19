package logging

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
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

	content, err := ioutil.ReadFile(logFile.FileInfo.Name())
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

	content, err := ioutil.ReadFile(filepath.Join(tempDir, logFiles[0]))
	require.NoError(t, err)
	require.Contains(t, string(content), "Second File")

	content, err = ioutil.ReadFile(filepath.Join(tempDir, logFiles[1]))
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

func TestLogFile_FileName(t *testing.T) {
	cases := []struct {
		desc                   string
		inputFilename          string
		outputFilenamePattern  string             
		expectError            string
	}{
		{
			desc: "append extension if not given",
			// Test whether a Unix timestamp and .log extension is appended if the
			// input filename has no extension.
			// For example:
			// "consul" => "consul-1629417801659820807.log"
			inputFilename: "consul",
			outputFilenamePattern: `consul-\d+.log`,
		},
		{
			desc: "leave extension if given",
			// Test whether a Unix timestamp is injected before the extension if the
			// input filename has a non-.log extension.
			// For example:
			// "consul.txt" => "consul-1629417801659820807.txt"
			inputFilename: "consul.txt",
			outputFilenamePattern: `consul-\d+.txt`,
		},
		{
			desc: "template with human readable timestamp",
			// Test whether a human-readable timestamp is injected at the location
			// specified in the template.
			// For example:
			// "con-${timestamp-human}-sul" => "con-2021-08-19_20-13-08-322108785-sul.log"
			inputFilename: "con-${timestamp-human}-sul",
			outputFilenamePattern: `con-\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}-\d{9}-sul.log`,
		},
		{
			desc: "template with unix timestamp",
			// Test whether a human-readable timestamp is injected at the location
			// specified in the template.
			// For example:
			// "${timestamp-unix}-consul" => "1629418506846614551-consul.log"
			inputFilename: "${timestamp-unix}-consul",
			outputFilenamePattern: `\d+-consul.log`,
		},
		{
			desc: "template with multiple fields",
			// Test whether multiple fields can be injected. This isn't expected to be
			// a real use case, but we want to test this extreme case.
			// For example:
			// "${timestamp-unix}-con${timestamp-unix}sul-${timestamp-human}.txt" =>
			// "1629418506846614551-con1629418506846614551sul-2021-08-19_20-13-08-322108785.txt"
			inputFilename: "${timestamp-unix}-con${timestamp-unix}sul-${timestamp-human}.txt",
			outputFilenamePattern: `\d+-con\d+sul-\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}-\d{9}.txt`,
		},
		{
			desc: "template valid but no known variables used",
			// Test whether a Unix timestamp is injected if a filename template is used,
			// but no known timestamp variables (timestamp-unix, timestamp-human) are
			// referenced.
			// For example:
			// "{1+3}" => "4-1629417801659820807.log"
			inputFilename: "${1+3}",
			outputFilenamePattern: `4-\d+.log`,
		},
		{
			desc: "template invalid unknown variable",
			// Test whether an error occurs when an unknown variable is referenced
			// in a template filename.
			// For example:
			// "${abc}-consul" => error
			inputFilename: "${abc}-consul",
			expectError: "failed to evaluate log filename template",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			require := require.New(t)

			tempDir := testutil.TempDir(t, t.Name())
			logFile := LogFile{
				fileName: c.inputFilename,
				logPath:  tempDir,
				MaxBytes: 10,
				MaxFiles: -1,
			}

			n, err := logFile.Write([]byte("[INFO] Hello World"))
			if c.expectError != "" {
				require.Equal(n, 0)
				require.Contains(err.Error(), c.expectError)
			} else {
				// Should be one file in the temporary directory
				logFiles := listDir(t, tempDir)
				require.Len(listDir(t, tempDir), 1)

				// The filename should match the specified pattern
				matched, err := regexp.Match(c.outputFilenamePattern, []byte(logFiles[0]))
				require.True(matched)
				require.Nil(err)	
			}
		})
	}
}

func listDir(t *testing.T, name string) []string {
	t.Helper()
	fh, err := os.Open(name)
	require.NoError(t, err)
	files, err := fh.Readdirnames(100)
	require.NoError(t, err)
	return files
}
