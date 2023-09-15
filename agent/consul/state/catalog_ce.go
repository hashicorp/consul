// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func withEnterpriseSchema(_ *memdb.DBSchema) {}

func serviceIndexName(name string, _ *acl.EnterpriseMeta, peerName string) string {
	return peeredIndexEntryName(fmt.Sprintf("service.%s", name), peerName)
}

func serviceKindIndexName(kind structs.ServiceKind, _ *acl.EnterpriseMeta, peerName string) string {
	base := "service_kind." + kind.Normalized()
	return peeredIndexEntryName(base, peerName)
}

func nodeIndexName(name string, _ *acl.EnterpriseMeta, peerName string) string {
	return peeredIndexEntryName(fmt.Sprintf("node.%s", name), peerName)
}

func catalogUpdateNodesIndexes(tx WriteTxn, idx uint64, _ *acl.EnterpriseMeta, peerName string) error {
	// overall nodes index for snapshot and ListNodes RPC
	if err := indexUpdateMaxTxn(tx, idx, tableNodes); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// peered index
	if err := indexUpdateMaxTxn(tx, idx, peeredIndexEntryName(tableNodes, peerName)); err != nil {
		return fmt.Errorf("failed updating partitioned+peered index for nodes table: %w", err)
	}

	return nil
}

// catalogUpdateNodeIndexes upserts the max index for a single node
func catalogUpdateNodeIndexes(tx WriteTxn, idx uint64, nodeName string, _ *acl.EnterpriseMeta, peerName string) error {
	// per-node index
	if err := indexUpdateMaxTxn(tx, idx, nodeIndexName(nodeName, nil, peerName)); err != nil {
		return fmt.Errorf("failed updating node index: %w", err)
	}

	return nil
}

// catalogUpdateServicesIndexes upserts the max index for the entire services table with varying levels
// of granularity (no-op if `idx` is lower than what exists for that index key):
//   - all services
//   - all services in a specified peer (including internal)
func catalogUpdateServicesIndexes(tx WriteTxn, idx uint64, _ *acl.EnterpriseMeta, peerName string) error {
	// overall services index for snapshot
	if err := indexUpdateMaxTxn(tx, idx, tableServices); err != nil {
		return fmt.Errorf("failed updating index for services table: %w", err)
	}

	// peered services index
	if err := indexUpdateMaxTxn(tx, idx, peeredIndexEntryName(tableServices, peerName)); err != nil {
		return fmt.Errorf("failed updating peered index for services table: %w", err)
	}

	return nil
}

// catalogUpdateServiceKindIndexes upserts the max index for the ServiceKind with varying levels
// of granularity (no-op if `idx` is lower than what exists for that index key):
//   - all services of ServiceKind
//   - all services of ServiceKind in a specified peer (including internal)
func catalogUpdateServiceKindIndexes(tx WriteTxn, idx uint64, kind structs.ServiceKind, _ *acl.EnterpriseMeta, peerName string) error {
	base := "service_kind." + kind.Normalized()
	// service-kind index
	if err := indexUpdateMaxTxn(tx, idx, base); err != nil {
		return fmt.Errorf("failed updating index for service kind: %w", err)
	}

	// peered index
	if err := indexUpdateMaxTxn(tx, idx, peeredIndexEntryName(base, peerName)); err != nil {
		return fmt.Errorf("failed updating peered index for service kind: %w", err)
	}
	return nil
}

