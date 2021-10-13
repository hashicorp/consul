// +build !consulent

package state

import (
	"fmt"
	"strings"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

func indexFromConfigEntryKindName(arg interface{}) ([]byte, error) {
	var b indexBuilder

	switch n := arg.(type) {
	case *structs.EnterpriseMeta:
		return nil, nil
	case structs.EnterpriseMeta:
		return b.Bytes(), nil
	case ConfigEntryKindQuery:
		b.String(strings.ToLower(n.Kind))
		return b.Bytes(), nil
	case ConfigEntryKindName:
		b.String(strings.ToLower(n.Kind))
		b.String(strings.ToLower(n.Name))
		return b.Bytes(), nil
	}

	return nil, fmt.Errorf("invalid type for ConfigEntryKindName query: %T", arg)
}

func validateConfigEntryEnterprise(_ ReadTxn, _ structs.ConfigEntry) error {
	return nil
}

func getAllConfigEntriesWithTxn(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableConfigEntries, indexID)
}

func getAllConfigEntriesByKindWithTxn(tx ReadTxn, kind string) (memdb.ResultIterator, error) {
	return getConfigEntryKindsWithTxn(tx, kind, nil)
}

func getConfigEntryKindsWithTxn(tx ReadTxn, kind string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableConfigEntries, indexID+"_prefix", ConfigEntryKindQuery{Kind: kind})
}

func configIntentionsConvertToList(iter memdb.ResultIterator, _ *structs.EnterpriseMeta) structs.Intentions {
	var results structs.Intentions
	for v := iter.Next(); v != nil; v = iter.Next() {
		entry := v.(*structs.ServiceIntentionsConfigEntry)
		for _, src := range entry.Sources {
			results = append(results, entry.ToIntention(src))
		}
	}
	return results
}
