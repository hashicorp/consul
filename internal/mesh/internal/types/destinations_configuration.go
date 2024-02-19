// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/catalog/workloadselector"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func RegisterDestinationsConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.DestinationsConfigurationType,
		Proto:    &pbmesh.DestinationsConfiguration{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateDestinationsConfiguration,
		ACLs:     workloadselector.ACLHooks[*pbmesh.DestinationsConfiguration](),
	})
}

var ValidateDestinationsConfiguration = resource.DecodeAndValidate(validateDestinationsConfiguration)

func validateDestinationsConfiguration(res *DecodedDestinationsConfiguration) error {
	var merr error

	// Validate the workload selector
	if selErr := catalog.ValidateSelector(res.Data.Workloads, false); selErr != nil {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "workloads",
			Wrapped: selErr,
		})
	}

	return merr
}
