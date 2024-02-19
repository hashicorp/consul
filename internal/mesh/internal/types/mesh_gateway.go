// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/meshgateways"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func RegisterMeshGateway(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.MeshGatewayType,
		Proto:    &pbmesh.MeshGateway{},
		Scope:    resource.ScopePartition,
		ACLs:     nil, // TODO NET-6416
		Mutate:   nil, // TODO NET-6418
		Validate: resource.DecodeAndValidate(validateMeshGateway),
	})
}

func validateMeshGateway(res *DecodedMeshGateway) error {
	var merr error

	if res.GetId().GetName() != meshgateways.GatewayName {
		merr = multierror.Append(merr, fmt.Errorf("invalid gateway name, must be %q", meshgateways.GatewayName))
	}

	if len(res.GetData().Listeners) != 1 {
		merr = multierror.Append(merr, errors.New("invalid listeners, must have exactly one listener"))
	}

	if len(res.GetData().Listeners) > 0 && (res.GetData().Listeners[0].GetName() != meshgateways.WANPortName) {
		merr = multierror.Append(merr, fmt.Errorf("invalid listener name, must be %q", meshgateways.WANPortName))
	}

	return merr
}
