//go:build !consulent
// +build !consulent

package state

import (
	"fmt"
	"strings"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func withEnterpriseSchema(_ *memdb.DBSchema) {}

func serviceIndexName(name string, _ *acl.EnterpriseMeta) string {
	return fmt.Sprintf("service.%s", name)
}

func serviceKindIndexName(kind structs.ServiceKind, _ *acl.EnterpriseMeta) string {
	return "service_kind." + kind.Normalized()
}

func catalogUpdateNodesIndexes(tx WriteTxn, idx uint64, entMeta *acl.EnterpriseMeta) error {
	// overall nodes index
	if err := indexUpdateMaxTxn(tx, idx, tableNodes); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogUpdateServicesIndexes(tx WriteTxn, idx uint64, _ *acl.EnterpriseMeta) error {
	// overall services index
	if err := indexUpdateMaxTxn(tx, idx, tableServices); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogUpdateServiceKindIndexes(tx WriteTxn, kind structs.ServiceKind, idx uint64, _ *acl.EnterpriseMeta) error {
	// service-kind index
	if err := indexUpdateMaxTxn(tx, idx, serviceKindIndexName(kind, nil)); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogUpdateServiceIndexes(tx WriteTxn, serviceName string, idx uint64, _ *acl.EnterpriseMeta) error {
	// per-service index
	if err := indexUpdateMaxTxn(tx, idx, serviceIndexName(serviceName, nil)); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogUpdateServiceExtinctionIndex(tx WriteTxn, idx uint64, _ *acl.EnterpriseMeta) error {
	if err := tx.Insert(tableIndex, &IndexEntry{indexServiceExtinction, idx}); err != nil {
		return fmt.Errorf("failed updating missing service extinction index: %s", err)
	}
	return nil
}

func catalogInsertNode(tx WriteTxn, node *structs.Node) error {
	// ensure that the Partition is always clear within the state store in OSS
	node.Partition = ""

	// Insert the node and update the index.
	if err := tx.Insert(tableNodes, node); err != nil {
		return fmt.Errorf("failed inserting node: %s", err)
	}

	if err := catalogUpdateNodesIndexes(tx, node.ModifyIndex, node.GetEnterpriseMeta()); err != nil {
		return err
	}

	// Update the node's service indexes as the node information is included
	// in health queries and we would otherwise miss node updates in some cases
	// for those queries.
	if err := updateAllServiceIndexesOfNode(tx, node.ModifyIndex, node.Node, node.GetEnterpriseMeta()); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogInsertService(tx WriteTxn, svc *structs.ServiceNode) error {
	// Insert the service and update the index
	if err := tx.Insert(tableServices, svc); err != nil {
		return fmt.Errorf("failed inserting service: %s", err)
	}

	if err := catalogUpdateServicesIndexes(tx, svc.ModifyIndex, &svc.EnterpriseMeta); err != nil {
		return err
	}

	if err := catalogUpdateServiceIndexes(tx, svc.ServiceName, svc.ModifyIndex, &svc.EnterpriseMeta); err != nil {
		return err
	}

	if err := catalogUpdateServiceKindIndexes(tx, svc.ServiceKind, svc.ModifyIndex, &svc.EnterpriseMeta); err != nil {
		return err
	}

	return nil
}

func catalogNodesMaxIndex(tx ReadTxn, entMeta *acl.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, tableNodes)
}

func catalogServicesMaxIndex(tx ReadTxn, _ *acl.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, tableServices)
}

func catalogServiceMaxIndex(tx ReadTxn, serviceName string, _ *acl.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(tableIndex, "id", serviceIndexName(serviceName, nil))
}

func catalogServiceKindMaxIndex(tx ReadTxn, ws memdb.WatchSet, kind structs.ServiceKind, entMeta *acl.EnterpriseMeta) uint64 {
	return maxIndexWatchTxn(tx, ws, serviceKindIndexName(kind, nil))
}

func catalogServiceListNoWildcard(tx ReadTxn, _ *acl.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableServices, indexID)
}

func catalogServiceListByNode(tx ReadTxn, node string, _ *acl.EnterpriseMeta, _ bool) (memdb.ResultIterator, error) {
	return tx.Get(tableServices, indexNode, Query{Value: node})
}

func catalogServiceLastExtinctionIndex(tx ReadTxn, _ *acl.EnterpriseMeta) (interface{}, error) {
	return tx.First(tableIndex, "id", indexServiceExtinction)
}

func catalogMaxIndex(tx ReadTxn, _ *acl.EnterpriseMeta, checks bool) uint64 {
	if checks {
		return maxIndexTxn(tx, tableNodes, tableServices, tableChecks)
	}
	return maxIndexTxn(tx, tableNodes, tableServices)
}

func catalogMaxIndexWatch(tx ReadTxn, ws memdb.WatchSet, _ *acl.EnterpriseMeta, checks bool) uint64 {
	if checks {
		return maxIndexWatchTxn(tx, ws, tableNodes, tableServices, tableChecks)
	}
	return maxIndexWatchTxn(tx, ws, tableNodes, tableServices)
}

func catalogUpdateCheckIndexes(tx WriteTxn, idx uint64, _ *acl.EnterpriseMeta) error {
	// update the universal index entry
	if err := tx.Insert(tableIndex, &IndexEntry{tableChecks, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

func catalogChecksMaxIndex(tx ReadTxn, _ *acl.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, tableChecks)
}

func catalogListChecksByNode(tx ReadTxn, q Query) (memdb.ResultIterator, error) {
	return tx.Get(tableChecks, indexNode, q)
}

func catalogInsertCheck(tx WriteTxn, chk *structs.HealthCheck, idx uint64) error {
	// Insert the check
	if err := tx.Insert(tableChecks, chk); err != nil {
		return fmt.Errorf("failed inserting check: %s", err)
	}

	if err := catalogUpdateCheckIndexes(tx, idx, &chk.EnterpriseMeta); err != nil {
		return err
	}

	return nil
}

func validateRegisterRequestTxn(_ ReadTxn, _ *structs.RegisterRequest, _ bool) (*acl.EnterpriseMeta, error) {
	return nil, nil
}

func (s *Store) ValidateRegisterRequest(_ *structs.RegisterRequest) (*acl.EnterpriseMeta, error) {
	return nil, nil
}

func indexFromKindServiceName(arg interface{}) ([]byte, error) {
	var b indexBuilder

	switch n := arg.(type) {
	case KindServiceNameQuery:
		b.String(strings.ToLower(string(n.Kind)))
		b.String(strings.ToLower(n.Name))
		return b.Bytes(), nil

	case *KindServiceName:
		b.String(strings.ToLower(string(n.Kind)))
		b.String(strings.ToLower(n.Service.Name))
		return b.Bytes(), nil

	default:
		return nil, fmt.Errorf("type must be KindServiceNameQuery or *KindServiceName: %T", arg)
	}
}
