// +build !consulent

package state

import (
	"fmt"
	"strings"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

func withEnterpriseSchema(_ *memdb.DBSchema) {}

func indexNodeServiceFromHealthCheck(raw interface{}) ([]byte, error) {
	hc, ok := raw.(*structs.HealthCheck)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.HealthCheck index", raw)
	}

	if hc.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(hc.Node))
	b.String(strings.ToLower(hc.ServiceID))
	return b.Bytes(), nil
}

func indexFromNodeServiceQuery(arg interface{}) ([]byte, error) {
	hc, ok := arg.(NodeServiceQuery)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for NodeServiceQuery index", arg)
	}

	var b indexBuilder
	b.String(strings.ToLower(hc.Node))
	b.String(strings.ToLower(hc.Service))
	return b.Bytes(), nil
}

func indexFromNode(raw interface{}) ([]byte, error) {
	n, ok := raw.(*structs.Node)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.Node index", raw)
	}

	if n.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(n.Node))
	return b.Bytes(), nil
}

// indexFromNodeQuery builds an index key where Query.Value is lowercase, and is
// a required value.
func indexFromNodeQuery(arg interface{}) ([]byte, error) {
	q, ok := arg.(Query)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for Query index", arg)
	}

	var b indexBuilder
	b.String(strings.ToLower(q.Value))
	return b.Bytes(), nil
}

func indexFromNodeIdentity(raw interface{}) ([]byte, error) {
	n, ok := raw.(interface {
		NodeIdentity() structs.Identity
	})
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for index, type must provide NodeIdentity()", raw)
	}

	id := n.NodeIdentity()
	if id.ID == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(id.ID))
	return b.Bytes(), nil
}

func indexFromServiceNode(raw interface{}) ([]byte, error) {
	n, ok := raw.(*structs.ServiceNode)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ServiceNode index", raw)
	}

	if n.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(n.Node))
	b.String(strings.ToLower(n.ServiceID))
	return b.Bytes(), nil
}

func indexFromHealthCheck(raw interface{}) ([]byte, error) {
	hc, ok := raw.(*structs.HealthCheck)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.HealthCheck index", raw)
	}

	if hc.Node == "" || hc.CheckID == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(hc.Node))
	b.String(strings.ToLower(string(hc.CheckID)))
	return b.Bytes(), nil
}

func indexFromNodeCheckID(raw interface{}) ([]byte, error) {
	hc, ok := raw.(NodeCheckID)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for NodeCheckID index", raw)
	}

	if hc.Node == "" || hc.CheckID == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(hc.Node))
	b.String(strings.ToLower(hc.CheckID))
	return b.Bytes(), nil
}

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
	if err := indexUpdateMaxTxn(tx, idx, "services"); err != nil {
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
	if err := tx.Insert("services", svc); err != nil {
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
	return maxIndexTxn(tx, "services")
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

func catalogServiceListByKind(tx ReadTxn, kind structs.ServiceKind, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("services", "kind", string(kind))
}

func catalogServiceListByNode(tx ReadTxn, node string, _ *structs.EnterpriseMeta, _ bool) (memdb.ResultIterator, error) {
	return tx.Get(tableServices, indexNode, Query{Value: node})
}

func catalogServiceNodeList(tx ReadTxn, name string, index string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("services", index, name)
}

func catalogServiceLastExtinctionIndex(tx ReadTxn, _ *structs.EnterpriseMeta) (interface{}, error) {
	return tx.First(tableIndex, "id", indexServiceExtinction)
}

func catalogMaxIndex(tx ReadTxn, _ *structs.EnterpriseMeta, checks bool) uint64 {
	if checks {
		return maxIndexTxn(tx, "nodes", "services", "checks")
	}
	return maxIndexTxn(tx, "nodes", "services")
}

func catalogMaxIndexWatch(tx ReadTxn, ws memdb.WatchSet, _ *structs.EnterpriseMeta, checks bool) uint64 {
	if checks {
		return maxIndexWatchTxn(tx, ws, "nodes", "services", "checks")
	}
	return maxIndexWatchTxn(tx, ws, "nodes", "services")
}

func catalogUpdateCheckIndexes(tx WriteTxn, idx uint64, _ *structs.EnterpriseMeta) error {
	// update the universal index entry
	if err := tx.Insert(tableIndex, &IndexEntry{"checks", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

func catalogChecksMaxIndex(tx ReadTxn, _ *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "checks")
}

func catalogListChecksByNode(tx ReadTxn, q Query) (memdb.ResultIterator, error) {
	return tx.Get(tableChecks, indexNode, q)
}

func catalogListChecksByService(tx ReadTxn, service string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("checks", "service", service)
}

func catalogListChecksInState(tx ReadTxn, state string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	// simpler than normal due to the use of the CompoundMultiIndex
	return tx.Get("checks", "status", state)
}

func catalogInsertCheck(tx WriteTxn, chk *structs.HealthCheck, idx uint64) error {
	// Insert the check
	if err := tx.Insert("checks", chk); err != nil {
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
