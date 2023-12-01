// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"

	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v1"
	"github.com/hashicorp/go-multierror"
)

type DecodedCloudLink = resource.DecodedResource[*pbhcp.HCCLink]

var (
	hccLinkConfigurationNameError = errors.New("only a single HCCLink resource is allowed and it must be named global")
)

func RegisterHCCLink(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbhcp.HCCLinkType,
		Proto:    &pbhcp.HCCLink{},
		Scope:    resource.ScopeCluster,
		Validate: ValidateHCCLink,
	})
}

var ValidateHCCLink = resource.DecodeAndValidate(validateHCCLink)

func validateHCCLink(res *DecodedCloudLink) error {
	var err error

	if res.Id.Name != "global" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "name",
			Wrapped: hccLinkConfigurationNameError,
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
