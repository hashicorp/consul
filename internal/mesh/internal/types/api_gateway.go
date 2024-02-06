// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func RegisterAPIGateway(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.APIGatewayType,
		Proto:    &pbmesh.APIGateway{},
		Scope:    resource.ScopeNamespace,
		ACLs:     nil, // TODO NET-7289
		Mutate:   nil, // TODO NET-7617
		Validate: nil, // TODO NET-7618
	})
}
