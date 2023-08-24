// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package state

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
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
