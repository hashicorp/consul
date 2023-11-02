// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type DecodedVirtualIPs = resource.DecodedResource[*pbcatalog.VirtualIPs]

func RegisterVirtualIPs(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.VirtualIPsType,
		Proto:    &pbcatalog.VirtualIPs{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateVirtualIPs,
		ACLs: &resource.ACLHooks{
			Read: func(authorizer acl.Authorizer, context *acl.AuthorizerContext, id *pbresource.ID, p *pbresource.Resource) error {
				return authorizer.ToAllowAuthorizer().ServiceReadAllowed(id.GetName(), context)
			},
			Write: func(authorizer acl.Authorizer, context *acl.AuthorizerContext, p *pbresource.Resource) error {
				return authorizer.ToAllowAuthorizer().ServiceWriteAllowed(p.GetId().GetName(), context)
			},
			List: resource.NoOpACLListHook,
		},
	})
}

var ValidateVirtualIPs = resource.DecodeAndValidate(validateVirtualIPs)

func validateVirtualIPs(res *DecodedVirtualIPs) error {
	var err error
	for idx, ip := range res.Data.Ips {
		if vipErr := validateIPAddress(ip.Address); vipErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:  "ips",
				Index: idx,
				Wrapped: resource.ErrInvalidField{
					Name:    "address",
					Wrapped: vipErr,
				},
			})
		}
	}
	return err
}
