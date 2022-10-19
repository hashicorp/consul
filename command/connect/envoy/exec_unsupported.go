//go:build !linux && !darwin && !windows
// +build !linux,!darwin,!windows

package envoy

func execEnvoy(binary string, prefixArgs, suffixArgs []string, bootstrapJson []byte) error {
	return errUnsupportedOS
}
