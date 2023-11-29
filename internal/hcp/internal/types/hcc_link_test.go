package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func createCloudLinkResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: pbhcp.HCCLinkType,
			Name: "global",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestValidateHCCLink_Ok(t *testing.T) {
	data := &pbhcp.HCCLink{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}

	res := createCloudLinkResource(t, data)

	err := ValidateHCCLink(res)
	require.NoError(t, err)
}

func TestValidateHCCLink_ParseError(t *testing.T) {
	// Any type other than the HCCLink type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createCloudLinkResource(t, data)

	err := ValidateHCCLink(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateHCCLink_InvalidName(t *testing.T) {
	data := &pbhcp.HCCLink{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}

	res := createCloudLinkResource(t, data)
	res.Id.Name = "default"

	err := ValidateHCCLink(res)

	expected := resource.ErrInvalidField{
		Name:    "name",
		Wrapped: hccLinkConfigurationNameError,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateHCCLink_MissingClientId(t *testing.T) {
	data := &pbhcp.HCCLink{
		ClientId:     "",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}

	res := createCloudLinkResource(t, data)

	err := ValidateHCCLink(res)

	expected := resource.ErrInvalidField{
		Name:    "client_id",
		Wrapped: resource.ErrMissing,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateHCCLink_MissingClientSecret(t *testing.T) {
	data := &pbhcp.HCCLink{
		ClientId:     "abc",
		ClientSecret: "",
		ResourceId:   "abc",
	}

	res := createCloudLinkResource(t, data)

	err := ValidateHCCLink(res)

	expected := resource.ErrInvalidField{
		Name:    "client_secret",
		Wrapped: resource.ErrMissing,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateHCCLink_MissingResourceId(t *testing.T) {
	data := &pbhcp.HCCLink{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "",
	}

	res := createCloudLinkResource(t, data)

	err := ValidateHCCLink(res)

	expected := resource.ErrInvalidField{
		Name:    "resource_id",
		Wrapped: resource.ErrMissing,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

// Currently, we have no specific ACLs configured so the default `operator` permissions are required
func TestHCCLinkACLs(t *testing.T) {
	registry := resource.NewRegistry()
	RegisterHCCLink(registry)

	data := &pbhcp.HCCLink{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}
	hccLink := createCloudLinkResource(t, data)

	cases := map[string]rtest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Res:     hccLink,
			ReadOK:  rtest.DENY,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DENY,
		},
		"hccLink test read": {
			Rules:   `operator = "read"`,
			Res:     hccLink,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.DENY,
			ListOK:  rtest.ALLOW,
		},
		"hccLink test write": {
			Rules:   `operator = "write"`,
			Res:     hccLink,
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
