// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/catalog"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterDestinationsConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.DestinationsConfigurationType,
		Proto:    &pbmesh.DestinationsConfiguration{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateDestinationsConfiguration,
		ACLs:     catalog.ACLHooksForWorkloadSelectingType[*pbmesh.DestinationsConfiguration](),
	})
}

func ValidateDestinationsConfiguration(res *pbresource.Resource) error {
	var cfg pbmesh.DestinationsConfiguration

	if err := res.Data.UnmarshalTo(&cfg); err != nil {
		return resource.NewErrDataParse(&cfg, err)
	}

	var merr error

	// Validate the workload selector
	if selErr := catalog.ValidateSelector(cfg.Workloads, false); selErr != nil {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "workloads",
			Wrapped: selErr,
		})
	}

	return merr
}
