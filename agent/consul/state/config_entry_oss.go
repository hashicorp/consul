// +build !consulent

package state

import (
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

// configTableSchema returns a new table schema used to store global
// config entries.
func configTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: configTableName,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Kind",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "Name",
							Lowercase: true,
						},
					},
				},
			},
			"kind": {
				Name:         "kind",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Kind",
					Lowercase: true,
				},
			},
			"link": {
				Name:         "link",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &ConfigEntryLinkIndex{},
			},
		},
	}
}

func firstConfigEntryWithTxn(tx ReadTxn, kind, name string, _ *structs.EnterpriseMeta) (interface{}, error) {
	return tx.First(configTableName, "id", kind, name)
}

func firstWatchConfigEntryWithTxn(
	tx ReadTxn,
	kind string,
	name string,
	_ *structs.EnterpriseMeta,
) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(configTableName, "id", kind, name)
}

func validateConfigEntryEnterprise(_ ReadTxn, _ structs.ConfigEntry) error {
	return nil
}

func getAllConfigEntriesWithTxn(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(configTableName, "id")
}

func getConfigEntryKindsWithTxn(tx ReadTxn, kind string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(configTableName, "kind", kind)
}
