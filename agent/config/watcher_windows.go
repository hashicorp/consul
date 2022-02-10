//go:build windows
// +build windows

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

	stat, ok := fileInfo.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return 0, fmt.Errorf("not a syscall.Stat_t %v", fileInfo.Sys())
	}

	w.logger.Debug("read CreationTime ", "CreationTime", stat.CreationTime.Nanoseconds())
	return uint64(stat.CreationTime.Nanoseconds()), nil
}
