// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"strings"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	tableConfigEntries = "config-entries"

	indexLink                 = "link"
	indexIntentionLegacyID    = "intention-legacy-id"
	indexSource               = "intention-source"
	indexSourceSamenessGroup  = "intention-source-sameness-group"
	indexSamenessGroupMember  = "sameness-group-member"
	indexSamenessGroupDefault = "sameness-group-default-for-failover"
)

// configTableSchema returns a new table schema used to store global
// config entries.
func configTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableConfigEntries,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[any, structs.ConfigEntry, any]{
					readIndex:   indexFromConfigEntryKindName,
					writeIndex:  indexFromConfigEntry,
					prefixIndex: indexFromConfigEntryKindName,
				},
			},
			indexLink: {
				Name:         indexLink,
				AllowMissing: true,
				Unique:       false,
				Indexer:      &ConfigEntryLinkIndex{},
			},
			indexIntentionLegacyID: {
				Name:         indexIntentionLegacyID,
				AllowMissing: true,
				Unique:       true,
				Indexer:      &ServiceIntentionLegacyIDIndex{},
			},
			indexSource: {
				Name:         indexSource,
				AllowMissing: true,
				Unique:       false,
				Indexer:      &ServiceIntentionSourceIndex{},
			},
			indexSourceSamenessGroup: {
				Name:         indexSourceSamenessGroup,
				AllowMissing: true,
				Unique:       false,
				Indexer:      &ServiceIntentionSourceSamenessGroupIndex{},
			},
			indexSamenessGroupMember: {
				Name:         indexSamenessGroupMember,
				AllowMissing: true,
				Unique:       false,
				Indexer:      &SamenessGroupMemberIndex{},
			},
			indexSamenessGroupDefault: {
				Name:         indexSamenessGroupDefault,
				AllowMissing: true,
				Unique:       true,
				Indexer:      &SamenessGroupDefaultIndex{},
			},
		},
	}
}

// configEntryIndexable is required because while structs.ConfigEntry
// has a GetEnterpriseMeta method, it does not directly expose the
// required NamespaceOrDefault and PartitionOrDefault methods of
// enterpriseIndexable.
//
// Config entries that embed *acl.EnterpriseMeta will automatically
// implement this interface.
type configEntryIndexable interface {
	structs.ConfigEntry
	enterpriseIndexable
}

var _ configEntryIndexable = (*structs.ExportedServicesConfigEntry)(nil)
var _ configEntryIndexable = (*structs.SamenessGroupConfigEntry)(nil)
var _ configEntryIndexable = (*structs.IngressGatewayConfigEntry)(nil)
var _ configEntryIndexable = (*structs.MeshConfigEntry)(nil)
var _ configEntryIndexable = (*structs.ProxyConfigEntry)(nil)
var _ configEntryIndexable = (*structs.ServiceConfigEntry)(nil)
var _ configEntryIndexable = (*structs.ServiceIntentionsConfigEntry)(nil)
var _ configEntryIndexable = (*structs.ServiceResolverConfigEntry)(nil)
var _ configEntryIndexable = (*structs.ServiceRouterConfigEntry)(nil)
var _ configEntryIndexable = (*structs.ServiceSplitterConfigEntry)(nil)
var _ configEntryIndexable = (*structs.TerminatingGatewayConfigEntry)(nil)
var _ configEntryIndexable = (*structs.JWTProviderConfigEntry)(nil)

func indexFromConfigEntry(c structs.ConfigEntry) ([]byte, error) {
	if c.GetName() == "" || c.GetKind() == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(c.GetKind()))
	b.String(strings.ToLower(c.GetName()))
	return b.Bytes(), nil
}

// indexKindFromConfigEntry indexes kinds without a namespace for any config
// entries that span all namespaces.
func indexKindFromConfigEntry(c configEntryIndexable) ([]byte, error) {
	if c.GetKind() == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(c.GetKind()))
	return b.Bytes(), nil
}
