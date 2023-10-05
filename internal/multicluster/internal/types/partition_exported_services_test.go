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

func createPartitionExportedServicesResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: &pbresource.Type{
				Group:        GroupName,
				GroupVersion: VersionV1Alpha1,
				Kind:         multiclusterv1alpha1.PartitionExportedServicesKind,
			},
			Tenancy: resource.DefaultPartitionedTenancy(),
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validPartitionExportedServices() *multiclusterv1alpha1.PartitionExportedServices {
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: []*multiclusterv1alpha1.ExportedServicesConsumer{
			{
				ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Peer{Peer: "peer"},
			},
		},
	}
}

func TestCreatePartitionExportedServicesResource(t *testing.T) {
	createPartitionExportedServicesResource(t, validNamespaceExportedServices())
}
