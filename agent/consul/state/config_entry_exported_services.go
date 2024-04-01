// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"sort"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
	"github.com/hashicorp/go-memdb"
)

// SimplifiedExportedServices contains a version of the exported-services that has
// been flattened by removing all of the sameness group references and replacing
// them with corresponding partition / peer entries.
type SimplifiedExportedServices structs.ExportedServicesConfigEntry

// ToPartitionMap is only used by the partition exporting logic.
// It returns a map[namespace][service] => []consuming_partitions
func (e *SimplifiedExportedServices) ToPartitionMap() map[string]map[string][]string {
	resp := make(map[string]map[string][]string)
	for _, svc := range e.Services {
		if _, ok := resp[svc.Namespace]; !ok {
			resp[svc.Namespace] = make(map[string][]string)
		}
		if _, ok := resp[svc.Namespace][svc.Name]; !ok {
			consumers := make([]string, 0, len(svc.Consumers))
			for _, c := range svc.Consumers {
				if c.Partition != "" {
					consumers = append(consumers, c.Partition)
				}
			}
			resp[svc.Namespace][svc.Name] = consumers
			resp[svc.Namespace][svc.Name+structs.SidecarProxySuffix] = consumers
		}
	}
	return resp
}

// getExportedServicesConfigEntryTxn is a convenience method for fetching a
// exported-services kind of config entry.
//
// If an override KEY is present for the requested config entry, the index
// returned will be 0. Any override VALUE (nil or otherwise) will be returned
// if there is a KEY match.
func getExportedServicesConfigEntryTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
) (uint64, *structs.ExportedServicesConfigEntry, error) {
	idx, entry, err := configEntryWithOverridesTxn(tx, ws, structs.ExportedServices, entMeta.PartitionOrDefault(), overrides, entMeta)
	if err != nil {
		return 0, nil, err
	} else if entry == nil {
		return idx, nil, nil
	}

	export, ok := entry.(*structs.ExportedServicesConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("invalid service config type %T", entry)
	}
	return idx, export, nil
}

// ResolvedExportedServices returns the list of exported services along with consumers.
// Sameness Groups and wild card entries are resolved.
func (s *Store) ResolvedExportedServices(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, []*pbconfigentry.ResolvedExportedService, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return resolvedExportedServicesTxn(tx, ws, entMeta)
}

func resolvedExportedServicesTxn(tx ReadTxn, ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, []*pbconfigentry.ResolvedExportedService, error) {
	var resp []*pbconfigentry.ResolvedExportedService

	// getSimplifiedExportedServices resolves the sameness group information to partitions and peers.
	maxIdx, exports, err := getSimplifiedExportedServices(tx, ws, nil, *entMeta)
	if err != nil {
		return 0, nil, err
	}
	if exports == nil {
		return maxIdx, nil, nil
	}

	var exportedServices []structs.ExportedService

	for _, svc := range exports.Services {
		// Prevent exporting the "consul" service.
		if svc.Name == structs.ConsulServiceName {
			continue
		}

		// If this isn't a wildcard, we can simply add it to the list of exportedServices and move to the next entry.
		if svc.Name != structs.WildcardSpecifier {
			exportedServices = append(exportedServices, svc)
			continue
		}

		svcEntMeta := acl.NewEnterpriseMetaWithPartition(entMeta.PartitionOrDefault(), svc.Namespace)

		// If all services in the namespace are exported by the wildcard, query those service names.
		idx, typicalServices, err := serviceNamesOfKindTxn(tx, ws, structs.ServiceKindTypical, svcEntMeta)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get typical service names: %w", err)
		}

		maxIdx = lib.MaxUint64(maxIdx, idx)

		for _, sn := range typicalServices {
			// Prevent exporting the "consul" service.
			if sn.Service.Name != structs.ConsulServiceName {
				exportedServices = append(exportedServices, structs.ExportedService{
					Name:      sn.Service.Name,
					Namespace: sn.Service.NamespaceOrDefault(),
					Consumers: svc.Consumers,
				})
			}
		}
	}

	uniqueExportedServices := getUniqueExportedServices(exportedServices, entMeta)
	resp = prepareExportedServicesResponse(uniqueExportedServices, entMeta)

	return maxIdx, resp, nil
}

// getUniqueExportedServices removes duplicate services and consumers. Services are also sorted in ascending order
func getUniqueExportedServices(exportedServices []structs.ExportedService, entMeta *acl.EnterpriseMeta) []structs.ExportedService {
	// Services -> ServiceConsumers
	var exportedServicesMapper = make(map[structs.ServiceName]map[structs.ServiceConsumer]struct{})
	for _, svc := range exportedServices {
		svcEntMeta := acl.NewEnterpriseMetaWithPartition(entMeta.PartitionOrDefault(), svc.Namespace)
		svcName := structs.NewServiceName(svc.Name, &svcEntMeta)

		for _, c := range svc.Consumers {
			cons, ok := exportedServicesMapper[svcName]
			if !ok {
				cons = make(map[structs.ServiceConsumer]struct{})
				exportedServicesMapper[svcName] = cons
			}
			cons[c] = struct{}{}
		}
	}

	uniqueExportedServices := make([]structs.ExportedService, 0, len(exportedServicesMapper))

	for svc, cons := range exportedServicesMapper {
		consumers := make([]structs.ServiceConsumer, 0, len(cons))
		for con := range cons {
			consumers = append(consumers, con)
		}

		uniqueExportedServices = append(uniqueExportedServices, structs.ExportedService{
			Name:      svc.Name,
			Namespace: svc.NamespaceOrDefault(),
			Consumers: consumers,
		})

	}

	sort.Slice(uniqueExportedServices, func(i, j int) bool {
		return (uniqueExportedServices[i].Name < uniqueExportedServices[j].Name) ||
			(uniqueExportedServices[i].Name == uniqueExportedServices[j].Name && uniqueExportedServices[i].Namespace < uniqueExportedServices[j].Namespace)
	})

	return uniqueExportedServices
}
