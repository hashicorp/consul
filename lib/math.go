// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lib

func AbsInt(a int) int {
	if a > 0 {
		return a
	}
	return a * -1
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MinInt(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func MaxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
