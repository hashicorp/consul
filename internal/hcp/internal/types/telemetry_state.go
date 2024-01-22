// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
)

type DecodedTelemetryState = resource.DecodedResource[*pbhcp.TelemetryState]

var (
	telemetryStateConfigurationNameError = errors.New("only a single Telemetry resource is allowed and it must be named global")
)

func RegisterTelemetryState(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbhcp.TelemetryStateType,
		Proto:    &pbhcp.TelemetryState{},
		Scope:    resource.ScopeCluster,
		Validate: ValidateTelemetryState,
	})
}

var ValidateTelemetryState = resource.DecodeAndValidate(validateTelemetryState)

func validateTelemetryState(res *DecodedTelemetryState) error {
	var err error

	if res.Id.Name != "global" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "name",
			Wrapped: telemetryStateConfigurationNameError,
		})
	}

	if res.Data.ClientId == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "client_id",
			Wrapped: resource.ErrMissing,
		})
	}

	if res.Data.ClientSecret == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "client_secret",
			Wrapped: resource.ErrMissing,
		})
	}

	if res.Data.ResourceId == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "resource_id",
			Wrapped: resource.ErrMissing,
		})
	}

	return err
}
