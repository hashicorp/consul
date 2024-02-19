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

type DecodedNode = resource.DecodedResource[*pbcatalog.Node]

func RegisterNode(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.NodeType,
		Proto:    &pbcatalog.Node{},
		Scope:    resource.ScopePartition,
		Validate: ValidateNode,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookNode,
			Write: aclWriteHookNode,
			List:  resource.NoOpACLListHook,
		},
	})
}

var ValidateNode = resource.DecodeAndValidate(validateNode)

func validateNode(res *DecodedNode) error {
	var err error
	// Validate that the node has at least 1 address
	if len(res.Data.Addresses) < 1 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "addresses",
			Wrapped: resource.ErrEmpty,
		})
	}

	// Validate each node address
	for idx, addr := range res.Data.Addresses {
		if addrErr := validateNodeAddress(addr); addrErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "addresses",
				Index:   idx,
				Wrapped: addrErr,
			})
		}
	}

	return err
}

// Validates a single node address
func validateNodeAddress(addr *pbcatalog.NodeAddress) error {
	// Currently the only field needing validation is the Host field. If that
	// changes then this func should get updated to use a multierror.Append
	// to collect the errors and return the overall set.

	// Check that the host is empty
	if addr.Host == "" {
		return resource.ErrInvalidField{
			Name:    "host",
			Wrapped: resource.ErrMissing,
		}
	}

	// Check if the host represents an IP address or a DNS name. Unix socket paths
	// are not allowed for Node addresses unlike workload addresses.
	if !isValidIPAddress(addr.Host) && !isValidDNSName(addr.Host) {
		return resource.ErrInvalidField{
			Name:    "host",
			Wrapped: errInvalidNodeHostFormat{Host: addr.Host},
		}
	}

	return nil
}

func aclReadHookNode(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().NodeReadAllowed(id.GetName(), authzContext)
}

func aclWriteHookNode(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().NodeWriteAllowed(res.GetId().GetName(), authzContext)
}
