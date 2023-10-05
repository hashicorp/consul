package types

import (
	"github.com/hashicorp/consul/internal/resource"
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
			Type: &pbresource.Type{
				Group:        GroupName,
				GroupVersion: VersionV1Alpha1,
				Kind:         multiclusterv1alpha1.ExportedServicesKind,
			},
			Tenancy: resource.DefaultPartitionedTenancy(),
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validExportedServices() *multiclusterv1alpha1.ExportedServices {
	return &multiclusterv1alpha1.ExportedServices{
		Services:  []string{"test1", "test2"},
		Consumers: []*multiclusterv1alpha1.ExportedServicesConsumer{},
	}
}

func TestCreateExportedServicesResource(t *testing.T) {
	createExportedServicesResource(t, validExportedServices())
}
