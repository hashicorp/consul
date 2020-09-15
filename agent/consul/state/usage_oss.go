// +build !consulent

package state

import (
	memdb "github.com/hashicorp/go-memdb"
)

type EnterpriseServiceUsage struct{}

func addEnterpriseServiceUsage(map[string]int, memdb.Change, uniqueServiceState) {}

func compileEnterpriseUsage(tx ReadTxn, usage ServiceUsage) (ServiceUsage, error) {
	return usage, nil
}
