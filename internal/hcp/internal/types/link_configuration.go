// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"

	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	linkConfigurationNameError = errors.New("Only a single HCP LinkConfiguration resource is allowed and it must be named hcp-link")
)

func RegisterLinkConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbhcp.LinkConfigurationType,
		Proto:    &pbhcp.LinkConfiguration{},
		Scope:    resource.ScopeCluster,
		Validate: ValidateLinkConfiguration,
	})
}

func ValidateLinkConfiguration(res *pbresource.Resource) error {
	var linkConfig pbhcp.LinkConfiguration

	if err := res.Data.UnmarshalTo(&linkConfig); err != nil {
		return resource.NewErrDataParse(&linkConfig, err)
	}

	if res.Id.Name != "hcp-link" {
		return resource.ErrInvalidField{
			Name: "id",
			Wrapped: resource.ErrInvalidField{
				Name:    "name",
				Wrapped: linkConfigurationNameError,
			},
		}
	}

	// perform other LinkConfiguration validations here

	return nil
}
