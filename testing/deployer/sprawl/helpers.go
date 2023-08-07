// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sprawl

// Deprecated: remove
func TruncateSquidError(err error) error {
	return err
}

// Deprecated: remove
func IsSquid503(err error) bool {
	return false
}
