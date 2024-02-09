// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package expander_ce

import (
	"context"
	"sort"

	"github.com/hashicorp/consul/internal/controller"
	expanderTypes "github.com/hashicorp/consul/internal/multicluster/internal/controllers/exportedservices/expander/types"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	pbmulticlusterv2beta1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
)

type SamenessGroupExpander struct{}

func New() *SamenessGroupExpander {
	return &SamenessGroupExpander{}
}

func (sg *SamenessGroupExpander) List(_ context.Context, _ controller.Runtime, _ controller.Request) ([]*types.DecodedSamenessGroup, error) {
	return nil, nil
}

func (sg *SamenessGroupExpander) Expand(consumers []*pbmulticluster.ExportedServicesConsumer, _ map[string][]*pbmulticlusterv2beta1.SamenessGroupMember) (*expanderTypes.ExpandedConsumers, error) {
	peers := make([]string, 0)
	peersMap := make(map[string]struct{})
	for _, c := range consumers {
		switch c.ConsumerTenancy.(type) {
		case *pbmulticluster.ExportedServicesConsumer_Peer:
			peersMap[c.GetPeer()] = struct{}{}
		default:
			panic("unexpected consumer tenancy type")
		}
	}

	for peer := range peersMap {
		peers = append(peers, peer)
	}
	sort.Strings(peers)

	return &expanderTypes.ExpandedConsumers{
		Peers: peers,
	}, nil
}
