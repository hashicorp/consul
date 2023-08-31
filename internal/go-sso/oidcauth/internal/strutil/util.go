// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package strutil

// StrListContains looks for a string in a list of strings.
func StrListContains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
