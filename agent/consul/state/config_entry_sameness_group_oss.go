// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// SamenessGroupDefaultIndex is a placeholder for OSS. Sameness-groups are enterprise only.
type SamenessGroupDefaultIndex struct{}

var _ memdb.Indexer = (*SamenessGroupDefaultIndex)(nil)
var _ memdb.MultiIndexer = (*SamenessGroupDefaultIndex)(nil)

func (*SamenessGroupDefaultIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	return false, nil, nil
}

func (*SamenessGroupDefaultIndex) FromArgs(args ...interface{}) ([]byte, error) {
	return nil, nil
}

func checkSamenessGroup(tx ReadTxn, newConfig structs.ConfigEntry) error {
	return fmt.Errorf("sameness-groups are an enterprise-only feature")
}

// getExportedServicesConfigEntryTxn is a convenience method for fetching a
// sameness-group config entries.
//
// If an override KEY is present for the requested config entry, the index
// returned will be 0. Any override VALUE (nil or otherwise) will be returned
// if there is a KEY match.
func getSamenessGroupConfigEntryTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	name string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	partition string,
) (uint64, *structs.SamenessGroupConfigEntry, error) {
	return 0, nil, nil
}

func getDefaultSamenessGroup(tx ReadTxn, ws memdb.WatchSet, partition string) (uint64, *structs.SamenessGroupConfigEntry, error) {
	return 0, nil, nil
}
