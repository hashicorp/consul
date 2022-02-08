//go:build windows
// +build windows

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

	stat, ok := fileInfo.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return 0, fmt.Errorf("not a syscall.Stat_t %v", fileInfo.Sys())
	}

	w.logger.Info("read CreationTime ", "CreationTime", stat.CreationTime.Nanoseconds())
	return uint64(stat.CreationTime.Nanoseconds()), nil
}
