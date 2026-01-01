// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"sort"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
)

// importedService represents a service imported from a peer
type importedService struct {
	service   string
	namespace string
	peer      string
}

// ResolvedImportedServices returns the list of imported services along with their sources.
// This shows which services are being imported from peers.
func (s *Store) ResolvedImportedServices(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, []*pbconfigentry.ResolvedImportedService, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return resolvedImportedServicesTxn(tx, ws, entMeta)
}

func resolvedImportedServicesTxn(tx ReadTxn, ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, []*pbconfigentry.ResolvedImportedService, error) {
	maxIdx := uint64(0)

	// Get all service intentions that have a source peer set
	// This indicates services that are imported from that peer
	iter, err := tx.Get(tableConfigEntries, indexID+"_prefix", ConfigEntryKindQuery{
		Kind:           structs.ServiceIntentions,
		EnterpriseMeta: *entMeta,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed to list service intentions: %w", err)
	}

	ws.Add(iter.WatchCh())

	// Collect imported services from intentions
	var importedServices []importedService

	for entry := iter.Next(); entry != nil; entry = iter.Next() {
		intention, ok := entry.(*structs.ServiceIntentionsConfigEntry)
		if !ok {
			continue
		}

		// Update max index
		if intention.RaftIndex.ModifyIndex > maxIdx {
			maxIdx = intention.RaftIndex.ModifyIndex
		}

		// Check each source intention for peer imports
		for _, source := range intention.Sources {
			if source.Peer != "" {
				importedServices = append(importedServices, importedService{
					service:   intention.Name,
					namespace: intention.NamespaceOrDefault(),
					peer:      source.Peer,
				})
			}
		}
	}

	uniqueImportedServices := getUniqueImportedServices(importedServices, entMeta)
	resp := prepareImportedServicesResponse(uniqueImportedServices, entMeta)

	return lib.MaxUint64(maxIdx, 1), resp, nil
}

// getUniqueImportedServices removes duplicate services and sources. Services are also sorted in ascending order
func getUniqueImportedServices(importedServices []importedService, entMeta *acl.EnterpriseMeta) []importedService {
	// Service -> SourcePeers
	type serviceKey struct {
		name      string
		namespace string
	}
	importedServicesMapper := make(map[serviceKey]map[string]struct{})

	for _, svc := range importedServices {
		key := serviceKey{
			name:      svc.service,
			namespace: svc.namespace,
		}

		peers, ok := importedServicesMapper[key]
		if !ok {
			peers = make(map[string]struct{})
			importedServicesMapper[key] = peers
		}
		peers[svc.peer] = struct{}{}
	}

	uniqueImportedServices := make([]importedService, 0)

	for svc, peers := range importedServicesMapper {
		for peer := range peers {
			uniqueImportedServices = append(uniqueImportedServices, importedService{
				service:   svc.name,
				namespace: svc.namespace,
				peer:      peer,
			})
		}
	}

	sort.Slice(uniqueImportedServices, func(i, j int) bool {
		if uniqueImportedServices[i].service != uniqueImportedServices[j].service {
			return uniqueImportedServices[i].service < uniqueImportedServices[j].service
		}
		if uniqueImportedServices[i].namespace != uniqueImportedServices[j].namespace {
			return uniqueImportedServices[i].namespace < uniqueImportedServices[j].namespace
		}
		return uniqueImportedServices[i].peer < uniqueImportedServices[j].peer
	})

	return uniqueImportedServices
}
