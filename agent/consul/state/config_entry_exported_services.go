// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
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
