//go:build windows
// +build windows

package freeport

func systemLimit() (int, error) {
	return 0, nil
}
