// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import "math/rand"

type RecursorStrategy string

const (
	RecursorStrategySequential RecursorStrategy = "sequential"
	RecursorStrategyRandom     RecursorStrategy = "random"
)

func (s RecursorStrategy) Indexes(max int) []int {
	switch s {
	case RecursorStrategyRandom:
		return rand.Perm(max)
	default:
		idxs := make([]int, max)
		for i := range idxs {
			idxs[i] = i
		}
		return idxs

	}
}
