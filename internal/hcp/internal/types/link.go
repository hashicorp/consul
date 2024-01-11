// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"

	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/go-multierror"
)

type DecodedLink = resource.DecodedResource[*pbhcp.Link]

var (
	linkConfigurationNameError = errors.New("only a single Link resource is allowed and it must be named global")
)

func RegisterLink(r resource.Registry) {
	r.Register(resource.RegisterRequest{
		Proto:    &pbhcp.Link{},
		Validate: ValidateLink,
	})
}

var ValidateLink = resource.DecodeAndValidate(validateLink)

func validateLink(res *DecodedLink) error {
	var err error

	if res.Id.Name != "global" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "name",
			Wrapped: linkConfigurationNameError,
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
