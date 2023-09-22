// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	NodeKind = "Node"
)

var (
	NodeV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         NodeKind,
	}

	NodeType = NodeV2Beta1Type
)

func RegisterNode(r resource.Registry) {
	r.Register(resource.Registration{
		Proto:    &pbcatalog.Node{},
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
