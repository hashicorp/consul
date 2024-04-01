// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resolver

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

type Result struct {
	acl.Authorizer
	// TODO: likely we can reduce this interface
	ACLIdentity structs.ACLIdentity
}

func (a Result) AccessorID() string {
	if a.ACLIdentity == nil {
		return ""
	}
	return a.ACLIdentity.ID()
}

func (a Result) Identity() structs.ACLIdentity {
	return a.ACLIdentity
}

func (a Result) ToAllowAuthorizer() acl.AllowAuthorizer {
	return acl.AllowAuthorizer{Authorizer: a, AccessorID: a.AccessorID()}
}
