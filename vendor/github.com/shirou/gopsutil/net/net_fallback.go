// +build !darwin,!linux,!freebsd,!openbsd,!windows

package net

import "github.com/shirou/gopsutil/internal/common"

func IOCounters(pernic bool) ([]IOCountersStat, error) {
	return []IOCountersStat{}, common.ErrNotImplementedError
}

func FilterCounters() ([]FilterStat, error) {
	return []FilterStat{}, common.ErrNotImplementedError
}

func ProtoCounters(protocols []string) ([]ProtoCountersStat, error) {
	return []ProtoCountersStat{}, common.ErrNotImplementedError
}

func Connections(kind string) ([]ConnectionStat, error) {
	return []ConnectionStat{}, common.ErrNotImplementedError
}

func ConnectionsMax(kind string, max int) ([]ConnectionStat, error) {
	return []ConnectionStat{}, common.ErrNotImplementedError
}
