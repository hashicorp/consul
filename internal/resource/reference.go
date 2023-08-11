// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import "github.com/hashicorp/consul/proto-public/pbresource"

// Reference returns a reference to the resource with the given ID.
func Reference(id *pbresource.ID, section string) *pbresource.Reference {
	return &pbresource.Reference{
		Type:    id.Type,
		Tenancy: id.Tenancy,
		Name:    id.Name,
		Section: section,
	}
}

// IDFromReference returns a Reference converted into an ID. NOTE: the UID
// field is not populated, and the Section field of a reference is dropped.
func IDFromReference(ref *pbresource.Reference) *pbresource.ID {
	return &pbresource.ID{
		Type:    ref.Type,
		Tenancy: ref.Tenancy,
		Name:    ref.Name,
	}
}

// ReferenceOrID is the common accessors shared by pbresource.Reference and
// pbresource.ID.
type ReferenceOrID interface {
	GetType() *pbresource.Type
	GetTenancy() *pbresource.Tenancy
	GetName() string
}

var (
	_ ReferenceOrID = (*pbresource.ID)(nil)
	_ ReferenceOrID = (*pbresource.Reference)(nil)
)
