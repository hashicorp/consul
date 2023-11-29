package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
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
	data := &pbhcp.HCCLink{}

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
	data := &pbhcp.HCCLink{}

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
