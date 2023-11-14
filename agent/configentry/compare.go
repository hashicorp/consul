// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"sort"

	"github.com/hashicorp/consul/agent/structs"
)

func SortSlice(configs []structs.ConfigEntry) {
	sort.SliceStable(configs, func(i, j int) bool {
		return Less(configs[i], configs[j])
	})
}

func Less(first structs.ConfigEntry, second structs.ConfigEntry) bool {
	if first.GetKind() < second.GetKind() {
		return true
	}
	if first.GetKind() > second.GetKind() {
		return false
	}

	if first.GetEnterpriseMeta().LessThan(second.GetEnterpriseMeta()) {
		return true
	}
	if second.GetEnterpriseMeta().LessThan(first.GetEnterpriseMeta()) {
		return false
	}

	return first.GetName() < second.GetName()
}

func EqualID(e1, e2 structs.ConfigEntry) bool {
	return e1.GetKind() == e2.GetKind() &&
		e1.GetEnterpriseMeta().IsSame(e2.GetEnterpriseMeta()) &&
		e1.GetName() == e2.GetName()
}
