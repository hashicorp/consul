package types

import (
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func TestMeshGatewayACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	meshGateway := &pbmesh.MeshGateway{}

	res := resourcetest.Resource(pbmesh.MeshGatewayType, "mesh-gateway").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, meshGateway).
		Build()

	testCases := []struct {
		Name        string
		Rules       string
		ExpectList  string
		ExpectRead  string
		ExpectWrite string
	}{
		{
			Name:        `no rules`,
			Rules:       ``,
			ExpectList:  resourcetest.DEFAULT,
			ExpectRead:  resourcetest.DENY,
			ExpectWrite: resourcetest.DENY,
		},
		{
			Name:        `mesh write`,
			Rules:       `mesh = "write" `,
			ExpectList:  resourcetest.DEFAULT,
			ExpectRead:  resourcetest.ALLOW,
			ExpectWrite: resourcetest.ALLOW,
		},
		{
			Name:        `mesh read only`,
			Rules:       `mesh = "read"`,
			ExpectList:  resourcetest.DEFAULT,
			ExpectRead:  resourcetest.ALLOW,
			ExpectWrite: resourcetest.DENY,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			tc := resourcetest.ACLTestCase{
				Rules:                    testCase.Rules,
				Res:                      res,
				ListOK:                   testCase.ExpectList,
				ReadOK:                   testCase.ExpectRead,
				WriteOK:                  testCase.ExpectWrite,
				ReadHookRequiresResource: false,
			}

			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}
}
