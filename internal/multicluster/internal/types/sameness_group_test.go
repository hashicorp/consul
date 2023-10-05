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

func createSamenessGroupResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: &pbresource.Type{
				Group:        GroupName,
				GroupVersion: VersionV1Alpha1,
				Kind:         multiclusterv1alpha1.SamenessGroupKind,
			},
			Tenancy: resource.DefaultPartitionedTenancy(),
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validSamenessGroup() *multiclusterv1alpha1.SamenessGroup {
	return &multiclusterv1alpha1.SamenessGroup{}
}

func TestCreateSamenessGroup(t *testing.T) {
	createSamenessGroupResource(t, validSamenessGroup())
}
