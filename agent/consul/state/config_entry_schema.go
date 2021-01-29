package state

import "github.com/hashicorp/go-memdb"

const (
	configTableName = "config-entries"

	indexLink              = "link"
	indexIntentionLegacyID = "intention-legacy-id"
	indexSource            = "intention-source"
)

// configTableSchema returns a new table schema used to store global
// config entries.
func configTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: configTableName,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
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
			indexKind: {
				Name:         indexKind,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Kind",
					Lowercase: true,
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
		},
	}
}
