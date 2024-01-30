// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import "github.com/hashicorp/consul/proto-public/pbresource"

func LessReference(a, b *pbresource.Reference) bool {
	return compareReference(a, b) < 0
}

func compareReference(a, b *pbresource.Reference) int {
	if a == nil || b == nil {
		panic("nil references cannot be compared")
	}

	diff := compareType(a.Type, b.Type)
	if diff != 0 {
		return diff
	}
	diff = compareTenancy(a.Tenancy, b.Tenancy)
	if diff != 0 {
		return diff
	}
	diff = compareString(a.Name, b.Name)
	if diff != 0 {
		return diff
	}
	return compareString(a.Section, b.Section)
}

func compareType(a, b *pbresource.Type) int {
	if a == nil || b == nil {
		panic("nil types cannot be compared")
	}
	diff := compareString(a.Group, b.Group)
	if diff != 0 {
		return diff
	}
	diff = compareString(a.GroupVersion, b.GroupVersion)
	if diff != 0 {
		return diff
	}
	return compareString(a.Kind, b.Kind)
}

func compareTenancy(a, b *pbresource.Tenancy) int {
	if a == nil || b == nil {
		panic("nil tenancies cannot be compared")
	}
	diff := compareString(a.Partition, b.Partition)
	if diff != 0 {
		return diff
	}
	return compareString(a.Namespace, b.Namespace)
}

func compareString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
