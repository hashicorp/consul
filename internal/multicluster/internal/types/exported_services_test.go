package types

import (
	"errors"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"testing"
)

func createExportedServicesResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: ExportedServicesType,
			Name: "exported-services-1",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validExportedServices() *multiclusterv1alpha1.ExportedServices {
	return &multiclusterv1alpha1.ExportedServices{
		Services: []string{"api", "frontend", "backend"},
	}
}

func inValidExportedServices() *multiclusterv1alpha1.ExportedServices {
	return &multiclusterv1alpha1.ExportedServices{}
}

func TestValidateExportedServices_Ok(t *testing.T) {
	res := createExportedServicesResource(t, validExportedServices())

	err := ValidateExportedServices(res)
	require.NoError(t, err)
}

func TestValidateExportedServices_NoServices(t *testing.T) {
	res := createExportedServicesResource(t, inValidExportedServices())

	err := ValidateExportedServices(res)
	require.Error(t, err)
	expectedError := errors.New("invalid \"services\" field: at least one service must be set")
	require.ErrorAs(t, err, &expectedError)
}
