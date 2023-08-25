// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package flags

import "flag"

func Merge(dst, src *flag.FlagSet) {
	if dst == nil {
		panic("dst cannot be nil")
	}
	if src == nil {
		return
	}
	src.VisitAll(func(f *flag.Flag) {
		dst.Var(f.Value, f.Name, f.Usage)
	})
}
