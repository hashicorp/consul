// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"sort"

	expanderTypes "github.com/hashicorp/consul/internal/multicluster/internal/controllers/exportedservices/expander/types"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	pbmulticlusterv2beta1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type serviceExports struct {
	ref        *pbresource.Reference
	partitions map[string]struct{}
	peers      map[string]struct{}
}

type exportedServicesBuilder struct {
	data                          map[resource.ReferenceKey]*serviceExports
	samenessGroupsExpander        ExportedServicesSamenessGroupExpander
	samenessGroupsNameToMemberMap map[string][]*pbmulticlusterv2beta1.SamenessGroupMember
	missingSamenessGroups         map[resource.ReferenceKey][]string
}

func newExportedServicesBuilder(samenessGroupsExpander ExportedServicesSamenessGroupExpander, samenessGroups []*types.DecodedSamenessGroup) *exportedServicesBuilder {
	samenessGroupsNameToMemberMap := make(map[string][]*pbmulticlusterv2beta1.SamenessGroupMember)
	for _, sg := range samenessGroups {
		sgData := sg.GetData()
		if sgData == nil {
			// This should never occur
			panic("sameness group resource cannot exist without data")
		}

		samenessGroupsNameToMemberMap[sg.GetId().GetName()] = sgData.GetMembers()
	}

	return &exportedServicesBuilder{
		data:                          make(map[resource.ReferenceKey]*serviceExports),
		samenessGroupsExpander:        samenessGroupsExpander,
		samenessGroupsNameToMemberMap: samenessGroupsNameToMemberMap,
		missingSamenessGroups:         make(map[resource.ReferenceKey][]string),
	}
}

// expandConsumers expands the consumers for a given ExportedServices resource
// and keeps track of the unresolved sameness groups
func (b *exportedServicesBuilder) expandConsumers(exportedSvcResourceRef resource.ReferenceKey, consumers []*pbmulticluster.ExportedServicesConsumer) (*expanderTypes.ExpandedConsumers, error) {
	expandedConsumers, err := b.samenessGroupsExpander.Expand(consumers, b.samenessGroupsNameToMemberMap)
	if err != nil {
		return nil, err
	}

	if len(expandedConsumers.MissingSamenessGroups) > 0 {
		b.missingSamenessGroups[exportedSvcResourceRef] = append(b.missingSamenessGroups[exportedSvcResourceRef], expandedConsumers.MissingSamenessGroups...)
	}

	return expandedConsumers, nil
}

// track associates a service resource with the corresponding partitions
// and peers declared by the various ExportedService resources.
func (b *exportedServicesBuilder) track(svcID *pbresource.ID, expandedConsumers *expanderTypes.ExpandedConsumers) {
	key := resource.NewReferenceKey(svcID)
	exports, ok := b.data[key]

	if !ok {
		exports = &serviceExports{
			ref:        resource.Reference(svcID, ""),
			partitions: make(map[string]struct{}),
			peers:      make(map[string]struct{}),
		}
		b.data[key] = exports
	}

	for _, p := range expandedConsumers.Partitions {
		exports.partitions[p] = struct{}{}
	}

	for _, p := range expandedConsumers.Peers {
		exports.peers[p] = struct{}{}
	}
}

func (b *exportedServicesBuilder) build() *pbmulticluster.ComputedExportedServices {
	if len(b.data) == 0 {
		return nil
	}

	ces := &pbmulticluster.ComputedExportedServices{
		Services: make([]*pbmulticluster.ComputedExportedService, 0, len(b.data)),
	}

	for _, svc := range sortRefValue(b.data) {
		consumers := make([]*pbmulticluster.ComputedExportedServiceConsumer, 0, len(svc.peers)+len(svc.partitions))

		for _, peer := range sortKeys(svc.peers) {
			consumers = append(consumers, &pbmulticluster.ComputedExportedServiceConsumer{
				Tenancy: &pbmulticluster.ComputedExportedServiceConsumer_Peer{
					Peer: peer,
				},
			})
		}

		for _, partition := range sortKeys(svc.partitions) {
			// Filter out the partition that matches with the
			// partition of the service reference. This is done
			// to avoid the name of the local partition to be
			// present as a consumer in the ComputedExportedService resource.
			if svc.ref.Tenancy.Partition == partition {
				continue
			}

			consumers = append(consumers, &pbmulticluster.ComputedExportedServiceConsumer{
				Tenancy: &pbmulticluster.ComputedExportedServiceConsumer_Partition{
					Partition: partition,
				},
			})
		}

		ces.Services = append(ces.Services, &pbmulticluster.ComputedExportedService{
			TargetRef: svc.ref,
			Consumers: consumers,
		})
	}

	return ces
}

// getMissingSamenessGroupsForComputedExportedService returns back the sorted
// list of unique SamenessGroup names that couldn't be resolved by the builder
// for the ComputedExportedService resource.
func (b *exportedServicesBuilder) getMissingSamenessGroupsForComputedExportedService() []string {
	samenessGroupMap := make(map[string]struct{})
	for _, val := range b.missingSamenessGroups {
		for _, v := range val {
			samenessGroupMap[v] = struct{}{}
		}
	}

	return sortKeys(samenessGroupMap)
}

func sortKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortRefValue(m map[resource.ReferenceKey]*serviceExports) []*serviceExports {
	vals := make([]*serviceExports, 0, len(m))
	for _, val := range m {
		vals = append(vals, val)
	}
	sort.Slice(vals, func(i, j int) bool {
		return resource.ReferenceToString(vals[i].ref) < resource.ReferenceToString(vals[j].ref)
	})
	return vals
}
