// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
)

const (
	NodeKind = "Node"
)

var (
	NodeV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         NodeKind,
	}

	NodeType = NodeV1Alpha1Type
)

func RegisterNode(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  NodeV1Alpha1Type,
		Proto: &pbcatalog.Node{},
		// TODO: A node should be partition scoped. However its HealthStatus which is
		// namespace scoped has Node as an owner. We do not support ownership between resources
		// of differing scope at this time. HealthStatus will probably be split out into two different
		// types, one for namespace scoped owners and the other for partition scoped owners.
		// Until that time, Node will remain namespace scoped.
		Scope:    resource.ScopeNamespace,
		Validate: ValidateNode,
	})
}

func ValidateNode(res *pbresource.Resource) error {
	var node pbcatalog.Node

	if err := res.Data.UnmarshalTo(&node); err != nil {
		return resource.NewErrDataParse(&node, err)
	}

	var err error
	// Validate that the node has at least 1 address
	if len(node.Addresses) < 1 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "addresses",
			Wrapped: resource.ErrEmpty,
		})
	}

	// Validate each node address
	for idx, addr := range node.Addresses {
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
