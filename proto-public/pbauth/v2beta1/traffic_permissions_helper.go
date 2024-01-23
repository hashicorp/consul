// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package authv2beta1

func (ctp *TrafficPermissions) HasReferencedSamenessGroups() bool {
	for _, dp := range ctp.Permissions {
		for _, source := range dp.Sources {
			if source.SamenessGroup != "" {
				return true
			}
		}
	}
	return false
}
