// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	// NullRouteBackend is the sentinel string used in ComputedRoutes backend
	// targets to indicate that traffic arriving at this destination should
	// fail in a protocol-specific way (i.e. HTTP is 5xx)
	NullRouteBackend = "NULL-ROUTE"
)

func RegisterComputedRoutes(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.ComputedRoutesType,
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

			switch target.Type {
			case pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_UNSPECIFIED:
				merr = multierror.Append(merr, wrapTargetErr(
					resource.ErrInvalidField{
						Name:    "type",
						Wrapped: resource.ErrMissing,
					}),
				)
			case pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT:
			case pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_INDIRECT:
				if target.FailoverConfig != nil {
					merr = multierror.Append(merr, wrapTargetErr(
						resource.ErrInvalidField{
							Name:    "failover_config",
							Wrapped: errors.New("failover_config not supported for type = INDIRECT"),
						}),
					)
				}
			default:
				merr = multierror.Append(merr, wrapTargetErr(
					resource.ErrInvalidField{
						Name:    "type",
						Wrapped: fmt.Errorf("not a supported enum value: %v", target.Type),
					},
				))
			}

			if target.DestinationConfig == nil {
				merr = multierror.Append(merr, wrapTargetErr(resource.ErrInvalidField{
					Name:    "destination_config",
					Wrapped: resource.ErrMissing,
				}))
			} else {
				wrapDestConfigErr := func(err error) error {
					return wrapTargetErr(resource.ErrInvalidField{
						Name:    "destination_config",
						Wrapped: err,
					})
				}

				destConfig := target.DestinationConfig
				if destConfig.ConnectTimeout == nil {
					merr = multierror.Append(merr, wrapDestConfigErr(resource.ErrInvalidField{
						Name:    "connect_timeout",
						Wrapped: resource.ErrMissing,
					}))
				} else {
					connectTimeout := destConfig.ConnectTimeout.AsDuration()
					if connectTimeout < 0 {
						merr = multierror.Append(merr, wrapDestConfigErr(resource.ErrInvalidField{
							Name:    "connect_timeout",
							Wrapped: errTimeoutCannotBeNegative(connectTimeout),
						}))
					}
				}
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
