// +build !consulent

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

func withEnterpriseSchema(_ *memdb.DBSchema) {}

func serviceIndexName(name string, _ *structs.EnterpriseMeta) string {
	return fmt.Sprintf("service.%s", name)
}

func serviceKindIndexName(kind structs.ServiceKind, _ *structs.EnterpriseMeta) string {
	switch kind {
	case structs.ServiceKindTypical:
		// needs a special case here
		return "service_kind.typical"
	default:
		return "service_kind." + string(kind)
	}
}

func catalogUpdateServicesIndexes(tx WriteTxn, idx uint64, _ *structs.EnterpriseMeta) error {
	// overall services index
	if err := indexUpdateMaxTxn(tx, idx, tableServices); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogUpdateServiceKindIndexes(tx WriteTxn, kind structs.ServiceKind, idx uint64, _ *structs.EnterpriseMeta) error {
	// service-kind index
	if err := indexUpdateMaxTxn(tx, idx, serviceKindIndexName(kind, nil)); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogUpdateServiceIndexes(tx WriteTxn, serviceName string, idx uint64, _ *structs.EnterpriseMeta) error {
	// per-service index
	if err := indexUpdateMaxTxn(tx, idx, serviceIndexName(serviceName, nil)); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogUpdateServiceExtinctionIndex(tx WriteTxn, idx uint64, _ *structs.EnterpriseMeta) error {
	if err := tx.Insert(tableIndex, &IndexEntry{indexServiceExtinction, idx}); err != nil {
		return fmt.Errorf("failed updating missing service extinction index: %s", err)
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

func catalogServicesMaxIndex(tx ReadTxn, _ *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, tableServices)
}

func catalogServiceMaxIndex(tx ReadTxn, serviceName string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(tableIndex, "id", serviceIndexName(serviceName, nil))
}

func catalogServiceKindMaxIndex(tx ReadTxn, ws memdb.WatchSet, kind structs.ServiceKind, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexWatchTxn(tx, ws, serviceKindIndexName(kind, nil))
}

func catalogServiceListNoWildcard(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get(tableServices, indexID)
}

func catalogServiceListByNode(tx ReadTxn, node string, _ *structs.EnterpriseMeta, _ bool) (memdb.ResultIterator, error) {
	return tx.Get(tableServices, indexNode, Query{Value: node})
}

func catalogServiceLastExtinctionIndex(tx ReadTxn, _ *structs.EnterpriseMeta) (interface{}, error) {
	return tx.First(tableIndex, "id", indexServiceExtinction)
}

func catalogMaxIndex(tx ReadTxn, _ *structs.EnterpriseMeta, checks bool) uint64 {
	if checks {
		return maxIndexTxn(tx, "nodes", tableServices, tableChecks)
	}
	return maxIndexTxn(tx, "nodes", tableServices)
}

func catalogMaxIndexWatch(tx ReadTxn, ws memdb.WatchSet, _ *structs.EnterpriseMeta, checks bool) uint64 {
	if checks {
		return maxIndexWatchTxn(tx, ws, "nodes", tableServices, tableChecks)
	}
	return maxIndexWatchTxn(tx, ws, "nodes", tableServices)
}

func catalogUpdateCheckIndexes(tx WriteTxn, idx uint64, _ *structs.EnterpriseMeta) error {
	// update the universal index entry
	if err := tx.Insert(tableIndex, &IndexEntry{tableChecks, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

func catalogChecksMaxIndex(tx ReadTxn, _ *structs.EnterpriseMeta) uint64 {
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

func validateRegisterRequestTxn(_ ReadTxn, _ *structs.RegisterRequest, _ bool) (*structs.EnterpriseMeta, error) {
	return nil, nil
}

func (s *Store) ValidateRegisterRequest(_ *structs.RegisterRequest) (*structs.EnterpriseMeta, error) {
	return nil, nil
}
