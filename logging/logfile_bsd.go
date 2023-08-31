//go:build darwin || freebsd || netbsd || openbsd
// +build darwin freebsd netbsd openbsd

package logging

import (
	"os"
	"syscall"
	"time"
)

func (l *LogFile) createTime(stat os.FileInfo) time.Time {
	stat_t := stat.Sys().(*syscall.Stat_t)
	createTime := stat_t.Ctimespec
	return time.Unix(createTime.Sec, createTime.Nsec)
}
