package types

import (
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/version/versiontest"
)

func TestMeshGatewayACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	meshGateway := &pbmesh.MeshGateway{}

	testCases := []struct {
		Name           string
		EnterpriseOnly bool
		Rules          string
		ExpectList     string
		ExpectRead     string
		ExpectWrite    string
		Resource       *pbresource.Resource
	}{
		{
			Name:        `no rules`,
			Rules:       ``,
			ExpectList:  resourcetest.DEFAULT,
			ExpectRead:  resourcetest.DENY,
			ExpectWrite: resourcetest.DENY,
			Resource: resourcetest.Resource(pbmesh.MeshGatewayType, "mesh-gateway").
				WithTenancy(resource.DefaultPartitionedTenancy()).
				WithData(t, meshGateway).
				Build(),
		},
		{
			Name:        `mesh write`,
			Rules:       `mesh = "write" `,
			ExpectList:  resourcetest.DEFAULT,
			ExpectRead:  resourcetest.ALLOW,
			ExpectWrite: resourcetest.ALLOW,
			Resource: resourcetest.Resource(pbmesh.MeshGatewayType, "mesh-gateway").
				WithTenancy(resource.DefaultPartitionedTenancy()).
				WithData(t, meshGateway).
				Build(),
		},
		{
			Name:        `mesh read only`,
			Rules:       `mesh = "read"`,
			ExpectList:  resourcetest.DEFAULT,
			ExpectRead:  resourcetest.ALLOW,
			ExpectWrite: resourcetest.DENY,
			Resource: resourcetest.Resource(pbmesh.MeshGatewayType, "mesh-gateway").
				WithTenancy(resource.DefaultPartitionedTenancy()).
				WithData(t, meshGateway).
				Build(),
		},
		{
			Name:           `non-default partition`,
			EnterpriseOnly: true,
			Rules:          `partition "other" { mesh = "write" }`,
			ExpectList:     resourcetest.DEFAULT,
			ExpectRead:     resourcetest.ALLOW,
			ExpectWrite:    resourcetest.ALLOW,
			Resource: resourcetest.Resource(pbmesh.MeshGatewayType, "mesh-gateway").
				WithTenancy(&pbresource.Tenancy{
					Partition: "other",
					PeerName:  resource.DefaultPeerName,
				}).
				WithData(t, meshGateway).
				Build(),
		},
		{
			Name:           `mesh write in wrong partition`,
			EnterpriseOnly: true,
			Rules:          `partition "other" { mesh = "write" }`,
			ExpectList:     resourcetest.DEFAULT,
			ExpectRead:     resourcetest.DENY,
			ExpectWrite:    resourcetest.DENY,
			Resource: resourcetest.Resource(pbmesh.MeshGatewayType, "mesh-gateway").
				WithTenancy(resource.DefaultPartitionedTenancy()).
				WithData(t, meshGateway).
				Build(),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			if testCase.EnterpriseOnly && !versiontest.IsEnterprise() {
				t.Skip("skipping enterprise-only test case")
			}

			tc := resourcetest.ACLTestCase{
				Rules:                    testCase.Rules,
				Res:                      testCase.Resource,
				ListOK:                   testCase.ExpectList,
				ReadOK:                   testCase.ExpectRead,
				WriteOK:                  testCase.ExpectWrite,
				ReadHookRequiresResource: false,
			}

			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}
}
