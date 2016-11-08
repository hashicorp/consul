package logbuf

import (
	"bufio"
	"container/ring"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	dirPerms  = syscall.S_IRWXU | syscall.S_IRGRP | syscall.S_IXGRP
	filePerms = syscall.S_IRUSR | syscall.S_IWUSR | syscall.S_IRGRP

	timeLayout = "2006-01-02:15:04:05.999"
)

func newLogBuffer(length uint, dirname string, quota uint64) *LogBuffer {
	logBuffer := &LogBuffer{
		buffer: ring.New(int(length)),
		logDir: dirname,
		quota:  quota}
	if err := logBuffer.setupFileLogging(); err != nil {
		fmt.Fprintln(logBuffer, err)
	}
	logBuffer.addHttpHandlers()
	return logBuffer
}

func (lb *LogBuffer) setupFileLogging() error {
	if lb.logDir == "" {
		return nil
	}
	if err := lb.createLogDirectory(); err != nil {
		return err
	}
	writeNotifier := make(chan struct{}, 1)
	lb.writeNotifier = writeNotifier
	go lb.flushWhenIdle(writeNotifier)
	return nil
}

func (lb *LogBuffer) createLogDirectory() error {
	if fi, err := os.Stat(lb.logDir); err != nil {
		if err := os.Mkdir(lb.logDir, dirPerms); err != nil {
			return fmt.Errorf("error creating: %s: %s", lb.logDir, err)
		}
		fi, err = os.Stat(lb.logDir)
	} else if !fi.IsDir() {
		return errors.New(lb.logDir + ": is not a directory")
	}
	lb.scanPreviousForPanic()
	return lb.enforceQuota()
}

func (lb *LogBuffer) scanPreviousForPanic() {
	target, err := os.Readlink(path.Join(lb.logDir, "latest"))
	if err != nil {
		return
	}
	targetPath := path.Join(lb.logDir, target)
	file, err := os.Open(targetPath)
	if err != nil {
		return
	}
	go func() {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "panic: ") {
				lb.rwMutex.Lock()
				lb.panicLogfile = &target
				lb.rwMutex.Unlock()
				if fi, err := os.Stat(targetPath); err != nil {
					return
				} else {
					os.Chmod(targetPath, fi.Mode()|os.ModeSticky)
				}
				return
			}
		}
	}()
}

func (lb *LogBuffer) dump(writer io.Writer, prefix, postfix string,
	recentFirst bool) error {
	entries := lb.getEntries()
	if recentFirst {
		reverseEntries(entries)
	}
	for _, entry := range entries {
		writer.Write([]byte(prefix))
		writer.Write(entry)
		writer.Write([]byte(postfix))
	}
	return nil
}

func (lb *LogBuffer) flush() error {
	lb.rwMutex.Lock()
	defer lb.rwMutex.Unlock()
	if lb.writer != nil {
		return lb.writer.Flush()
	}
	return nil
}

func (lb *LogBuffer) write(p []byte) (n int, err error) {
	if *alsoLogToStderr {
		os.Stderr.Write(p)
	}
	val := make([]byte, len(p))
	copy(val, p)
	lb.rwMutex.Lock()
	sendNotify := lb.writeToLogFile(p)
	lb.buffer.Value = val
	lb.buffer = lb.buffer.Next()
	lb.rwMutex.Unlock()
	if sendNotify {
		lb.writeNotifier <- struct{}{}
	}
	return len(p), nil
}

// This should be called with the lock held.
func (lb *LogBuffer) writeToLogFile(p []byte) bool {
	if lb.writer == nil {
		return false
	}
	lb.writer.Write(p)
	lb.usage += uint64(len(p))
	if lb.usage <= lb.quota {
		return true
	}
	lb.enforceQuota()
	return true
}

