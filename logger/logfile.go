package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/logutils"
)

var (
	now = time.Now
)

//LogFile is used to setup a file based logger that also performs log rotation
type LogFile struct {
	// Log level Filter to filter out logs that do not matcch LogLevel criteria
	logFilter *logutils.LevelFilter

	//Name of the log file
	fileName string

	//Path to the log file
	logPath string

	//Duration between each file rotation operation
	duration time.Duration

	//LastCreated represents the creation time of the latest log
	LastCreated time.Time

	//FileInfo is the pointer to the current file being written to
	FileInfo *os.File

	//MaxBytes is the maximum number of desired bytes for a log file
	MaxBytes int

	//BytesWritten is the number of bytes written in the current log file
	BytesWritten int64

	// Max rotated files to keep before removing them.
	MaxLogArchives int

	//acquire is the mutex utilized to ensure we have no concurrency issues
	acquire sync.Mutex
}

func (l *LogFile) getFileNamePattern() string {
	// Extract the file extension
	fileExt := filepath.Ext(l.fileName)
	// If we have no file extension we append .log
	if fileExt == "" {
		fileExt = ".log"
	}
	// Remove the file extension from the filename
	fileNameWithoutExtension := strings.TrimSuffix(l.fileName, fileExt)
	return fileNameWithoutExtension + "-%s" + fileExt
}

func (l *LogFile) openNew() error {
	fileNamePattern := l.getFileNamePattern()
	// New file name has the format : filename-timestamp.extension
	createTime := now()
	newfileName := fmt.Sprintf(fileNamePattern, strconv.FormatInt(createTime.UnixNano(), 10))
	newfilePath := filepath.Join(l.logPath, newfileName)
	// Try creating a file. We truncate the file because we are the only authority to write the logs
	filePointer, err := os.OpenFile(newfilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}

	l.FileInfo = filePointer
	// New file, new bytes tracker, new creation time :)
	l.LastCreated = createTime
	l.BytesWritten = 0
	return nil
}

func (l *LogFile) rotate() error {
	// Get the time from the last point of contact
	timeElapsed := time.Since(l.LastCreated)
	// Rotate if we hit the byte file limit or the time limit
	if (l.BytesWritten >= int64(l.MaxBytes) && (l.MaxBytes > 0)) || timeElapsed >= l.duration {
		l.FileInfo.Close()
		if err := l.purgeArchivesIfNeeded(); err != nil {
			return err
		}
		return l.openNew()
	}
	return nil
}

func (l *LogFile) purgeArchivesIfNeeded() error {
	if l.MaxLogArchives == -1 {
		return nil
	}
	fileNamePattern := l.getFileNamePattern()
	//get all the files that match the log file pattern
	globExpression := filepath.Join(l.logPath, fmt.Sprintf(fileNamePattern, "*"))
	var matches []string
	var err error
	if matches, err = filepath.Glob(globExpression); err != nil {
		return err
	}
	// Prune if there are more files stored than the configured max
	if len(matches) > l.MaxLogArchives {
		//sort files alphanumerically to delete old files first
		sort.Strings(matches)
		for _, filename := range matches[:len(matches)-l.MaxLogArchives] {
			if err = os.Remove(filename); err != nil {
				return err
			}
		}
	}
	return nil
}

// Write is used to implement io.Writer
func (l *LogFile) Write(b []byte) (n int, err error) {
	// Filter out log entries that do not match log level criteria
	if !l.logFilter.Check(b) {
		return 0, nil
	}

	l.acquire.Lock()
	defer l.acquire.Unlock()
	//Create a new file if we have no file to write to
	if l.FileInfo == nil {
		if err := l.openNew(); err != nil {
			return 0, err
		}
	}
	// Check for the last contact and rotate if necessary
	if err := l.rotate(); err != nil {
		return 0, err
	}
	l.BytesWritten += int64(len(b))
	return l.FileInfo.Write(b)
}
