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

func prepareExportedServicesResponse(exportedServices map[structs.ServiceName]map[structs.ServiceConsumer]struct{}) []*pbconfigentry.ResolvedExportedService {
	var resp []*pbconfigentry.ResolvedExportedService

	for svc, consumers := range exportedServices {
		consumerPeers := []string{}

		for consumer := range consumers {
			if consumer.Peer != "" {
				consumerPeers = append(consumerPeers, consumer.Peer)
			}
		}

		sort.Strings(consumerPeers)

		resp = append(resp, &pbconfigentry.ResolvedExportedService{
			Service: svc.Name,
			Consumers: &pbconfigentry.Consumers{
				Peers: consumerPeers,
			},
		})
	}

	return resp
}
