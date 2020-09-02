// +build !consulent

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
)

type EnterpriseServiceUsage struct{}

func addEnterpriseUsage(map[string]int, memdb.Change) {}

func compileServiceUsage(tx ReadTxn, totalInstances int) (ServiceUsage, error) {
	var totalServices int
	results, err := tx.Get(
		"index",
		"id_prefix",
		serviceIndexName("", nil),
	)
	if err != nil {
		return ServiceUsage{}, fmt.Errorf("failed services index lookup: %s", err)
	}
	for i := results.Next(); i != nil; i = results.Next() {
		totalServices += 1
	}

	return ServiceUsage{
		Services:         totalServices,
		ServiceInstances: totalInstances,
	}, nil
}
