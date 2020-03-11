// +build windows

package agent

// Getrlimit is no-op on Windows, as max fd/process is 2^24 on Wow64
// Return (16 777 216, nil)
func Getrlimit() (uint64, error) {
	return 16_777_216, nil
}
