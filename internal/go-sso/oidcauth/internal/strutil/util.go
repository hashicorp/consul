// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

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
