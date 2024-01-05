// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pbacl

import (
	"github.com/hashicorp/consul/api"
)

func (a *ACLLink) ToAPI() api.ACLLink {
	return api.ACLLink{
		ID:   a.ID,
		Name: a.Name,
	}
}

func ACLLinkFromAPI(a api.ACLLink) *ACLLink {
	return &ACLLink{
		ID:   a.ID,
		Name: a.Name,
	}
}
