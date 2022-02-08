//go:build windows
// +build windows

package config

import (
	"fmt"
	"os"
	"syscall"
)

func (w Watcher) getInode(filename string) (uint64, error) {
	realFilename := filename
	if linkedFile, err := os.Readlink(filename); err == nil {
		realFilename = linkedFile
	}
	fileInfo, err := os.Stat(realFilename)
	if err != nil {
		return 0, err
	}

	stat := fileinfo.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return 0, fmt.Errorf("not a syscall.Stat_t %v", fileInfo.Sys())
	}

	w.logger.Info("read inode ", "inode", stat.Ino)
	return time.Since(time.Unix(0, stat.CreationTime.Nanoseconds())), nil
}
