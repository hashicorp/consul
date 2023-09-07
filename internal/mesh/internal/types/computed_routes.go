// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ComputedRoutesKind = "ComputedRoutes"
)

var (
	ComputedRoutesV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ComputedRoutesKind,
	}

	ComputedRoutesType = ComputedRoutesV1Alpha1Type
)

func RegisterComputedRoutes(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     ComputedRoutesV1Alpha1Type,
		Proto:    &pbmesh.ComputedRoutes{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateComputedRoutes,
	})
}

func ValidateComputedRoutes(res *pbresource.Resource) error {
	var config pbmesh.ComputedRoutes

	if err := res.Data.UnmarshalTo(&config); err != nil {
		return resource.NewErrDataParse(&config, err)
	}

	var merr error

	if len(config.PortedConfigs) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "ported_configs",
			Wrapped: resource.ErrEmpty,
		})
	}

	// TODO(rb): do more elaborate validation

	for port, pmc := range config.PortedConfigs {
		wrapErr := func(err error) error {
			return resource.ErrInvalidMapValue{
				Map:     "ported_configs",
				Key:     port,
				Wrapped: err,
			}
		}
		if pmc.Config == nil {
			merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
				Name:    "config",
				Wrapped: resource.ErrEmpty,
			}))
		}
		if len(pmc.Targets) == 0 {
			merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
				Name:    "targets",
				Wrapped: resource.ErrEmpty,
			}))
		}
	}

	return merr
}
