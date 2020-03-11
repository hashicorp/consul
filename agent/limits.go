// +build !windows

package agent

import "golang.org/x/sys/unix"

// Getrlimit return the max file descriptors allocated by system
// return the number of file descriptors max
func Getrlimit() (uint64, error) {
	var limit unix.Rlimit
	err := unix.Getrlimit(unix.RLIMIT_NOFILE, &limit)
	return uint64(limit.Cur), err
}
