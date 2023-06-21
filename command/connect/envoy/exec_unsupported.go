<<<<<<< HEAD
=======
// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

>>>>>>> southworks/qa-consul
//go:build !linux && !darwin && !windows
// +build !linux,!darwin,!windows

package envoy

func execEnvoy(binary string, prefixArgs, suffixArgs []string, bootstrapJson []byte) error {
	return errUnsupportedOS
}
