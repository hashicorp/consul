// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux && !darwin
// +build !linux,!darwin

package envoy

func execEnvoy(binary string, prefixArgs, suffixArgs []string, bootstrapJson []byte) error {
	return errUnsupportedOS
}
