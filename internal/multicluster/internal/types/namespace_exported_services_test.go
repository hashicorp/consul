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

func createNamespaceExportedServicesResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: &pbresource.Type{
				Group:        GroupName,
				GroupVersion: VersionV1Alpha1,
				Kind:         multiclusterv1alpha1.NamespaceExportedServicesKind,
			},
			Tenancy: resource.DefaultPartitionedTenancy(),
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validNamespaceExportedServices() *multiclusterv1alpha1.NamespaceExportedServices {
	return &multiclusterv1alpha1.NamespaceExportedServices{
		Consumers: []*multiclusterv1alpha1.ExportedServicesConsumer{
			{
				ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Partition{Partition: "partition"},
			},
		},
	}
}

func TestCreateNamespaceExportedServicesResource(t *testing.T) {
	createNamespaceExportedServicesResource(t, validNamespaceExportedServices())
}
