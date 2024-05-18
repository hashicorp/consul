// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalogv2beta1

// IsEmpty returns true if a config has no definition.
func (x *FailoverConfig) IsEmpty() bool {
	if x == nil {
		return true
	}
	return len(x.Destinations) == 0 &&
		x.Mode == 0 &&
		len(x.Regions) == 0 &&
		x.SamenessGroup == ""
}
