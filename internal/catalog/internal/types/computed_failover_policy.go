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
	if res.Data.Config != nil {
		return fmt.Errorf(`invalid "config" field: computed failover policy cannot have a config`)
	}
	for _, fc := range res.Data.PortConfigs {
		if fc.GetSamenessGroup() != "" {
			return fmt.Errorf(`invalid "config" field: computed failover policy cannot have a sameness_group`)
		}
	}
	return validateCommonFailoverConfigs(&pbcatalog.FailoverPolicy{
		Config:      res.Data.Config,
		PortConfigs: res.Data.PortConfigs,
	})
}

func aclWriteHookComputedFailoverPolicy(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedComputedFailoverPolicy) error {
	// FailoverPolicy is name-aligned with Service
	serviceName := res.Id.Name

	// Check service:write permissions on the service this is controlling.
	if err := authorizer.ToAllowAuthorizer().ServiceWriteAllowed(serviceName, authzContext); err != nil {
		return err
	}

	// Ensure you have service:read on any destination that may be affected by
	// traffic FROM this config change.
	if res.Data.Config != nil {
		for _, dest := range res.Data.Config.Destinations {
			destAuthzContext := resource.AuthorizerContext(dest.Ref.GetTenancy())
			destServiceName := dest.Ref.GetName()
			if err := authorizer.ToAllowAuthorizer().ServiceReadAllowed(destServiceName, destAuthzContext); err != nil {
				return err
			}
		}
	}
	for _, pc := range res.Data.PortConfigs {
		for _, dest := range pc.Destinations {
			destAuthzContext := resource.AuthorizerContext(dest.Ref.GetTenancy())
			destServiceName := dest.Ref.GetName()
			if err := authorizer.ToAllowAuthorizer().ServiceReadAllowed(destServiceName, destAuthzContext); err != nil {
				return err
			}
		}
	}

	return nil
}
