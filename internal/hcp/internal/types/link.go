// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-uuid"
	hcpresource "github.com/hashicorp/hcp-sdk-go/resource"
	"github.com/stretchr/testify/require"
)

type DecodedLink = resource.DecodedResource[*pbhcp.Link]

const (
	LinkName             = "global"
	MetadataSourceKey    = "source"
	MetadataSourceConfig = "config"
)

var (
	errLinkConfigurationName = errors.New("only a single Link resource is allowed and it must be named global")
	errInvalidHCPResourceID  = errors.New("could not parse, invalid format")
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
			Wrapped: errLinkConfigurationName,
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
	} else {
		_, parseErr := hcpresource.FromString(res.Data.ResourceId)
		if parseErr != nil {
			err = multierror.Append(err, resource.ErrInvalidField{
				Name:    "resource_id",
				Wrapped: errInvalidHCPResourceID,
			})
		}
	}

	return err
}

func GenerateTestResourceID(t *testing.T) string {
	t.Helper()

	orgID, err := uuid.GenerateUUID()
	require.NoError(t, err)

	projectID, err := uuid.GenerateUUID()
	require.NoError(t, err)

	template := "organization/%s/project/%s/hashicorp.consul.global-network-manager.cluster/test-cluster"
	return fmt.Sprintf(template, orgID, projectID)
}
