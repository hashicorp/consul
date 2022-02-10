//go:build !windows
// +build !windows

package config

import (
	"fmt"
	"os"
	"syscall"
)

func (w *FileWatcher) getFileId(filename string) (uint64, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}

	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("not a syscall.Stat_t %v", fileInfo.Sys())
	}

	w.logger.Debug("read inode ", "inode", stat.Ino)
	return stat.Ino, nil
}
