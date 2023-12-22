// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
)

type DecodedComputedFailoverPolicy = resource.DecodedResource[*pbcatalog.ComputedFailoverPolicy]

func RegisterComputedFailoverPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.ComputedFailoverPolicyType,
		Proto:    &pbcatalog.ComputedFailoverPolicy{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateComputedFailoverPolicy,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookFailoverPolicy,
			Write: resource.DecodeAndAuthorizeWrite(aclWriteHookComputedFailoverPolicy),
			List:  resource.NoOpACLListHook,
		},
	})
}

var ValidateComputedFailoverPolicy = resource.DecodeAndValidate(validateComputedFailoverPolicy)

func validateComputedFailoverPolicy(res *DecodedComputedFailoverPolicy) error {
	if res.Data.Config != nil && res.Data.Config.SamenessGroup != "" {
		return fmt.Errorf(`invalid "config" field: computed failover policy cannot have a sameness_group`)
	}
	for _, fc := range res.Data.PortConfigs {
		if fc.GetSamenessGroup() != "" {
			return fmt.Errorf(`invalid "config" field: computed failover policy cannot have a sameness_group`)
		}
	}
	dfp := convertToDecodedFailoverPolicy(res)
	return validateFailoverPolicy(dfp)
}

func aclWriteHookComputedFailoverPolicy(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedComputedFailoverPolicy) error {
	dfp := convertToDecodedFailoverPolicy(res)
	return aclWriteHookFailoverPolicy(authorizer, authzContext, dfp)
}

func convertToDecodedFailoverPolicy(res *DecodedComputedFailoverPolicy) *DecodedFailoverPolicy {
	dfp := &DecodedFailoverPolicy{}
	dfp.Data = (*pbcatalog.FailoverPolicy)(res.GetData())
	dfp.Resource = res.GetResource()
	return dfp
}
