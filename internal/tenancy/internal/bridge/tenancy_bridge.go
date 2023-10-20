package bridge

import (
	"context"
	"github.com/hashicorp/consul/internal/tenancy/internal/types"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// V2TenancyBridge is used by the resource service to access V2 implementations of
// partitions and namespaces.
type V2TenancyBridge struct {
	client pbresource.ResourceServiceClient
}

// WithClient inject a ResourceServiceClient in the V2TenancyBridge.
// This is needed to break a circular dependency between
// the ResourceServiceServer, ResourceServiceClient and the TenancyBridge
func (b *V2TenancyBridge) WithClient(client pbresource.ResourceServiceClient) *V2TenancyBridge {
	b.client = client
	return b
}

func NewV2TenancyBridge() *V2TenancyBridge {
	return &V2TenancyBridge{}
}

func (b *V2TenancyBridge) NamespaceExists(partition, namespace string) (bool, error) {
	read, err := b.client.Read(context.Background(), &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Name: namespace,
			Tenancy: &pbresource.Tenancy{
				Partition: partition,
			},
			Type: types.NamespaceType,
		},
	})
	return read != nil && read.Resource != nil, err
}

func (b *V2TenancyBridge) IsNamespaceMarkedForDeletion(partition, namespace string) (bool, error) {
	read, err := b.client.Read(context.Background(), &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Name: namespace,
			Tenancy: &pbresource.Tenancy{
				Partition: partition,
			},
			Type: types.NamespaceType,
		},
	})
	return read.Resource != nil, err
}
