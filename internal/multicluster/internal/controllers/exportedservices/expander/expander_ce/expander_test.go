// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package expander_ce

import (
	"testing"

	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/stretchr/testify/suite"
)

type expanderSuite struct {
	suite.Suite
	samenessGroupExpander *SamenessGroupExpander
}

func (suite *expanderSuite) SetupTest() {
	suite.samenessGroupExpander = New()
}

func TestExpander(t *testing.T) {
	suite.Run(t, new(expanderSuite))
}

func (suite *expanderSuite) TestExpand_NoSamenessGroupsPresent() {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: "peer-1",
			},
		},
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: "peer-2",
			},
		},
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: "peer-3",
			},
		},
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: "peer-1",
			},
		},
	}

	expandedConsumers, err := suite.samenessGroupExpander.Expand(consumers, nil)
	suite.Require().NoError(err)

	suite.Require().Equal([]string{"peer-1", "peer-2", "peer-3"}, expandedConsumers.Peers)
	suite.Require().Nil(expandedConsumers.Partitions)
	suite.Require().Empty(expandedConsumers.MissingSamenessGroups)
}
