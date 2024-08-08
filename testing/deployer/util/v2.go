// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// functions copied out of consul:internal/resource/*.go

// IDToString returns a string representation of pbresource.ID. This should not
// be relied upon nor parsed and is provided just for debugging and logging
// reasons.
//
// This format should be aligned with ReferenceToString and
// (ReferenceKey).String.
func IDToString(id *pbresource.ID) string {
	s := fmt.Sprintf("%s/%s/%s",
		TypeToString(id.Type),
		TenancyToString(id.Tenancy),
		id.Name,
	)
	if id.Uid != "" {
		return s + "?uid=" + id.Uid
	}
	return s
}

// ReferenceToString returns a string representation of pbresource.Reference.
// This should not be relied upon nor parsed and is provided just for debugging
// and logging reasons.
//
// This format should be aligned with IDToString and (ReferenceKey).String.
func ReferenceToString(ref *pbresource.Reference) string {
	s := fmt.Sprintf("%s/%s/%s",
		TypeToString(ref.Type),
		TenancyToString(ref.Tenancy),
		ref.Name,
	)

	if ref.Section != "" {
		return s + "?section=" + ref.Section
	}
	return s
}

// TenancyToString returns a string representation of pbresource.Tenancy. This
// should not be relied upon nor parsed and is provided just for debugging and
// logging reasons.
func TenancyToString(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s.%s", tenancy.Partition, tenancy.Namespace)
}

// TypeToString returns a string representation of pbresource.Type. This should
// not be relied upon nor parsed and is provided just for debugging and logging
// reasons.
func TypeToString(typ *pbresource.Type) string {
	return ToGVK(typ)
}

func ToGVK(resourceType *pbresource.Type) string {
	return fmt.Sprintf("%s.%s.%s", resourceType.Group, resourceType.GroupVersion, resourceType.Kind)
}

// EqualType compares two resource types for equality without reflection.
func EqualType(a, b *pbresource.Type) bool {
	if a == b {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return a.Group == b.Group &&
		a.GroupVersion == b.GroupVersion &&
		a.Kind == b.Kind
}

func IsTypePartitionScoped(typ *pbresource.Type) bool {
	return false
}
