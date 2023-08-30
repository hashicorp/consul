// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package stringslice

// StrContains => Contains
// StringSliceEqual => Equal
// StringSliceMergeSorted => MergeSorted

// Contains checks if a list contains a string
func Contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

// Equal compares two string slices for equality. Both the existence
// of the elements and the order of those elements matter for equality. Empty
// slices are treated identically to nil slices.
func Equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// MergeSorted takes two string slices that are assumed to be sorted
// and does a zipper merge of the two sorted slices, removing any cross-slice
// duplicates. If any individual slice contained duplicates those will be
// retained.
func MergeSorted(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	} else if len(a) == 0 {
		return b
	} else if len(b) == 0 {
		return a
	}

	out := make([]string, 0, len(a)+len(b))

	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			out = append(out, a[i])
			i++
		case a[i] > b[j]:
			out = append(out, b[j])
			j++
		default:
			out = append(out, a[i])
			i++
			j++
		}
	}
	if i < len(a) {
		out = append(out, a[i:]...)
	}
	if j < len(b) {
		out = append(out, b[j:]...)
	}
	return out
}

func CloneStringSlice(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}
