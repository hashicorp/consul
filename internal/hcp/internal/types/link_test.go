// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func createCloudLinkResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: pbhcp.LinkType,
			Name: "global",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestValidateLink_Ok(t *testing.T) {
	data := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   GenerateTestResourceID(t),
	}

	res := createCloudLinkResource(t, data)

	err := ValidateLink(res)
	require.NoError(t, err)
}

func TestValidateLink_ParseError(t *testing.T) {
	// Any type other than the Link type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createCloudLinkResource(t, data)

	err := ValidateLink(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateLink_InvalidName(t *testing.T) {
	data := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   GenerateTestResourceID(t),
	}

	res := createCloudLinkResource(t, data)
	res.Id.Name = "default"

	err := ValidateLink(res)

	expected := resource.ErrInvalidField{
		Name:    "name",
		Wrapped: errLinkConfigurationName,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateLink_MissingClientId(t *testing.T) {
	data := &pbhcp.Link{
		ClientId:     "",
		ClientSecret: "abc",
		ResourceId:   GenerateTestResourceID(t),
	}

	res := createCloudLinkResource(t, data)

	err := ValidateLink(res)

	expected := resource.ErrInvalidField{
		Name:    "client_id",
		Wrapped: resource.ErrMissing,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateLink_MissingClientSecret(t *testing.T) {
	data := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "",
		ResourceId:   GenerateTestResourceID(t),
	}

	res := createCloudLinkResource(t, data)

	err := ValidateLink(res)

	expected := resource.ErrInvalidField{
		Name:    "client_secret",
		Wrapped: resource.ErrMissing,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateLink_MissingResourceId(t *testing.T) {
	data := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "",
	}

	res := createCloudLinkResource(t, data)

	err := ValidateLink(res)

	expected := resource.ErrInvalidField{
		Name:    "resource_id",
		Wrapped: resource.ErrMissing,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateLink_InvalidResourceId(t *testing.T) {
	data := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}

	res := createCloudLinkResource(t, data)

	err := ValidateLink(res)

	expected := resource.ErrInvalidField{
		Name:    "resource_id",
		Wrapped: errInvalidHCPResourceID,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

// Currently, we have no specific ACLs configured so the default `operator` permissions are required
func TestLinkACLs(t *testing.T) {
	registry := resource.NewRegistry()
	RegisterLink(registry)

	data := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   GenerateTestResourceID(t),
	}
	link := createCloudLinkResource(t, data)

	cases := map[string]rtest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Res:     link,
			ReadOK:  rtest.DENY,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DENY,
		},
		"link test read": {
			Rules:   `operator = "read"`,
			Res:     link,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.DENY,
			ListOK:  rtest.ALLOW,
		},
		"link test write": {
			Rules:   `operator = "write"`,
			Res:     link,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.ALLOW,
			ListOK:  rtest.ALLOW,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rtest.RunACLTestCase(t, tc, registry)
		})
	}
}
