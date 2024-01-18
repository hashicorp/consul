// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
)

type DecodedLink = resource.DecodedResource[*pbhcp.Link]

const (
	LinkName             = "global"
	MetadataSourceKey    = "source"
	MetadataSourceConfig = "config"
)

var (
	linkConfigurationNameError = errors.New("only a single Link resource is allowed and it must be named global")
)

func RegisterLink(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbhcp.LinkType,
		Proto:    &pbhcp.Link{},
		Scope:    resource.ScopeCluster,
		Validate: ValidateLink,
	})
}

var ValidateLink = resource.DecodeAndValidate(validateLink)

func validateLink(res *DecodedLink) error {
	var err error

	if res.Id.Name != LinkName {
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
