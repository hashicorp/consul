package logger

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	now = time.Now
)

//LogFile is used to setup a file based logger that also performs log rotation
type LogFile struct {
	//Name of the log file
	fileName string

	//Path to the log file
	logPath string

	//Duration between each file rotation operation
	duration time.Duration

	//LastContact is the last time a write action was performed (Tracked as Unix seconds)
	LastContact time.Time

	//FileInfo is the pointer to the current file being written to
	FileInfo *os.File

	//MaxBytes is the maximum number of desired bytes for a log file
	MaxBytes int

	//BytesWritten is the number of bytes written in the current log file
	BytesWritten int
}

func (l *LogFile) openNew() error {
	// Extract the file extention
	fileExt := filepath.Ext(l.fileName)
	// Remove the file extention from the filename
	fileName := strings.TrimSuffix(l.fileName, fileExt)
	// New file name has the format : filename-timestamp.extension
	newfileName := fileName + "-" + strconv.FormatInt(now().Unix(), 10) + fileExt
	newfilePath := filepath.Join(l.logPath, newfileName)
	// Try creating a file. We truncate the file because we are the only authority to write the logs
	filePointer, err := os.OpenFile(newfilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 640)
	if err != nil {
		return err
	}
	l.FileInfo = filePointer
	// New file, new bytes tracker :)
	l.BytesWritten = 0
	return nil
}

func (l *LogFile) rotate() error {
	// Get the time from the last point of contact
	timeElapsed := time.Since(l.LastContact)
	// Rotate if we hit the byte file limit or the time limit
	if (l.BytesWritten > l.MaxBytes && (l.MaxBytes > 0)) || timeElapsed >= l.duration {
		l.FileInfo.Close()
		return l.openNew()
	}
	return nil
}

func (l *LogFile) Write(b []byte) (n int, err error) {
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
	l.LastContact = now()
	l.BytesWritten += len(b)
	return l.FileInfo.Write(b)
}
