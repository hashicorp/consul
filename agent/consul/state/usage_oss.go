//go:build !consulent
// +build !consulent

package state

import (
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

type EnterpriseServiceUsage struct{}
type EnterpriseNodeUsage struct{}
type EnterpriseKVUsage struct{}

func addEnterpriseNodeUsage(map[string]int, memdb.Change) {}

func addEnterpriseServiceInstanceUsage(map[string]int, memdb.Change) {}

func addEnterpriseServiceUsage(map[string]int, map[structs.ServiceName]uniqueServiceState) {}

func addEnterpriseKVUsage(map[string]int, memdb.Change) {}

func compileEnterpriseServiceUsage(tx ReadTxn, usage ServiceUsage) (ServiceUsage, error) {
	return usage, nil
}

func compileEnterpriseNodeUsage(tx ReadTxn, usage NodeUsage) (NodeUsage, error) {
	return usage, nil
}

func compileEnterpriseKVUsage(tx ReadTxn, usage KVUsage) (KVUsage, error) {
	return usage, nil
}
