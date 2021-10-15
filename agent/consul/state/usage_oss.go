// +build !consulent

package state

import (
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

type EnterpriseServiceUsage struct{}

type EnterpriseConfigEntryUsage struct{}

type EnterpriseKVUsage struct{}

func addEnterpriseServiceInstanceUsage(map[string]int, memdb.Change) {}

func addEnterpriseServiceUsage(map[string]int, map[structs.ServiceName]uniqueServiceState) {}

func addEnterpriseConnectServiceInstanceUsage(map[string]int, *structs.ServiceNode, int) {}

func addEnterpriseKVUsage(map[string]int, memdb.Change) {}

func addEnterpriseConfigEntryUsage(map[string]int, memdb.Change) {}

func compileEnterpriseUsage(tx ReadTxn, usage ServiceUsage) (ServiceUsage, error) {
	return usage, nil
}

func compileEnterpriseKVUsage(tx ReadTxn, usage KVUsage) (KVUsage, error) {
	return usage, nil
}

func compileEnterpriseConfigEntryUsage(tx ReadTxn, usage ConfigEntryUsage) (ConfigEntryUsage, error) {
	return usage, nil
}