// This should be called with the lock held.
func (lb *LogBuffer) enforceQuota() error {
	file, err := os.Open(lb.logDir)
	if err != nil {
		return err
	}
	names, err := file.Readdirnames(-1)
	file.Close()
	if err != nil {
		return err
	}
	sort.Strings(names)
	var usage uint64
	deletedLatestFile := false
	deleteRemainingFiles := false
	latestFile := true
	for index := len(names) - 1; index >= 0; index-- {
		filename := path.Join(lb.logDir, names[index])
		fi, err := os.Lstat(filename)
		if err == os.ErrNotExist {
			continue
		}
		if err != nil {
			return err
		}
		if fi.Mode().IsRegular() {
			size := uint64(fi.Size())
			if size < lb.quota>>10 {
				size = lb.quota >> 10 // Limit number of files to 1024.
			}
			if size+usage > lb.quota || deleteRemainingFiles {
				os.Remove(filename)
				deleteRemainingFiles = true
				if latestFile {
					deletedLatestFile = true
				}
			} else {
				usage += size
			}
			latestFile = false
		}
	}
	lb.usage = usage
	if deletedLatestFile && lb.file != nil {
		lb.writer.Flush()
		lb.writer = nil
		lb.file.Close()
		lb.file = nil
	}
	if lb.file == nil {
		filename := time.Now().Format(timeLayout)
		file, err := os.OpenFile(path.Join(lb.logDir, filename),
			os.O_CREATE|os.O_WRONLY, filePerms)
		if err != nil {
			return err
		}
		if !*alsoLogToStderr {
			syscall.Dup2(int(file.Fd()), int(os.Stdout.Fd()))
			syscall.Dup2(int(file.Fd()), int(os.Stderr.Fd()))
		}
		lb.file = file
		lb.writer = bufio.NewWriter(file)
		symlink := path.Join(lb.logDir, "latest")
		tmpSymlink := symlink + "~"
		os.Remove(tmpSymlink)
		os.Symlink(filename, tmpSymlink)
		os.Rename(tmpSymlink, symlink)
	}
	return nil
}

func (lb *LogBuffer) flushWhenIdle(writeNotifier <-chan struct{}) {
	flushTimer := time.NewTimer(time.Second)
	idleMarkDuration := *idleMarkTimeout
	if idleMarkDuration < 1 {
		idleMarkDuration = time.Hour * 24 * 365 * 280 // Far in the future.
	}
	idleMarkTimer := time.NewTimer(idleMarkDuration)
	for {
		select {
		case <-writeNotifier:
			flushTimer.Reset(time.Second)
			idleMarkTimer.Reset(idleMarkDuration)
		case <-flushTimer.C:
			lb.flush()
		case <-idleMarkTimer.C:
			lb.writeMark()
			flushTimer.Reset(time.Second)
			idleMarkTimer.Reset(idleMarkDuration)
		}
	}
}

func (lb *LogBuffer) getEntries() [][]byte {
	lb.rwMutex.RLock()
	defer lb.rwMutex.RUnlock()
	entries := make([][]byte, 0, lb.buffer.Len())
	lb.buffer.Do(func(p interface{}) {
		if p != nil {
			entries = append(entries, p.([]byte))
		}
	})
	return entries
}

func (lb *LogBuffer) dumpSince(writer io.Writer, name string,
	earliestTime time.Time, prefix, postfix string, recentFirst bool) error {
	file, err := os.Open(path.Join(lb.logDir, path.Base(path.Clean(name))))
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	lines := make([]string, 0)
	timeFormat := "2006/01/02 15:04:05"
	minLength := len(timeFormat) + 2
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) >= minLength {
			timeString := line[:minLength-2]
			timeStamp, err := time.ParseInLocation(timeFormat, timeString,
				time.Local)
			if err == nil && timeStamp.Before(earliestTime) {
				continue
			}
		}
		if recentFirst {
			lines = append(lines, line)
		} else {
			writer.Write([]byte(prefix))
			writer.Write([]byte(line))
			writer.Write([]byte(postfix))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if recentFirst {
		reverseStrings(lines)
		for _, line := range lines {
			writer.Write([]byte(prefix))
			writer.Write([]byte(line))
			writer.Write([]byte(postfix))
		}
	}
	return nil
}

func (lb *LogBuffer) writeMark() {
	now := time.Now()
	year, month, day := now.Date()
	hour, minute, second := now.Clock()
	str := fmt.Sprintf("%d/%02d/%02d %02d:%02d:%02d MARK\n",
		year, month, day, hour, minute, second)
	lb.rwMutex.Lock()
	defer lb.rwMutex.Unlock()
	lb.writeToLogFile([]byte(str))
}

func reverseEntries(entries [][]byte) {
	length := len(entries)
	for index := 0; index < length/2; index++ {
		entries[index], entries[length-1-index] =
			entries[length-1-index], entries[index]
	}
}

func reverseStrings(strings []string) {
	length := len(strings)
	for index := 0; index < length/2; index++ {
		strings[index], strings[length-1-index] =
			strings[length-1-index], strings[index]
	}
}

func init() {
	UseFlagSet(flag.CommandLine)
}
