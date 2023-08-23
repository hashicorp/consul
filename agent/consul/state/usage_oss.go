//go:build !consulent
// +build !consulent

package state

import (
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

type EnterpriseServiceUsage struct{}
type EnterpriseNodeUsage struct{}
type EnterprisePeeringUsage struct{}
type EnterpriseKVUsage struct{}
type EnterpriseConfigEntryUsage struct{}

func addEnterpriseNodeUsage(map[string]int, memdb.Change) {}

func addEnterprisePeeringUsage(map[string]int, memdb.Change) {}

func addEnterpriseServiceInstanceUsage(map[string]int, memdb.Change) {}

func addEnterpriseServiceUsage(map[string]int, map[structs.ServiceName]uniqueServiceState) {}

func addEnterpriseConnectServiceInstanceUsage(map[string]int, *structs.ServiceNode, int) {}

func addEnterpriseBillableServiceInstanceUsage(map[string]int, *structs.ServiceNode, int) {}

func addEnterpriseKVUsage(map[string]int, memdb.Change) {}

func addEnterpriseConfigEntryUsage(map[string]int, memdb.Change) {}

func compileEnterpriseServiceUsage(tx ReadTxn, usage structs.ServiceUsage) (structs.ServiceUsage, error) {
	return usage, nil
}

func compileEnterpriseNodeUsage(tx ReadTxn, usage NodeUsage) (NodeUsage, error) {
	return usage, nil
}

func compileEnterprisePeeringUsage(tx ReadTxn, usage PeeringUsage) (PeeringUsage, error) {
	return usage, nil
}

func compileEnterpriseKVUsage(tx ReadTxn, usage KVUsage) (KVUsage, error) {
	return usage, nil
}

func compileEnterpriseConfigEntryUsage(tx ReadTxn, usage ConfigEntryUsage) (ConfigEntryUsage, error) {
	return usage, nil
}
