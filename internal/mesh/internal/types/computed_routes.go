// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ComputedRoutesKind = "ComputedRoutes"

	// NullRouteBackend is the sentinel string used in ComputedRoutes backend
	// targets to indicate that traffic arriving at this destination should
	// fail in a protocol-specific way (i.e. HTTP is 5xx)
	NullRouteBackend = "NULL-ROUTE"
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

		for targetName, target := range pmc.Targets {
			wrapTargetErr := func(err error) error {
				return wrapErr(resource.ErrInvalidMapValue{
					Map:     "targets",
					Key:     targetName,
					Wrapped: err,
				})
			}
			if target.MeshPort == "" {
				merr = multierror.Append(merr, wrapTargetErr(resource.ErrInvalidField{
					Name:    "mesh_port",
					Wrapped: resource.ErrEmpty,
				}))
			}
			if target.ServiceEndpointsId != nil {
				merr = multierror.Append(merr, wrapTargetErr(resource.ErrInvalidField{
					Name:    "service_endpoints_id",
					Wrapped: fmt.Errorf("field should be empty"),
				}))
			}
			if target.ServiceEndpoints != nil {
				merr = multierror.Append(merr, wrapTargetErr(resource.ErrInvalidField{
					Name:    "service_endpoints",
					Wrapped: fmt.Errorf("field should be empty"),
				}))
			}
			if len(target.IdentityRefs) > 0 {
				merr = multierror.Append(merr, wrapTargetErr(resource.ErrInvalidField{
					Name:    "identity_refs",
					Wrapped: fmt.Errorf("field should be empty"),
				}))
			}
		}

		// TODO(rb): do a deep inspection of the config to verify that all
		// xRoute backends ultimately point to an item in the targets map.
	}

	return merr
}
