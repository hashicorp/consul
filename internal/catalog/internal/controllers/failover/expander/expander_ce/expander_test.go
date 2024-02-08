// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package expander_ce

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type expanderSuite struct {
	suite.Suite
	ctx       context.Context
	cache     cache.Cache
	rt        controller.Runtime
	tenancies []*pbresource.Tenancy

	samenessGroupExpander *SamenessGroupExpander
}

func (suite *expanderSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = rtest.TestTenancies()

	suite.samenessGroupExpander = New()

	suite.cache = cache.New()
	suite.cache.AddType(pbmulticluster.SamenessGroupType)
	suite.rt = controller.Runtime{
		Cache:  suite.cache,
		Logger: testutil.Logger(suite.T()),
	}
}

func TestExpander(t *testing.T) {
	suite.Run(t, new(expanderSuite))
}
func (suite *expanderSuite) Test_ComputeFailoverDestinationsFromSamenessGroup() {
	fpData := &pbcatalog.FailoverPolicy{
		PortConfigs: map[string]*pbcatalog.FailoverConfig{
			"http": {
				SamenessGroup: "sg1",
			},
		},
	}
	fp := rtest.Resource(pbcatalog.FailoverPolicyType, "apisvc").
		WithData(suite.T(), fpData).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	decFp, err := resource.Decode[*pbcatalog.FailoverPolicy](fp)
	require.NoError(suite.T(), err)
	dests, sg, err := suite.samenessGroupExpander.ComputeFailoverDestinationsFromSamenessGroup(suite.rt, decFp.Id, "sg1", "http")
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dests)
	require.Equal(suite.T(), "", sg)
}
