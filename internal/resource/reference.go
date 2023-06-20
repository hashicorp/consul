// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
