//go:build !linux
// +build !linux

package xds

func kernelSupportsIPv6() (bool, error) {
	return true, nil
}
