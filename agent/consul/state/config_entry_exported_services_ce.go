// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package state

import (
	"sort"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
	"github.com/hashicorp/go-memdb"
)

func getSimplifiedExportedServices(
	tx ReadTxn,
	ws memdb.WatchSet,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta acl.EnterpriseMeta,
) (uint64, *SimplifiedExportedServices, error) {
	idx, exports, err := getExportedServicesConfigEntryTxn(tx, ws, overrides, &entMeta)
	if exports == nil {
		return idx, nil, err
	}
	simple := SimplifiedExportedServices(*exports)
	return idx, &simple, err
}

func (s *Store) GetSimplifiedExportedServices(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, *SimplifiedExportedServices, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return getSimplifiedExportedServices(tx, ws, nil, entMeta)
}

func prepareExportedServicesResponse(exportedServices []structs.ExportedService, entMeta *acl.EnterpriseMeta) []*pbconfigentry.ResolvedExportedService {

	resp := make([]*pbconfigentry.ResolvedExportedService, len(exportedServices))

	for idx, exportedService := range exportedServices {
		consumerPeers := []string{}

		for _, consumer := range exportedService.Consumers {
			if consumer.Peer != "" {
				consumerPeers = append(consumerPeers, consumer.Peer)
			}
		}

		sort.Strings(consumerPeers)

		resp[idx] = &pbconfigentry.ResolvedExportedService{
			Service: exportedService.Name,
			Consumers: &pbconfigentry.Consumers{
				Peers: consumerPeers,
			},
		}
	}

	return resp
}
