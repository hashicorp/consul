//go:build !windows
// +build !windows

package config

import (
	"fmt"
	"os"
	"syscall"
)

func (w Watcher) getFileId(filename string) (uint64, error) {
	realFilename := filename
	if linkedFile, err := os.Readlink(filename); err == nil {
		realFilename = linkedFile
	}
	fileInfo, err := os.Stat(realFilename)
	if err != nil {
		return 0, err
	}

	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("not a syscall.Stat_t %v", fileInfo.Sys())
	}

	w.logger.Info("read inode ", "inode", stat.Ino)
	return stat.Ino, nil
}
