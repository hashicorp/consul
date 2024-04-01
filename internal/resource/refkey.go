// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ReferenceKey is the pointer-free representation of a ReferenceOrID
// suitable for a go map key.
type ReferenceKey struct {
	GVK       string
	Partition string // Tenancy.*
	Namespace string // Tenancy.*
	// TODO(peering/v2) account for peer tenancy
	Name string
}

// String returns a string representation of the ReferenceKey. This should not
// be relied upon nor parsed and is provided just for debugging and logging
// reasons.
//
// This format should be aligned with IDToString and ReferenceToString.
func (r ReferenceKey) String() string {
	// TODO(peering/v2) account for peer tenancy
	return fmt.Sprintf("%s/%s.%s/%s",
		r.GVK,
		orDefault(r.Partition, "default"),
		orDefault(r.Namespace, "default"),
		r.Name,
	)
}

func (r ReferenceKey) GetTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Partition: r.Partition,
		Namespace: r.Namespace,
	}
}

// ToReference converts this back into a pbresource.ID.
func (r ReferenceKey) ToID() *pbresource.ID {
	return &pbresource.ID{
		Type:    GVKToType(r.GVK),
		Tenancy: r.GetTenancy(),
		Name:    r.Name,
	}
}

// ToReference converts this back into a pbresource.Reference.
func (r ReferenceKey) ToReference() *pbresource.Reference {
	return &pbresource.Reference{
		Type:    GVKToType(r.GVK),
		Tenancy: r.GetTenancy(),
		Name:    r.Name,
	}
}

func (r ReferenceKey) GoString() string { return r.String() }

func NewReferenceKey(refOrID ReferenceOrID) ReferenceKey {
	return ReferenceKey{
		GVK:       ToGVK(refOrID.GetType()),
		Partition: orDefault(refOrID.GetTenancy().GetPartition(), "default"),
		Namespace: orDefault(refOrID.GetTenancy().GetNamespace(), "default"),
		Name:      refOrID.GetName(),
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func GVKToType(gvk string) *pbresource.Type {
	parts := strings.Split(gvk, ".")
	if len(parts) != 3 {
		panic("bad gvk")
	}
	return &pbresource.Type{
		Group:        parts[0],
		GroupVersion: parts[1],
		Kind:         parts[2],
	}
}