func catalogUpdateServiceIndexes(tx WriteTxn, idx uint64, serviceName string, _ *acl.EnterpriseMeta, peerName string) error {
	// per-service index
	if err := indexUpdateMaxTxn(tx, idx, serviceIndexName(serviceName, nil, peerName)); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogUpdateServiceExtinctionIndex(tx WriteTxn, idx uint64, _ *acl.EnterpriseMeta, peerName string) error {
	if err := indexUpdateMaxTxn(tx, idx, peeredIndexEntryName(indexServiceExtinction, peerName)); err != nil {
		return fmt.Errorf("failed updating missing service extinction peered index: %w", err)
	}
	return nil
}

func catalogUpdateNodeExtinctionIndex(tx WriteTxn, idx uint64, _ *acl.EnterpriseMeta, peerName string) error {
	if err := indexUpdateMaxTxn(tx, idx, peeredIndexEntryName(indexNodeExtinction, peerName)); err != nil {
		return fmt.Errorf("failed updating missing node extinction peered index: %w", err)
	}
	return nil
}

func catalogInsertNode(tx WriteTxn, node *structs.Node) error {
	// ensure that the Partition is always clear within the state store in CE
	node.Partition = ""

	// Insert the node and update the index.
	if err := tx.Insert(tableNodes, node); err != nil {
		return fmt.Errorf("failed inserting node: %s", err)
	}

	if err := catalogUpdateNodesIndexes(tx, node.ModifyIndex, node.GetEnterpriseMeta(), node.PeerName); err != nil {
		return fmt.Errorf("failed updating nodes indexes: %w", err)
	}
	if err := catalogUpdateNodeIndexes(tx, node.ModifyIndex, node.Node, node.GetEnterpriseMeta(), node.PeerName); err != nil {
		return fmt.Errorf("failed updating node indexes: %w", err)
	}

	// Update the node's service indexes as the node information is included
	// in health queries and we would otherwise miss node updates in some cases
	// for those queries.
	if err := updateAllServiceIndexesOfNode(tx, node.ModifyIndex, node.Node, node.GetEnterpriseMeta(), node.PeerName); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func catalogInsertService(tx WriteTxn, svc *structs.ServiceNode) error {
	// Insert the service and update the index
	if err := tx.Insert(tableServices, svc); err != nil {
		return fmt.Errorf("failed inserting service: %s", err)
	}

	if err := catalogUpdateServicesIndexes(tx, svc.ModifyIndex, &svc.EnterpriseMeta, svc.PeerName); err != nil {
		return fmt.Errorf("failed updating services indexes: %w", err)
	}

	if err := catalogUpdateServiceIndexes(tx, svc.ModifyIndex, svc.ServiceName, &svc.EnterpriseMeta, svc.PeerName); err != nil {
		return fmt.Errorf("failed updating service indexes: %w", err)
	}

	if err := catalogUpdateServiceKindIndexes(tx, svc.ModifyIndex, svc.ServiceKind, &svc.EnterpriseMeta, svc.PeerName); err != nil {
		return fmt.Errorf("failed updating service-kind indexes: %w", err)
	}

	// Update the node indexes as the service information is included in node catalog queries.
	if err := catalogUpdateNodesIndexes(tx, svc.ModifyIndex, &svc.EnterpriseMeta, svc.PeerName); err != nil {
		return fmt.Errorf("failed updating nodes indexes: %w", err)
	}
	if err := catalogUpdateNodeIndexes(tx, svc.ModifyIndex, svc.Node, &svc.EnterpriseMeta, svc.PeerName); err != nil {
		return fmt.Errorf("failed updating node indexes: %w", err)
	}

	return nil
}

func catalogNodesMaxIndex(tx ReadTxn, _ *acl.EnterpriseMeta, peerName string) uint64 {
	return maxIndexTxn(tx, peeredIndexEntryName(tableNodes, peerName))
}

func catalogNodeMaxIndex(tx ReadTxn, nodeName string, _ *acl.EnterpriseMeta, peerName string) uint64 {
	return maxIndexTxn(tx, nodeIndexName(nodeName, nil, peerName))
}

func catalogNodeLastExtinctionIndex(tx ReadTxn, _ *acl.EnterpriseMeta, peerName string) uint64 {
	return maxIndexTxn(tx, peeredIndexEntryName(indexNodeExtinction, peerName))
}

func catalogServicesMaxIndex(tx ReadTxn, _ *acl.EnterpriseMeta, peerName string) uint64 {
	return maxIndexTxn(tx, peeredIndexEntryName(tableServices, peerName))
}

func catalogServiceMaxIndex(tx ReadTxn, serviceName string, _ *acl.EnterpriseMeta, peerName string) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(tableIndex, indexID, serviceIndexName(serviceName, nil, peerName))
}

func catalogServiceKindMaxIndex(tx ReadTxn, ws memdb.WatchSet, kind structs.ServiceKind, _ *acl.EnterpriseMeta, peerName string) uint64 {
	return maxIndexWatchTxn(tx, ws, serviceKindIndexName(kind, nil, peerName))
}

func catalogServiceListNoWildcard(tx ReadTxn, _ *acl.EnterpriseMeta, peerName string) (memdb.ResultIterator, error) {
	q := Query{
		PeerName: peerName,
	}
	return tx.Get(tableServices, indexID+"_prefix", q)
}

func catalogServiceListByNode(tx ReadTxn, node string, _ *acl.EnterpriseMeta, peerName string, _ bool) (memdb.ResultIterator, error) {
	return tx.Get(tableServices, indexNode, Query{Value: node, PeerName: peerName})
}

func catalogServiceLastExtinctionIndex(tx ReadTxn, _ *acl.EnterpriseMeta, peerName string) (interface{}, error) {
	return tx.First(tableIndex, indexID, peeredIndexEntryName(indexServiceExtinction, peerName))
}

func catalogMaxIndex(tx ReadTxn, _ *acl.EnterpriseMeta, peerName string, checks bool) uint64 {
	if checks {
		return maxIndexTxn(tx,
			peeredIndexEntryName(tableChecks, peerName),
			peeredIndexEntryName(tableServices, peerName),
			peeredIndexEntryName(tableNodes, peerName),
		)
	}
	return maxIndexTxn(tx,
		peeredIndexEntryName(tableServices, peerName),
		peeredIndexEntryName(tableNodes, peerName),
	)
}

func catalogMaxIndexWatch(tx ReadTxn, ws memdb.WatchSet, _ *acl.EnterpriseMeta, peerName string, checks bool) uint64 {
	if checks {
		return maxIndexWatchTxn(tx, ws,
			peeredIndexEntryName(tableChecks, peerName),
			peeredIndexEntryName(tableServices, peerName),
			peeredIndexEntryName(tableNodes, peerName),
		)
	}
	return maxIndexWatchTxn(tx, ws,
		peeredIndexEntryName(tableServices, peerName),
		peeredIndexEntryName(tableNodes, peerName),
	)
}

func catalogUpdateCheckIndexes(tx WriteTxn, idx uint64, _ *acl.EnterpriseMeta, peerName string) error {
	// update the overall index entry for snapshot
	if err := indexUpdateMaxTxn(tx, idx, tableChecks); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	if err := indexUpdateMaxTxn(tx, idx, peeredIndexEntryName(tableChecks, peerName)); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

func catalogChecksMaxIndex(tx ReadTxn, _ *acl.EnterpriseMeta, peerName string) uint64 {
	return maxIndexTxn(tx, peeredIndexEntryName(tableChecks, peerName))
}

func catalogListChecksByNode(tx ReadTxn, q Query) (memdb.ResultIterator, error) {
	return tx.Get(tableChecks, indexNode, q)
}

func catalogInsertCheck(tx WriteTxn, chk *structs.HealthCheck, idx uint64) error {
	// Insert the check
	if err := tx.Insert(tableChecks, chk); err != nil {
		return fmt.Errorf("failed inserting check: %s", err)
	}

	if err := catalogUpdateCheckIndexes(tx, idx, &chk.EnterpriseMeta, chk.PeerName); err != nil {
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

func updateKindServiceNamesIndex(tx WriteTxn, idx uint64, kind structs.ServiceKind, entMeta acl.EnterpriseMeta) error {
	if err := indexUpdateMaxTxn(tx, idx, kindServiceNameIndexName(kind.Normalized())); err != nil {
		return fmt.Errorf("failed updating %s table index: %v", tableKindServiceNames, err)
	}
	return nil
}

func indexFromPeeredServiceName(psn structs.PeeredServiceName) ([]byte, error) {
	peer := structs.LocalPeerKeyword
	if psn.Peer != "" {
		// This prefix is unusual but necessary for reads which want
		// to isolate peered resources.
		// This allows you to prefix query for "peer:":
		//   internal/name
		//   peer:peername/name
		peer = "peer:" + psn.Peer
	}

	var b indexBuilder
	b.String(strings.ToLower(peer))
	b.String(strings.ToLower(psn.ServiceName.Name))
	return b.Bytes(), nil
}
