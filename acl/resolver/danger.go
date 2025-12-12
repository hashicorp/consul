// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package resolver

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// DANGER_NO_AUTH implements an ACL resolver short-circuit authorization in
// cases where it is handled somewhere else or expressly not required.
type DANGER_NO_AUTH struct{}

// ResolveTokenAndDefaultMeta returns an authorizer with unfettered permissions.
func (DANGER_NO_AUTH) ResolveTokenAndDefaultMeta(_ string, entMeta *acl.EnterpriseMeta, _ *acl.AuthorizerContext) (Result, error) {
	entMeta.Merge(structs.DefaultEnterpriseMetaInDefaultPartition())
	return Result{Authorizer: acl.ManageAll()}, nil
}
