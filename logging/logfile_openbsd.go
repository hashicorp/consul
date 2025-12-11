// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build openbsd

package logging

import (
	"os"
	"syscall"
	"time"
)

func (l *LogFile) createTime(stat os.FileInfo) time.Time {
	stat_t := stat.Sys().(*syscall.Stat_t)
	createTime := stat_t.Ctim
	return time.Unix(int64(createTime.Sec), int64(createTime.Nsec)) //nolint:unconvert
}
