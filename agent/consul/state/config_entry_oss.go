// +build !consulent

package state

import (
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

func firstConfigEntryWithTxn(tx ReadTxn, kind, name string, _ *structs.EnterpriseMeta) (interface{}, error) {
	return tx.First(tableConfigEntries, "id", kind, name)
}

func firstWatchConfigEntryWithTxn(
	tx ReadTxn,
	kind string,
	name string,
	_ *structs.EnterpriseMeta,
) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(tableConfigEntries, "id", kind, name)
}

func validateConfigEntryEnterprise(_ ReadTxn, _ structs.ConfigEntry) error {
	return nil
}

func getAllConfigEntriesWithTxn(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableConfigEntries, "id")
}

func getConfigEntryKindsWithTxn(tx ReadTxn, kind string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableConfigEntries, "kind", kind)
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
