// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/maps"
	"github.com/hashicorp/consul/types"
)

const (
	// indexServiceExtinction keeps track of the last raft index when the last instance
	// of any service was unregistered. This is used by blocking queries on missing services.
	indexServiceExtinction = "service_last_extinction"

	// indexNodeExtinction keeps track of the last raft index when the last instance
	// of any node was unregistered. This is used by blocking queries on missing nodes.
	indexNodeExtinction = "node_last_extinction"
)

const (
	// minUUIDLookupLen is used as a minimum length of a node name required before
	// we test to see if the name is actually a UUID and perform an ID-based node
	// lookup.
	minUUIDLookupLen = 2
)

var (
	// startingVirtualIP is the start of the virtual IP range we assign to services.
	// The effective CIDR range is startingVirtualIP to (startingVirtualIP + virtualIPMaxOffset).
	startingVirtualIP = net.IP{240, 0, 0, 0}

	virtualIPMaxOffset = net.IP{15, 255, 255, 254}

	ErrNodeNotFound = errors.New("node not found")
)

func resizeNodeLookupKey(s string) string {
	l := len(s)

	if l%2 != 0 {
		return s[0 : l-1]
	}

	return s
}

// Nodes is used to pull the full list of nodes for use during snapshots.
func (s *Snapshot) Nodes() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get(tableNodes, indexID)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Services is used to pull the full list of services for a given node for use
// during snapshots.
func (s *Snapshot) Services(node string, entMeta *acl.EnterpriseMeta, peerName string) (memdb.ResultIterator, error) {
	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}
	return s.tx.Get(tableServices, indexNode, Query{
		Value:          node,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
}

// Checks is used to pull the full list of checks for a given node for use
// during snapshots.
func (s *Snapshot) Checks(node string, entMeta *acl.EnterpriseMeta, peerName string) (memdb.ResultIterator, error) {
	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}
	return s.tx.Get(tableChecks, indexNode, Query{
		Value:          node,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
}

// ServiceVirtualIPs is used to pull the service virtual IP mappings for use during snapshots.
func (s *Snapshot) ServiceVirtualIPs() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get(tableServiceVirtualIPs, indexID)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// FreeVirtualIPs is used to pull the freed virtual IPs for use during snapshots.
func (s *Snapshot) FreeVirtualIPs() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get(tableFreeVirtualIPs, indexID)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Registration is used to make sure a node, service, and check registration is
// performed within a single transaction to avoid race conditions on state
// updates.
func (s *Restore) Registration(idx uint64, req *structs.RegisterRequest) error {
	return s.store.ensureRegistrationTxn(s.tx, idx, true, req, true)
}

func (s *Restore) ServiceVirtualIP(req ServiceVirtualIP) error {
	if err := s.tx.Insert(tableServiceVirtualIPs, req); err != nil {
		return err
	}
	if err := updateVirtualIPMaxIndexes(s.tx, req.ModifyIndex, req.Service.ServiceName.PartitionOrDefault(), req.Service.Peer); err != nil {
		return err
	}
	return nil
}

func (s *Restore) FreeVirtualIP(req FreeVirtualIP) error {
	return s.tx.Insert(tableFreeVirtualIPs, req)
}

// EnsureRegistration is used to make sure a node, service, and check
// registration is performed within a single transaction to avoid race
// conditions on state updates.
func (s *Store) EnsureRegistration(idx uint64, req *structs.RegisterRequest) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := s.ensureRegistrationTxn(tx, idx, false, req, false); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ensureCheckIfNodeMatches(
	tx WriteTxn,
	idx uint64,
	preserveIndexes bool,
	node string,
	nodePartition string,
	nodePeerName string,
	check *structs.HealthCheck,
) error {
	if !strings.EqualFold(check.PeerName, nodePeerName) {
		return fmt.Errorf("check peer name %q does not match node peer name %q", check.PeerName, nodePeerName)
	}
	if !strings.EqualFold(check.Node, node) || !acl.EqualPartitions(nodePartition, check.PartitionOrDefault()) {
		return fmt.Errorf("check node %q does not match node %q",
			printNodeName(check.Node, check.PartitionOrDefault()),
			printNodeName(node, nodePartition),
		)
	}
	if err := s.ensureCheckTxn(tx, idx, preserveIndexes, check); err != nil {
		return fmt.Errorf("failed inserting check on node %q: %v", printNodeName(check.Node, check.PartitionOrDefault()), err)
	}
	return nil
}

func printNodeName(nodeName, partition string) string {
	if acl.IsDefaultPartition(partition) {
		return nodeName
	}
	return partition + "/" + nodeName
}

// ensureRegistrationTxn is used to make sure a node, service, and check
// registration is performed within a single transaction to avoid race
// conditions on state updates.
func (s *Store) ensureRegistrationTxn(tx WriteTxn, idx uint64, preserveIndexes bool, req *structs.RegisterRequest, restore bool) error {
	if err := validateRegisterRequestPeerNamesTxn(tx, req, restore); err != nil {
		return err
	}
	if _, err := validateRegisterRequestTxn(tx, req, restore); err != nil {
		return err
	}

	// Create a node structure.
	node := &structs.Node{
		ID:              req.ID,
		Node:            req.Node,
		Address:         req.Address,
		Datacenter:      req.Datacenter,
		Partition:       req.PartitionOrDefault(),
		TaggedAddresses: req.TaggedAddresses,
		Meta:            req.NodeMeta,
		PeerName:        req.PeerName,
		Locality:        req.Locality,
	}
	if preserveIndexes {
		node.CreateIndex = req.CreateIndex
		node.ModifyIndex = req.ModifyIndex
	}

	// Since this gets called for all node operations (service and check
	// updates) and churn on the node itself is basically none after the
	// node updates itself the first time, it's worth seeing if we need to
	// modify the node at all so we prevent watch churn and useless writes
	// and modify index bumps on the node.
	{
		existing, err := tx.First(tableNodes, indexID, Query{
			Value:          node.Node,
			EnterpriseMeta: *node.GetEnterpriseMeta(),
			PeerName:       node.PeerName,
		})
		if err != nil {
			return fmt.Errorf("node lookup failed: %s", err)
		}
		if existing == nil || req.ChangesNode(existing.(*structs.Node)) {
			if err := s.ensureNodeTxn(tx, idx, preserveIndexes, node); err != nil {
				return fmt.Errorf("failed inserting node: %s", err)
			}
		}
	}

	// Add the service, if any. We perform a similar check as we do for the
	// node info above to make sure we actually need to update the service
	// definition in order to prevent useless churn if nothing has changed.
	if req.Service != nil {
		existing, err := tx.First(tableServices, indexID, NodeServiceQuery{
			EnterpriseMeta: req.Service.EnterpriseMeta,
			Node:           req.Node,
			Service:        req.Service.ID,
			PeerName:       req.PeerName,
		})
		if err != nil {
			return fmt.Errorf("failed service lookup: %s", err)
		}
		if existing == nil || !(existing.(*structs.ServiceNode).ToNodeService()).IsSame(req.Service) {
			if err := ensureServiceTxn(tx, idx, req.Node, preserveIndexes, req.Service); err != nil {
				return fmt.Errorf("failed inserting service: %s", err)

			}
		}
	}

	// Add the checks, if any.
	if req.Check != nil {
		err := s.ensureCheckIfNodeMatches(tx, idx, preserveIndexes, req.Node, req.PartitionOrDefault(), req.PeerName, req.Check)
		if err != nil {
			return err
		}
	}
	for _, check := range req.Checks {
		err := s.ensureCheckIfNodeMatches(tx, idx, preserveIndexes, req.Node, req.PartitionOrDefault(), req.PeerName, check)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateRegisterRequestPeerNamesTxn(_ ReadTxn, args *structs.RegisterRequest, _ bool) error {
	var (
		peerNames = make(map[string]struct{})
	)
	if args.Service != nil {
		if args.Service.PeerName == "" {
			args.Service.PeerName = args.PeerName
		}

		peerName := args.Service.PeerName
		// TODO(peering): validate the peering exists (skip check on restore)

		peerNames[peerName] = struct{}{}
	}

	validateCheck := func(chk *structs.HealthCheck) error {
		if chk.PeerName == "" {
			chk.PeerName = args.PeerName
		}

		peerName := chk.PeerName
		// TODO(peering): validate the peering exists (skip check on restore)

		peerNames[peerName] = struct{}{}

		return nil
	}

	if args.Check != nil {
		if err := validateCheck(args.Check); err != nil {
			return err
		}
	}
	for _, chk := range args.Checks {
		if err := validateCheck(chk); err != nil {
			return err
		}
	}

	{
		// TODO(peering): validate the node's peering exists (skip check on restore)
		peerName := args.PeerName
		peerNames[peerName] = struct{}{}
	}

	if len(peerNames) > 1 {
		return fmt.Errorf("Cannot register services and checks for multiple peer names in one registration request")
	} else if len(peerNames) == 0 {
		return fmt.Errorf("No peer names are present on the registration request; this makes no sense")
	}

	return nil

}

// EnsureNode is used to upsert node registration or modification.
func (s *Store) EnsureNode(idx uint64, node *structs.Node) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Call the node upsert
	if err := s.ensureNodeTxn(tx, idx, false, node); err != nil {
		return err
	}

	return tx.Commit()
}

// ensureNoNodeWithSimilarNameTxn checks that no other node has conflict in its name
// If allowClashWithoutID then, getting a conflict on another node without ID will be allowed
func ensureNoNodeWithSimilarNameTxn(tx ReadTxn, node *structs.Node, allowClashWithoutID bool) error {
	// Retrieve all of the nodes
	q := Query{
		PeerName:       node.PeerName,
		EnterpriseMeta: *node.GetEnterpriseMeta(),
	}
	enodes, err := tx.Get(tableNodes, indexID+"_prefix", q)
	if err != nil {
		return fmt.Errorf("Cannot lookup all nodes: %s", err)
	}
	for nodeIt := enodes.Next(); nodeIt != nil; nodeIt = enodes.Next() {
		enode := nodeIt.(*structs.Node)
		if strings.EqualFold(node.Node, enode.Node) && node.ID != enode.ID {
			// Look up the existing node's Serf health check to see if it's failed.
			// If it is, the node can be renamed.
			enodeCheck, err := tx.First(tableChecks, indexID, NodeCheckQuery{
				EnterpriseMeta: *node.GetEnterpriseMeta(),
				Node:           enode.Node,
				CheckID:        string(structs.SerfCheckID),
				PeerName:       enode.PeerName,
			})
			if err != nil {
				return fmt.Errorf("Cannot get status of node %s: %s", enode.Node, err)
			}

			// Get the node health. If there's no Serf health check, we consider it safe to rename
			// the node as it's likely an external node registration not managed by Consul.
			var nodeHealthy bool
			if enodeCheck != nil {
				enodeSerfCheck, ok := enodeCheck.(*structs.HealthCheck)
				if ok {
					nodeHealthy = enodeSerfCheck.Status != api.HealthCritical
				}
			}

			if !(enode.ID == "" && allowClashWithoutID) && nodeHealthy {
				return fmt.Errorf("Node name %s is reserved by node %s with name %s (%s)", node.Node, enode.ID, enode.Node, enode.Address)
			}
		}
	}
	return nil
}

// ensureNodeCASTxn updates a node only if the existing index matches the given index.
// Returns a bool indicating if a write happened and any error.
func (s *Store) ensureNodeCASTxn(tx WriteTxn, idx uint64, node *structs.Node) (bool, error) {
	// Retrieve the existing entry.
	existing, err := getNodeTxn(tx, node.Node, node.GetEnterpriseMeta(), node.PeerName)
	if err != nil {
		return false, err
	}

	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	if node.ModifyIndex == 0 && existing != nil {
		return false, nil
	}
	if node.ModifyIndex != 0 && existing == nil {
		return false, nil
	}
	if existing != nil && node.ModifyIndex != 0 && node.ModifyIndex != existing.ModifyIndex {
		return false, nil
	}

	// Perform the update.
	if err := s.ensureNodeTxn(tx, idx, false, node); err != nil {
		return false, err
	}

	return true, nil
}

// ensureNodeTxn is the inner function called to actually create a node
// registration or modify an existing one in the state store. It allows
// passing in a memdb transaction so it may be part of a larger txn.
func (s *Store) ensureNodeTxn(tx WriteTxn, idx uint64, preserveIndexes bool, node *structs.Node) error {
	// See if there's an existing node with this UUID, and make sure the
	// name is the same.
	var n *structs.Node
	if node.ID != "" {
		existing, err := getNodeIDTxn(tx, node.ID, node.GetEnterpriseMeta(), node.PeerName)
		if err != nil {
			return fmt.Errorf("node lookup failed: %s", err)
		}
		if existing != nil {
			n = existing
			if !strings.EqualFold(n.Node, node.Node) {
				// Lets first get all nodes and check whether name do match, we do not allow clash on nodes without ID
				dupNameError := ensureNoNodeWithSimilarNameTxn(tx, node, false)
				if dupNameError != nil {
					return fmt.Errorf("Error while renaming Node ID: %q (%s): %s", node.ID, node.Address, dupNameError)
				}
				// We are actually renaming a node, remove its reference first
				err := s.deleteNodeTxn(tx, idx, n.Node, n.GetEnterpriseMeta(), n.PeerName)
				if err != nil {
					return fmt.Errorf("Error while renaming Node ID: %q (%s) from %s to %s: %w",
						node.ID, node.Address, n.Node, node.Node, err)
				}
			}
		} else {
			// We allow to "steal" another node name that would have no ID
			// It basically means that we allow upgrading a node without ID and add the ID
			dupNameError := ensureNoNodeWithSimilarNameTxn(tx, node, true)
			if dupNameError != nil {
				return fmt.Errorf("Error while renaming Node ID: %q: %s", node.ID, dupNameError)
			}
		}
	}
	// TODO: else Node.ID == "" should be forbidden in future Consul releases
	// See https://github.com/hashicorp/consul/pull/3983 for context

	// Check for an existing node by name to support nodes with no IDs.
	if n == nil {
		existing, err := tx.First(tableNodes, indexID, Query{
			Value:          node.Node,
			EnterpriseMeta: *node.GetEnterpriseMeta(),
			PeerName:       node.PeerName,
		})
		if err != nil {
			return fmt.Errorf("node name lookup failed: %s", err)
		}

		if existing != nil {
			n = existing.(*structs.Node)
		}
		// WARNING, for compatibility reasons with tests, we do not check
		// for case insensitive matches, which may lead to DB corruption
		// See https://github.com/hashicorp/consul/pull/3983 for context
	}

	// Get the indexes.
	if n != nil {
		node.CreateIndex = n.CreateIndex
		node.ModifyIndex = n.ModifyIndex
		// We do not need to update anything
		if node.IsSame(n) {
			return nil
		}
		node.ModifyIndex = idx
	} else if !preserveIndexes || node.CreateIndex == 0 {
		// If this isn't a snapshot or there were no saved indexes, set CreateIndex
		// and ModifyIndex from the given index. Prior to 1.9.0/1.8.3/1.7.7, nodes
		// were not saved with an index, so this is to avoid ending up with a 0 index
		// when loading a snapshot from an older version.
		node.CreateIndex = idx
		node.ModifyIndex = idx
	}

	// Insert the node and update the index.
	return catalogInsertNode(tx, node)
}

// GetNode is used to retrieve a node registration by node name ID.
func (s *Store) GetNode(nodeNameOrID string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.Node, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Get the table index.
	idx := catalogNodesMaxIndex(tx, entMeta, peerName)

	// Retrieve the node from the state store
	node, err := getNodeTxn(tx, nodeNameOrID, entMeta, peerName)
	if err != nil {
		return 0, nil, fmt.Errorf("node lookup failed: %s", err)
	}
	return idx, node, nil
}

func getNodeTxn(tx ReadTxn, nodeNameOrID string, entMeta *acl.EnterpriseMeta, peerName string) (*structs.Node, error) {
	node, err := tx.First(tableNodes, indexID, Query{
		Value:          nodeNameOrID,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return nil, fmt.Errorf("node lookup failed: %s", err)
	}
	if node != nil {
		return node.(*structs.Node), nil
	}
	return nil, nil
}

func getNodeIDTxn(tx ReadTxn, id types.NodeID, entMeta *acl.EnterpriseMeta, peerName string) (*structs.Node, error) {
	node, err := tx.First(tableNodes, indexUUID+"_prefix", Query{
		Value:          string(id),
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return nil, fmt.Errorf("node lookup by ID failed: %s", err)
	}
	if node != nil {
		return node.(*structs.Node), nil
	}
	return nil, nil
}

// GetNodeID is used to retrieve a node registration by node ID.
func (s *Store) GetNodeID(id types.NodeID, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.Node, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	idx := catalogNodesMaxIndex(tx, entMeta, peerName)

	// Retrieve the node from the state store
	node, err := getNodeIDTxn(tx, id, entMeta, peerName)
	return idx, node, err
}

// Nodes is used to return all of the known nodes.
func (s *Store) Nodes(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.Nodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	idx := catalogNodesMaxIndex(tx, entMeta, peerName)

	// Retrieve all of the nodes
	q := Query{
		PeerName:       peerName,
		EnterpriseMeta: *entMeta,
	}
	nodes, err := tx.Get(tableNodes, indexID+"_prefix", q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	ws.Add(nodes.WatchCh())

	// Create and return the nodes list.
	var results structs.Nodes
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		results = append(results, node.(*structs.Node))
	}
	return idx, results, nil
}

// NodesByMeta is used to return all nodes with the given metadata key/value pairs.
func (s *Store) NodesByMeta(ws memdb.WatchSet, filters map[string]string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.Nodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	idx := catalogNodesMaxIndex(tx, entMeta, peerName)

	if len(filters) == 0 {
		return idx, nil, nil // NodesByMeta is never called with an empty map, but just in case make it return no results.
	}

	// Retrieve all of the nodes. We'll do a lookup of just ONE KV pair, which
	// over-matches if multiple pairs are requested, but then in the loop below
	// we'll finish filtering.
	var firstKey, firstValue string
	for firstKey, firstValue = range filters {
		break
	}

	nodes, err := tx.Get(tableNodes, indexMeta, KeyValueQuery{
		Key:            firstKey,
		Value:          firstValue,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	ws.Add(nodes.WatchCh())

	// Create and return the nodes list.
	var results structs.Nodes
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		n := node.(*structs.Node)
		if len(filters) <= 1 || structs.SatisfiesMetaFilters(n.Meta, filters) {
			results = append(results, n)
		}
	}
	return idx, results, nil
}

// DeleteNode is used to delete a given node by its ID.
func (s *Store) DeleteNode(idx uint64, nodeName string, entMeta *acl.EnterpriseMeta, peerName string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Call the node deletion.
	if err := s.deleteNodeTxn(tx, idx, nodeName, entMeta, peerName); err != nil {
		return err
	}

	return tx.Commit()
}

// deleteNodeCASTxn is used to try doing a node delete operation with a given
// raft index. If the CAS index specified is not equal to the last observed index for
// the given check, then the call is a noop, otherwise a normal check delete is invoked.
func (s *Store) deleteNodeCASTxn(tx WriteTxn, idx, cidx uint64, nodeName string, entMeta *acl.EnterpriseMeta, peerName string) (bool, error) {
	// Look up the node.
	node, err := getNodeTxn(tx, nodeName, entMeta, peerName)
	if err != nil {
		return false, err
	}
	if node == nil {
		return false, nil
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	if node.ModifyIndex != cidx {
		return false, nil
	}

	// Call the actual deletion if the above passed.
	if err := s.deleteNodeTxn(tx, idx, nodeName, entMeta, peerName); err != nil {
		return false, err
	}

	return true, nil
}

// deleteNodeTxn is the inner method used for removing a node from
// the store within a given transaction.
func (s *Store) deleteNodeTxn(tx WriteTxn, idx uint64, nodeName string, entMeta *acl.EnterpriseMeta, peerName string) error {
	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// Look up the node.
	nodeRaw, err := tx.First(tableNodes, indexID, Query{
		Value:          nodeName,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return fmt.Errorf("node lookup failed: %s", err)
	}
	if nodeRaw == nil {
		return nil
	}

	// Delete all services associated with the node and update the service index.
	services, err := tx.Get(tableServices, indexNode, Query{
		Value:          nodeName,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}
	var deleteServices []*structs.ServiceNode
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		deleteServices = append(deleteServices, svc)

		if err := catalogUpdateServiceIndexes(tx, idx, svc.ServiceName, &svc.EnterpriseMeta, svc.PeerName); err != nil {
			return err
		}
		if err := catalogUpdateServiceKindIndexes(tx, idx, svc.ServiceKind, &svc.EnterpriseMeta, svc.PeerName); err != nil {
			return err
		}
	}

	// Do the delete in a separate loop so we don't trash the iterator.
	for _, svc := range deleteServices {
		if err := s.deleteServiceTxn(tx, idx, nodeName, svc.ServiceID, &svc.EnterpriseMeta, svc.PeerName); err != nil {
			return err
		}
	}

	// Delete all checks associated with the node. This will invalidate
	// sessions as necessary.
	checks, err := tx.Get(tableChecks, indexNode, Query{
		Value:          nodeName,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return fmt.Errorf("failed check lookup: %s", err)
	}
	var deleteChecks []*structs.HealthCheck
	for check := checks.Next(); check != nil; check = checks.Next() {
		deleteChecks = append(deleteChecks, check.(*structs.HealthCheck))
	}

	// Do the delete in a separate loop so we don't trash the iterator.
	for _, chk := range deleteChecks {
		if err := s.deleteCheckTxn(tx, idx, nodeName, chk.CheckID, &chk.EnterpriseMeta, chk.PeerName); err != nil {
			return err
		}
	}

	if peerName == "" {
		// Delete any coordinates associated with this node.
		coords, err := tx.Get(tableCoordinates, indexNode, Query{
			Value:          nodeName,
			EnterpriseMeta: *entMeta,
			PeerName:       structs.DefaultPeerKeyword,
		})
		if err != nil {
			return fmt.Errorf("failed coordinate lookup: %s", err)
		}
		var coordsToDelete []*structs.Coordinate
		for coord := coords.Next(); coord != nil; coord = coords.Next() {
			coordsToDelete = append(coordsToDelete, coord.(*structs.Coordinate))
		}
		for _, coord := range coordsToDelete {
			if err := deleteCoordinateTxn(tx, idx, coord); err != nil {
				return fmt.Errorf("failed deleting coordinate: %s", err)
			}
		}
	}

	// Delete the node and update the index.
	if err := tx.Delete(tableNodes, nodeRaw); err != nil {
		return fmt.Errorf("failed deleting node: %s", err)
	}
	node := nodeRaw.(*structs.Node)
	if err := catalogUpdateNodesIndexes(tx, idx, entMeta, node.PeerName); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// Clean up node entry from index table
	if err := tx.Delete(tableIndex, &IndexEntry{Key: nodeIndexName(nodeName, entMeta, node.PeerName)}); err != nil {
		return fmt.Errorf("failed deleting nodeIndex %q: %w", nodeIndexName(nodeName, entMeta, node.PeerName), err)
	}

	if err := catalogUpdateNodeExtinctionIndex(tx, idx, entMeta, node.PeerName); err != nil {
		return err
	}

	if peerName == "" {
		// Invalidate any sessions for this node.
		toDelete, err := allNodeSessionsTxn(tx, nodeName, entMeta.PartitionOrDefault())
		if err != nil {
			return err
		}

		for _, session := range toDelete {
			if err := s.deleteSessionTxn(tx, idx, session.ID, &session.EnterpriseMeta); err != nil {
				return fmt.Errorf("failed to delete session '%s': %v", session.ID, err)
			}
		}
	}

	return nil
}

// EnsureService is called to upsert creation of a given NodeService.
func (s *Store) EnsureService(idx uint64, node string, svc *structs.NodeService) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Call the service registration upsert
	if err := ensureServiceTxn(tx, idx, node, false, svc); err != nil {
		return err
	}

	return tx.Commit()
}

var errCASCompareFailed = errors.New("compare-and-set: comparison failed")

// ensureServiceCASTxn updates a service only if the existing index matches the given index.
// Returns an error if the write didn't happen and nil if write was successful.
func ensureServiceCASTxn(tx WriteTxn, idx uint64, node string, svc *structs.NodeService) error {
	// Retrieve the existing service.
	existing, err := tx.First(tableServices, indexID,
		NodeServiceQuery{
			EnterpriseMeta: svc.EnterpriseMeta,
			Node:           node,
			Service:        svc.ID,
			PeerName:       svc.PeerName,
		})
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}

	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	if svc.ModifyIndex == 0 && existing != nil {
		return errCASCompareFailed
	}
	if svc.ModifyIndex != 0 && existing == nil {
		return errCASCompareFailed
	}
	e, ok := existing.(*structs.ServiceNode)
	if ok && svc.ModifyIndex != 0 && svc.ModifyIndex != e.ModifyIndex {
		return errCASCompareFailed
	}

	return ensureServiceTxn(tx, idx, node, false, svc)
}

// ensureServiceTxn is used to upsert a service registration within an
// existing memdb transaction.
func ensureServiceTxn(tx WriteTxn, idx uint64, node string, preserveIndexes bool, svc *structs.NodeService) error {
	// Check for existing service
	existing, err := tx.First(tableServices, indexID, NodeServiceQuery{
		EnterpriseMeta: svc.EnterpriseMeta,
		Node:           node,
		Service:        svc.ID,
		PeerName:       svc.PeerName,
	})
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}

	if err = structs.ValidateServiceMetadata(svc.Kind, svc.Meta, false); err != nil {
		return fmt.Errorf("Invalid Service Meta for node %s and serviceID %s: %v", node, svc.ID, err)
	}

	if svc.PeerName == "" {
		// Do not associate non-typical services with gateways or consul services
		if svc.Kind == structs.ServiceKindTypical && svc.Service != "consul" {
			// Check if this service is covered by a gateway's wildcard specifier, we force the service kind to a gateway-service here as that take precedence
			sn := structs.NewServiceName(svc.Service, &svc.EnterpriseMeta)
			if err = checkGatewayWildcardsAndUpdate(tx, idx, &sn, svc, structs.GatewayServiceKindService); err != nil {
				return fmt.Errorf("failed updating gateway mapping: %s", err)
			}
			if err = checkGatewayAndUpdate(tx, idx, &sn, structs.GatewayServiceKindService); err != nil {
				return fmt.Errorf("failed updating gateway mapping: %s", err)
			}
		}
		// Only upsert KindServiceName if service is local
		if err := upsertKindServiceName(tx, idx, svc.Kind, svc.CompoundServiceName()); err != nil {
			return fmt.Errorf("failed to persist service name: %v", err)
		}
	}

	// Update upstream/downstream mappings if it's a connect service
	if svc.Kind == structs.ServiceKindConnectProxy || svc.Connect.Native {
		if err = updateMeshTopology(tx, idx, node, svc, existing); err != nil {
			return fmt.Errorf("failed updating upstream/downstream association")
		}

		service := svc.Service
		if svc.Kind == structs.ServiceKindConnectProxy {
			service = svc.Proxy.DestinationServiceName
		}
		sn := structs.ServiceName{Name: service, EnterpriseMeta: svc.EnterpriseMeta}
		if err = checkGatewayWildcardsAndUpdate(tx, idx, &sn, svc, structs.GatewayServiceKindService); err != nil {
			return fmt.Errorf("failed updating gateway mapping: %s", err)
		}

		if svc.PeerName == "" && sn.Name != "" {
			if err := upsertKindServiceName(tx, idx, structs.ServiceKindConnectEnabled, sn); err != nil {
				return fmt.Errorf("failed to persist service name as connect-enabled: %v", err)
			}
		}

		// Update the virtual IP for the service
		supported, err := virtualIPsSupported(tx, nil)
		if err != nil {
			return err
		}
		if supported && sn.Name != "" {
			psn := structs.PeeredServiceName{Peer: svc.PeerName, ServiceName: sn}
			vip, err := assignServiceVirtualIP(tx, idx, psn)
			if err != nil {
				return fmt.Errorf("failed updating virtual IP: %s", err)
			}
			if svc.TaggedAddresses == nil {
				svc.TaggedAddresses = make(map[string]structs.ServiceAddress)
			}
			svc.TaggedAddresses[structs.TaggedAddressVirtualIP] = structs.ServiceAddress{Address: vip, Port: svc.Port}
		}
	}

	if svc.PeerName == "" {
		// If there's a terminating gateway config entry for this service, populate the tagged addresses
		// with virtual IP mappings.
		termGatewayVIPsSupported, err := terminatingGatewayVirtualIPsSupported(tx, nil)
		if err != nil {
			return err
		}
		if termGatewayVIPsSupported && svc.Kind == structs.ServiceKindTerminatingGateway {
			_, conf, err := configEntryTxn(tx, nil, structs.TerminatingGateway, svc.Service, &svc.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("failed to retrieve terminating gateway config: %s", err)
			}
			if conf != nil {
				termGatewayConf := conf.(*structs.TerminatingGatewayConfigEntry)
				addrs, err := getTermGatewayVirtualIPs(tx, idx, termGatewayConf.Services, &svc.EnterpriseMeta)
				if err != nil {
					return err
				}
				if svc.TaggedAddresses == nil {
					svc.TaggedAddresses = make(map[string]structs.ServiceAddress)
				}
				for key, addr := range addrs {
					svc.TaggedAddresses[key] = addr
				}
			}
		}
	}

	// Create the service node entry and populate the indexes. Note that
	// conversion doesn't populate any of the node-specific information.
	// That's always populated when we read from the state store.
	entry := svc.ToServiceNode(node)
	// Get the node
	n, err := tx.First(tableNodes, indexID, Query{
		Value:          node,
		EnterpriseMeta: svc.EnterpriseMeta,
		PeerName:       svc.PeerName,
	})
	if err != nil {
		return fmt.Errorf("failed node lookup: %s", err)
	}
	if n == nil {
		return ErrMissingNode
	}
	if existing != nil {
		serviceNode := existing.(*structs.ServiceNode)
		entry.CreateIndex = serviceNode.CreateIndex
		entry.ModifyIndex = serviceNode.ModifyIndex
		// We cannot return here because: we want to keep existing behavior (ex: failed node lookup -> ErrMissingNode)
		// It might be modified in future, but it requires changing many unit tests
		// Enforcing saving the entry also ensures that if we add default values in .ToServiceNode()
		// those values will be saved even if node is not really modified for a while.
		if entry.IsSameService(serviceNode) {
			return nil
		}
	}
	if !preserveIndexes {
		entry.ModifyIndex = idx
		if existing == nil {
			entry.CreateIndex = idx
		}
	}

	// Insert the service and update the index
	return catalogInsertService(tx, entry)
}

// assignServiceVirtualIP assigns a virtual IP to the target service and updates
// the global virtual IP counter if necessary.
func assignServiceVirtualIP(tx WriteTxn, idx uint64, psn structs.PeeredServiceName) (string, error) {
	serviceVIP, err := tx.First(tableServiceVirtualIPs, indexID, psn)
	if err != nil {
		return "", fmt.Errorf("failed service virtual IP lookup: %s", err)
	}

	// Service already has a virtual IP assigned, nothing to do.
	if serviceVIP != nil {
		sVIP := serviceVIP.(ServiceVirtualIP).IP
		result, err := addIPOffset(startingVirtualIP, sVIP)
		if err != nil {
			return "", err
		}

		return result.String(), nil
	}

	// Get the next available virtual IP, drawing from any freed from deleted services
	// first and then falling back to the global counter if none are available.
	latestVIP, err := tx.First(tableFreeVirtualIPs, indexCounterOnly, false)
	if err != nil {
		return "", fmt.Errorf("failed virtual IP index lookup: %s", err)
	}
	if latestVIP == nil {
		latestVIP, err = tx.First(tableFreeVirtualIPs, indexCounterOnly, true)
		if err != nil {
			return "", fmt.Errorf("failed virtual IP index lookup: %s", err)
		}
	}
	if latestVIP != nil {
		if err := tx.Delete(tableFreeVirtualIPs, latestVIP); err != nil {
			return "", fmt.Errorf("failed updating freed virtual IP table: %v", err)
		}
	}

	var latest FreeVirtualIP
	if latestVIP == nil {
		latest = FreeVirtualIP{
			IP:        net.IPv4zero,
			IsCounter: true,
		}
	} else {
		latest = latestVIP.(FreeVirtualIP)
	}

	// Store the next virtual IP from the counter if there aren't any freed IPs to draw from.
	// Then increment to store the next free virtual IP.
	newEntry := FreeVirtualIP{
		IP:        latest.IP,
		IsCounter: latest.IsCounter,
	}
	if latest.IsCounter {
		newEntry.IP = make(net.IP, len(latest.IP))
		copy(newEntry.IP, latest.IP)
		for i := len(newEntry.IP) - 1; i >= 0; i-- {
			newEntry.IP[i]++
			if newEntry.IP[i] != 0 {
				break
			}
		}

		// Out of virtual IPs, fail registration.
		if newEntry.IP.Equal(virtualIPMaxOffset) {
			return "", fmt.Errorf("cannot allocate any more unique service virtual IPs")
		}

		if err := tx.Insert(tableFreeVirtualIPs, newEntry); err != nil {
			return "", fmt.Errorf("failed updating freed virtual IP table: %v", err)
		}
	}

	assignedVIP := ServiceVirtualIP{
		Service: psn,
		IP:      newEntry.IP,
		RaftIndex: structs.RaftIndex{
			ModifyIndex: idx,
			CreateIndex: idx,
		},
	}
	if err := tx.Insert(tableServiceVirtualIPs, assignedVIP); err != nil {
		return "", fmt.Errorf("failed inserting service virtual IP entry: %s", err)
	}
	if err := updateVirtualIPMaxIndexes(tx, idx, psn.ServiceName.PartitionOrDefault(), psn.Peer); err != nil {
		return "", err
	}

	result, err := addIPOffset(startingVirtualIP, assignedVIP.IP)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

// AssignManualServiceVIPs attempts to associate a list of manual virtual IP addresses with a given service name.
// Any IP addresses given will be removed from other services in the same partition. This is done to ensure
// that a manual VIP can only exist once for a given partition.
// This function returns:
// - a bool indicating whether the given service exists.
// - a list of service names that had ip addresses removed from them.
// - an error indicating success or failure of the call.
func (s *Store) AssignManualServiceVIPs(idx uint64, psn structs.PeeredServiceName, ips []string) (bool, []structs.PeeredServiceName, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// First remove the given IPs from any existing services, to avoid duplicate assignments.
	assignedIPs := map[string]struct{}{}
	for _, ip := range ips {
		assignedIPs[ip] = struct{}{}
	}
	modifiedEntries := make(map[structs.PeeredServiceName]struct{})
	for ip := range assignedIPs {
		entry, err := tx.First(tableServiceVirtualIPs, indexManualVIPs, psn.ServiceName.PartitionOrDefault(), ip)
		if err != nil {
			return false, nil, fmt.Errorf("failed service virtual IP lookup: %s", err)
		}

		if entry == nil {
			continue
		}

		newEntry := entry.(ServiceVirtualIP)
		if newEntry.Service.ServiceName.Matches(psn.ServiceName) {
			continue
		}

		// Rebuild this entry's list of manual IPs, removing any that are present
		// in new list we're assigning.
		var filteredIPs []string
		for _, existingIP := range newEntry.ManualIPs {
			if _, ok := assignedIPs[existingIP]; !ok {
				filteredIPs = append(filteredIPs, existingIP)
			}
		}

		newEntry.ManualIPs = filteredIPs
		newEntry.ModifyIndex = idx
		if err := tx.Insert(tableServiceVirtualIPs, newEntry); err != nil {
			return false, nil, fmt.Errorf("failed inserting service virtual IP entry: %s", err)
		}
		modifiedEntries[newEntry.Service] = struct{}{}
	}

	entry, err := tx.First(tableServiceVirtualIPs, indexID, psn)
	if err != nil {
		return false, nil, fmt.Errorf("failed service virtual IP lookup: %s", err)
	}

	if entry == nil {
		return false, nil, nil
	}

	newEntry := entry.(ServiceVirtualIP)
	newEntry.ManualIPs = ips
	newEntry.ModifyIndex = idx

	if err := tx.Insert(tableServiceVirtualIPs, newEntry); err != nil {
		return false, nil, fmt.Errorf("failed inserting service virtual IP entry: %s", err)
	}
	if err := updateVirtualIPMaxIndexes(tx, idx, psn.ServiceName.PartitionOrDefault(), psn.Peer); err != nil {
		return false, nil, err
	}
	if err = tx.Commit(); err != nil {
		return false, nil, err
	}

	return true, maps.SliceOfKeys(modifiedEntries), nil
}

func updateVirtualIPMaxIndexes(txn WriteTxn, idx uint64, partition, peerName string) error {
	// update per-partition max index
	if err := indexUpdateMaxTxn(txn, idx, partitionedIndexEntryName(tableServiceVirtualIPs, partition)); err != nil {
		return fmt.Errorf("failed while updating partitioned index: %w", err)
	}
	if peerName != "" {
		// track a separate max index for imported services
		if err := indexUpdateMaxTxn(txn, idx, partitionedIndexEntryName(tableServiceVirtualIPs+".imported", partition)); err != nil {
			return fmt.Errorf("failed while updating partitioned index for imported services: %w", err)
		}
	}
	return nil
}

func addIPOffset(a, b net.IP) (net.IP, error) {
	a4 := a.To4()
	b4 := b.To4()
	if a4 == nil || b4 == nil {
		return nil, errors.New("ip is not ipv4")
	}

	var raw uint64
	for i := 0; i < 4; i++ {
		raw = raw<<8 + uint64(a4[i]) + uint64(b4[i])
	}
	return net.IPv4(byte(raw>>24), byte(raw>>16), byte(raw>>8), byte(raw)), nil
}

func virtualIPsSupported(tx ReadTxn, ws memdb.WatchSet) (bool, error) {
	_, entry, err := systemMetadataGetTxn(tx, ws, structs.SystemMetadataVirtualIPsEnabled)
	if err != nil {
		return false, fmt.Errorf("failed system metadata lookup: %s", err)
	}
	if entry == nil {
		return false, nil
	}

	return entry.Value != "", nil
}

func terminatingGatewayVirtualIPsSupported(tx ReadTxn, ws memdb.WatchSet) (bool, error) {
	_, entry, err := systemMetadataGetTxn(tx, ws, structs.SystemMetadataTermGatewayVirtualIPsEnabled)
	if err != nil {
		return false, fmt.Errorf("failed system metadata lookup: %s", err)
	}
	if entry == nil {
		return false, nil
	}

	return entry.Value != "", nil
}

// Services returns all services along with a list of associated tags.
func (s *Store) Services(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta, peerName string) (uint64, []*structs.ServiceNode, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := catalogServicesMaxIndex(tx, entMeta, peerName)

	// List all the services.
	services, err := catalogServiceListNoWildcard(tx, entMeta, peerName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying services: %s", err)
	}
	ws.Add(services.WatchCh())

	var result []*structs.ServiceNode
	for service := services.Next(); service != nil; service = services.Next() {
		result = append(result, service.(*structs.ServiceNode))
	}
	return idx, result, nil
}

func (s *Store) ServiceList(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.ServiceList, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return serviceListTxn(tx, ws, entMeta, peerName)
}

func serviceListTxn(tx ReadTxn, ws memdb.WatchSet, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.ServiceList, error) {
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	idx := catalogServicesMaxIndex(tx, entMeta, peerName)

	services, err := tx.Get(tableServices, indexID+"_prefix", Query{
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying services: %s", err)
	}
	ws.Add(services.WatchCh())

	unique := make(map[structs.ServiceName]struct{})
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		unique[svc.CompoundServiceName().ServiceName] = struct{}{}
	}

	results := make(structs.ServiceList, 0, len(unique))
	for sn := range unique {
		results = append(results, structs.ServiceName{Name: sn.Name, EnterpriseMeta: sn.EnterpriseMeta})
	}

	return idx, results, nil
}

// ServicesByNodeMeta returns all services, filtered by the given node metadata.
func (s *Store) ServicesByNodeMeta(ws memdb.WatchSet, filters map[string]string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, []*structs.ServiceNode, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Get the table index.
	idx := catalogServicesMaxIndex(tx, entMeta, peerName)

	if nodeIdx := catalogNodesMaxIndex(tx, entMeta, peerName); nodeIdx > idx {
		idx = nodeIdx
	}

	if len(filters) == 0 {
		return idx, nil, nil // ServicesByNodeMeta is never called with an empty map, but just in case make it return no results.
	}

	// Retrieve all of the nodes. We'll do a lookup of just ONE KV pair, which
	// over-matches if multiple pairs are requested, but then in the loop below
	// we'll finish filtering.
	var firstKey, firstValue string
	for firstKey, firstValue = range filters {
		break
	}

	nodes, err := tx.Get(tableNodes, indexMeta, KeyValueQuery{
		Key:            firstKey,
		Value:          firstValue,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	ws.Add(nodes.WatchCh())

	// We don't want to track an unlimited number of services, so we pull a
	// top-level watch to use as a fallback.
	allServices, err := catalogServiceListNoWildcard(tx, entMeta, peerName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed services lookup: %s", err)
	}
	allServicesCh := allServices.WatchCh()

	var result structs.ServiceNodes
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		n := node.(*structs.Node)
		if len(filters) > 1 && !structs.SatisfiesMetaFilters(n.Meta, filters) {
			continue
		}

		// List all the services on the node
		services, err := catalogServiceListByNode(tx, n.Node, entMeta, n.PeerName, false)
		if err != nil {
			return 0, nil, fmt.Errorf("failed querying services: %s", err)
		}
		ws.AddWithLimit(watchLimit, services.WatchCh(), allServicesCh)

		for service := services.Next(); service != nil; service = services.Next() {
			result = append(result, service.(*structs.ServiceNode))
		}
	}
	return idx, result, nil
}

// maxIndexForService return the maximum Raft Index for a service
// If the index is not set for the service, it will return the missing
// service index.
// The service_last_extinction is set to the last raft index when a service
// was unregistered (or 0 if no services were ever unregistered). This
// allows blocking queries to
//   - return when the last instance of a service is removed
//   - block until an instance for this service is available, or another
//     service is unregistered.
func maxIndexForService(tx ReadTxn, serviceName string, serviceExists, checks bool, entMeta *acl.EnterpriseMeta, peerName string) uint64 {
	idx, _ := maxIndexAndWatchChForService(tx, serviceName, serviceExists, checks, entMeta, peerName)
	return idx
}

// maxIndexAndWatchChForService return the maximum Raft Index for a service. If
// the index is not set for the service, it will return the missing service
// index. The service_last_extinction is set to the last raft index when a
// service was unregistered (or 0 if no services were ever unregistered). This
// allows blocking queries to
//   - return when the last instance of a service is removed
//   - block until an instance for this service is available, or another
//     service is unregistered.
//
// It also _may_ return a watch chan to add to a WatchSet. It will only return
// one if the service exists, and has a service index. If it doesn't then nil is
// returned for the chan. This allows for blocking watchers to _only_ watch this
// one chan in the common case, falling back to watching all touched MemDB
// indexes in more complicated cases.
func maxIndexAndWatchChForService(tx ReadTxn, serviceName string, serviceExists, checks bool, entMeta *acl.EnterpriseMeta, peerName string) (uint64, <-chan struct{}) {
	if !serviceExists {
		res, err := catalogServiceLastExtinctionIndex(tx, entMeta, peerName)
		if missingIdx, ok := res.(*IndexEntry); ok && err == nil {
			// Note safe to only watch the extinction index as it's not updated when new instances come along so return nil watchCh
			return missingIdx.Value, nil
		}
	}

	ch, res, err := catalogServiceMaxIndex(tx, serviceName, entMeta, peerName)
	if idx, ok := res.(*IndexEntry); ok && err == nil {
		return idx.Value, ch
	}
	return catalogMaxIndex(tx, entMeta, peerName, checks), nil
}

// Wrapper for maxIndexAndWatchChForService that operates on a list of ServiceNodes
func maxIndexAndWatchChsForServiceNodes(tx ReadTxn,
	nodes structs.ServiceNodes, watchChecks bool) (uint64, []<-chan struct{}) {

	var watchChans []<-chan struct{}
	var maxIdx uint64

	seen := make(map[structs.ServiceName]bool)
	for i := 0; i < len(nodes); i++ {
		sn := structs.NewServiceName(nodes[i].ServiceName, &nodes[i].EnterpriseMeta)
		if ok := seen[sn]; !ok {
			idx, svcCh := maxIndexAndWatchChForService(tx, sn.Name, true, watchChecks, &sn.EnterpriseMeta, nodes[i].PeerName)
			if idx > maxIdx {
				maxIdx = idx
			}
			if svcCh != nil {
				watchChans = append(watchChans, svcCh)
			}
			seen[sn] = true
		}
	}

	return maxIdx, watchChans
}

// ConnectServiceNodes returns the nodes associated with a Connect
// compatible destination for the given service name. This will include
// both proxies and native integrations.
func (s *Store) ConnectServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.ServiceNodes, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	q := Query{
		Value:          serviceName,
		PeerName:       peerName,
		EnterpriseMeta: *entMeta,
	}
	return serviceNodesTxn(tx, ws, indexConnect, q)
}

// ServiceNodes returns the nodes associated with a given service name.
func (s *Store) ServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.ServiceNodes, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	q := Query{
		Value:          serviceName,
		PeerName:       peerName,
		EnterpriseMeta: *entMeta,
	}
	return serviceNodesTxn(tx, ws, indexService, q)
}

func serviceNodesTxn(tx ReadTxn, ws memdb.WatchSet, index string, q Query) (uint64, structs.ServiceNodes, error) {
	connect := index == indexConnect
	serviceName := q.Value
	services, err := tx.Get(tableServices, index, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}
	ws.Add(services.WatchCh())

	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		results = append(results, service.(*structs.ServiceNode))
	}

	// If we are querying for Connect nodes, the associated proxy might be a gateway.
	// Gateways are tracked in a separate table, and we append them to the result set.
	// We append rather than replace since it allows users to migrate a service
	// to the mesh with a mix of sidecars and gateways until all its instances have a sidecar.
	var idx uint64
	if connect && q.PeerName == "" {
		// Look up gateway nodes associated with the service
		gwIdx, nodes, err := serviceGatewayNodes(tx, ws, serviceName, structs.ServiceKindTerminatingGateway, &q.EnterpriseMeta, structs.DefaultPeerKeyword)
		if err != nil {
			return 0, nil, fmt.Errorf("failed gateway nodes lookup: %v", err)
		}
		if idx < gwIdx {
			idx = gwIdx
		}

		// Watch for index changes to the gateway nodes
		svcIdx, chans := maxIndexAndWatchChsForServiceNodes(tx, nodes, false)
		if svcIdx > idx {
			idx = svcIdx
		}
		for _, ch := range chans {
			ws.Add(ch)
		}

		for i := 0; i < len(nodes); i++ {
			results = append(results, nodes[i])
		}
	}

	// Fill in the node details.
	results, err = parseServiceNodes(tx, ws, results, &q.EnterpriseMeta, q.PeerName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed parsing service nodes: %s", err)
	}

	// Get the table index.
	// TODO (gateways) (freddy) Why do we always consider the main service index here?
	//      This doesn't seem to make sense for Connect when there's more than 1 result
	svcIdx := maxIndexForService(tx, serviceName, len(results) > 0, false, &q.EnterpriseMeta, q.PeerName)
	if idx < svcIdx {
		idx = svcIdx
	}

	return idx, results, nil
}

// ServiceTagNodes returns the nodes associated with a given service, filtering
// out services that don't contain the given tags.
func (s *Store) ServiceTagNodes(ws memdb.WatchSet, service string, tags []string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.ServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	services, err := tx.Get(tableServices, indexService, Query{
		Value:          service,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}
	ws.Add(services.WatchCh())

	// Gather all the services and apply the tag filter.
	serviceExists := false
	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		serviceExists = true
		if !serviceTagsFilter(svc, tags) {
			results = append(results, svc)
		}
	}

	// Fill in the node details.
	results, err = parseServiceNodes(tx, ws, results, entMeta, peerName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed parsing service nodes: %s", err)
	}
	// Get the table index.
	idx := maxIndexForService(tx, service, serviceExists, false, entMeta, peerName)

	return idx, results, nil
}

// serviceTagFilter returns true (should filter) if the given service node
// doesn't contain the given tag.
func serviceTagFilter(sn *structs.ServiceNode, tag string) bool {
	tag = strings.ToLower(tag)

	// Look for the lower cased version of the tag.
	for _, t := range sn.ServiceTags {
		if strings.ToLower(t) == tag {
			return false
		}
	}

	// If we didn't hit the tag above then we should filter.
	return true
}

// serviceTagsFilter returns true (should filter) if the given service node
// doesn't contain the given set of tags.
func serviceTagsFilter(sn *structs.ServiceNode, tags []string) bool {
	for _, tag := range tags {
		if serviceTagFilter(sn, tag) {
			// If any one of the expected tags was not found, filter the service
			return true
		}
	}

	// If all tags were found, don't filter the service
	return false
}

// ServiceAddressNodes returns the nodes associated with a given service, filtering
// out services that don't match the given serviceAddress
func (s *Store) ServiceAddressNodes(ws memdb.WatchSet, address string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.ServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// List all the services.
	q := Query{
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	services, err := tx.Get(tableServices, indexID+"_prefix", q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}
	ws.Add(services.WatchCh())

	// Gather all the services and apply the tag filter.
	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		if svc.ServiceAddress == address {
			results = append(results, svc)
		} else {
			for _, addr := range svc.ServiceTaggedAddresses {
				if addr.Address == address {
					results = append(results, svc)
					break
				}
			}
		}
	}

	// Fill in the node details.
	results, err = parseServiceNodes(tx, ws, results, entMeta, peerName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed parsing service nodes: %s", err)
	}
	return 0, results, nil
}

// parseServiceNodes iterates over a services query and fills in the node details,
// returning a ServiceNodes slice.
func parseServiceNodes(tx ReadTxn, ws memdb.WatchSet, services structs.ServiceNodes, entMeta *acl.EnterpriseMeta, peerName string) (structs.ServiceNodes, error) {
	// We don't want to track an unlimited number of nodes, so we pull a
	// top-level watch to use as a fallback.
	q := Query{
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	allNodes, err := tx.Get(tableNodes, indexID+"_prefix", q)
	if err != nil {
		return nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	allNodesCh := allNodes.WatchCh()

	// Fill in the node data for each service instance.
	var results structs.ServiceNodes
	for _, sn := range services {
		// Note that we have to clone here because we don't want to
		// modify the node-related fields on the object in the database,
		// which is what we are referencing.
		s := sn.PartialClone()

		// Grab the corresponding node record.
		watchCh, n, err := tx.FirstWatch(tableNodes, indexID, Query{
			Value:          sn.Node,
			EnterpriseMeta: sn.EnterpriseMeta,
			PeerName:       sn.PeerName,
		})
		if err != nil {
			return nil, fmt.Errorf("failed node lookup: %s", err)
		}
		ws.AddWithLimit(watchLimit, watchCh, allNodesCh)

		// Populate the node-related fields. The tagged addresses may be
		// used by agents to perform address translation if they are
		// configured to do that.
		node := n.(*structs.Node)
		s.ID = node.ID
		s.Address = node.Address
		s.Datacenter = node.Datacenter
		s.TaggedAddresses = node.TaggedAddresses
		s.EnterpriseMeta.Merge(node.GetEnterpriseMeta())
		s.NodeMeta = node.Meta

		results = append(results, s)
	}
	return results, nil
}

// NodeService is used to retrieve a specific service associated with the given
// node.
func (s *Store) NodeService(ws memdb.WatchSet, nodeName string, serviceID string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.NodeService, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := catalogServicesMaxIndex(tx, entMeta, peerName)

	// Query the service
	service, err := getNodeServiceTxn(tx, ws, nodeName, serviceID, entMeta, peerName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying service for node %q: %s", nodeName, err)
	}

	return idx, service, nil
}

func getNodeServiceTxn(tx ReadTxn, ws memdb.WatchSet, nodeName, serviceID string, entMeta *acl.EnterpriseMeta, peerName string) (*structs.NodeService, error) {
	sn, err := getServiceNodeTxn(tx, ws, nodeName, serviceID, entMeta, peerName)
	if err != nil {
		return nil, err
	}
	if sn != nil {
		return sn.ToNodeService(), nil
	}
	return nil, nil
}

func getServiceNodeTxn(tx ReadTxn, ws memdb.WatchSet, nodeName, serviceID string, entMeta *acl.EnterpriseMeta, peerName string) (*structs.ServiceNode, error) {
	// TODO: pass non-pointer type for ent meta
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// Query the service
	watch, service, err := tx.FirstWatch(tableServices, indexID, NodeServiceQuery{
		EnterpriseMeta: *entMeta,
		Node:           nodeName,
		Service:        serviceID,
		PeerName:       peerName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed querying service for node %q: %s", nodeName, err)
	}
	ws.Add(watch)

	if service != nil {
		return service.(*structs.ServiceNode), nil
	}

	return nil, nil
}

// ServiceNode is used to retrieve a specific service by service ID and node ID or name.
func (s *Store) ServiceNode(nodeID, nodeName, serviceID string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.ServiceNode, error) {
	var (
		node *structs.Node
		err  error
	)
	if nodeID != "" {
		_, node, err = s.GetNodeID(types.NodeID(nodeID), entMeta, peerName)
		if err != nil {
			return 0, nil, fmt.Errorf("Failure looking up node by ID %s: %w", nodeID, err)
		}
	} else if nodeName != "" {
		_, node, err = s.GetNode(nodeName, entMeta, peerName)
		if err != nil {
			return 0, nil, fmt.Errorf("Failure looking up node by name %s: %w", nodeName, err)
		}
	} else {
		return 0, nil, fmt.Errorf("Node ID or name required to lookup the service")
	}
	if node == nil {
		return 0, nil, ErrNodeNotFound
	}

	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := catalogServicesMaxIndex(tx, entMeta, peerName)

	// Query the service
	service, err := getServiceNodeTxn(tx, nil, node.Node, serviceID, entMeta, peerName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying service for node %q: %w", node.Node, err)
	}
	if service != nil {
		service.ID = node.ID
	}

	return idx, service, nil
}

func (s *Store) nodeServices(ws memdb.WatchSet, nodeNameOrID string, entMeta *acl.EnterpriseMeta, peerName string, allowWildcard bool) (bool, uint64, *structs.Node, memdb.ResultIterator, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value for entMeta
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// Query the node by node name
	watchCh, n, err := tx.FirstWatch(tableNodes, indexID, Query{
		Value:          nodeNameOrID,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return true, 0, nil, nil, fmt.Errorf("node lookup failed: %s", err)
	}

	if n != nil {
		ws.Add(watchCh)
	} else {
		if len(nodeNameOrID) < minUUIDLookupLen {
			ws.Add(watchCh)
			return true, 0, nil, nil, nil
		}

		// Attempt to lookup the node by its node ID
		iter, err := tx.Get(tableNodes, indexUUID+"_prefix", Query{
			Value:          resizeNodeLookupKey(nodeNameOrID),
			EnterpriseMeta: *entMeta,
			PeerName:       peerName,
		})
		if err != nil {
			ws.Add(watchCh)
			idx := catalogNodeLastExtinctionIndex(tx, entMeta, peerName)
			return true, idx, nil, nil, nil
		}

		n = iter.Next()
		if n == nil {
			// No nodes matched, even with the Node ID: add a watch on the node name.
			ws.Add(watchCh)
			idx := catalogNodeLastExtinctionIndex(tx, entMeta, peerName)
			return true, idx, nil, nil, nil
		}

		idWatchCh := iter.WatchCh()
		if iter.Next() != nil {
			// More than one match present: Watch on the node name channel and return
			// an empty result (node lookups can not be ambiguous).
			ws.Add(watchCh)
			return true, 0, nil, nil, nil
		}

		ws.Add(idWatchCh)
	}

	node := n.(*structs.Node)
	nodeName := node.Node

	// Read all of the services
	services, err := catalogServiceListByNode(tx, nodeName, entMeta, peerName, allowWildcard)
	if err != nil {
		return true, 0, nil, nil, fmt.Errorf("failed querying services for node %q: %s", nodeName, err)
	}
	ws.Add(services.WatchCh())

	// Get the table index.
	idx := catalogNodeMaxIndex(tx, nodeName, entMeta, peerName)

	return false, idx, node, services, nil
}

// NodeServices is used to query service registrations by node name or UUID.
func (s *Store) NodeServices(ws memdb.WatchSet, nodeNameOrID string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.NodeServices, error) {
	done, idx, node, services, err := s.nodeServices(ws, nodeNameOrID, entMeta, peerName, false)
	if done || err != nil {
		return idx, nil, err
	}

	// Initialize the node services struct
	ns := &structs.NodeServices{
		Node:     node,
		Services: make(map[string]*structs.NodeService),
	}

	if services != nil {
		// Add all of the services to the map.
		for service := services.Next(); service != nil; service = services.Next() {
			svc := service.(*structs.ServiceNode).ToNodeService()
			ns.Services[svc.ID] = svc
		}
	}

	return idx, ns, nil
}

// NodeServices is used to query service registrations by node name or UUID.
func (s *Store) NodeServiceList(ws memdb.WatchSet, nodeNameOrID string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.NodeServiceList, error) {
	done, idx, node, services, err := s.nodeServices(ws, nodeNameOrID, entMeta, peerName, true)
	if done || err != nil {
		return idx, nil, err
	}

	if idx == 0 {
		return 0, nil, nil
	}

	// Initialize the node services struct
	ns := &structs.NodeServiceList{
		Node: node,
	}

	if services != nil {
		// Add all of the services to the map.
		for service := services.Next(); service != nil; service = services.Next() {
			svc := service.(*structs.ServiceNode).ToNodeService()
			ns.Services = append(ns.Services, svc)
		}
	}

	return idx, ns, nil
}

// DeleteService is used to delete a given service associated with a node.
func (s *Store) DeleteService(idx uint64, nodeName, serviceID string, entMeta *acl.EnterpriseMeta, peerName string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Call the service deletion
	if err := s.deleteServiceTxn(tx, idx, nodeName, serviceID, entMeta, peerName); err != nil {
		return err
	}

	return tx.Commit()
}

// deleteServiceCASTxn is used to try doing a service delete operation with a given
// raft index. If the CAS index specified is not equal to the last observed index for
// the given service, then the call is a noop, otherwise a normal delete is invoked.
func (s *Store) deleteServiceCASTxn(tx WriteTxn, idx, cidx uint64, nodeName, serviceID string, entMeta *acl.EnterpriseMeta, peerName string) (bool, error) {
	// Look up the service.
	service, err := getNodeServiceTxn(tx, nil, nodeName, serviceID, entMeta, peerName)
	if err != nil {
		return false, fmt.Errorf("service lookup failed: %s", err)
	}
	if service == nil {
		return false, nil
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	if service.ModifyIndex != cidx {
		return false, nil
	}

	// Call the actual deletion if the above passed.
	if err := s.deleteServiceTxn(tx, idx, nodeName, serviceID, entMeta, peerName); err != nil {
		return false, err
	}

	return true, nil
}

// deleteServiceTxn is the inner method called to remove a service
// registration within an existing transaction.
func (s *Store) deleteServiceTxn(tx WriteTxn, idx uint64, nodeName, serviceID string, entMeta *acl.EnterpriseMeta, peerName string) error {
	// TODO: pass non-pointer type for ent meta
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	service, err := tx.First(tableServices, indexID,
		NodeServiceQuery{
			EnterpriseMeta: *entMeta,
			Node:           nodeName,
			Service:        serviceID,
			PeerName:       peerName,
		})
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}
	if service == nil {
		return nil
	}

	// Delete any checks associated with the service. This will invalidate
	// sessions as necessary.
	nsq := NodeServiceQuery{
		Node:           nodeName,
		Service:        serviceID,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	checks, err := tx.Get(tableChecks, indexNodeService, nsq)
	if err != nil {
		return fmt.Errorf("failed service check lookup: %s", err)
	}
	var deleteChecks []*structs.HealthCheck
	for check := checks.Next(); check != nil; check = checks.Next() {
		deleteChecks = append(deleteChecks, check.(*structs.HealthCheck))
	}

	// Do the delete in a separate loop so we don't trash the iterator.
	for _, check := range deleteChecks {
		if err := s.deleteCheckTxn(tx, idx, nodeName, check.CheckID, &check.EnterpriseMeta, check.PeerName); err != nil {
			return err
		}
	}

	// Update the index.
	if err := catalogUpdateCheckIndexes(tx, idx, entMeta, peerName); err != nil {
		return err
	}

	// Delete the service and update the index
	if err := tx.Delete(tableServices, service); err != nil {
		return fmt.Errorf("failed deleting service: %s", err)
	}

	svc := service.(*structs.ServiceNode)
	if err := catalogUpdateServicesIndexes(tx, idx, entMeta, svc.PeerName); err != nil {
		return fmt.Errorf("failed updating services indexes: %w", err)
	}
	if err := catalogUpdateServiceKindIndexes(tx, idx, svc.ServiceKind, &svc.EnterpriseMeta, svc.PeerName); err != nil {
		return fmt.Errorf("failed updating service-kind indexes: %w", err)
	}
	// Update the node indexes as the service information is included in node catalog queries.
	if err := catalogUpdateNodesIndexes(tx, idx, entMeta, svc.PeerName); err != nil {
		return fmt.Errorf("failed updating nodes indexes: %w", err)
	}
	if err := catalogUpdateNodeIndexes(tx, idx, nodeName, entMeta, svc.PeerName); err != nil {
		return fmt.Errorf("failed updating node indexes: %w", err)
	}

	psn := svc.CompoundServiceName()

	if err := cleanupMeshTopology(tx, idx, svc); err != nil {
		return fmt.Errorf("failed to clean up mesh-topology associations for %q: %v", psn.String(), err)
	}

	q := Query{
		Value:          svc.ServiceName,
		EnterpriseMeta: *entMeta,
		PeerName:       svc.PeerName,
	}
	if remainingService, err := tx.First(tableServices, indexService, q); err == nil {
		if remainingService != nil {
			// We have at least one remaining service, update the index
			if err := catalogUpdateServiceIndexes(tx, idx, svc.ServiceName, entMeta, svc.PeerName); err != nil {
				return err
			}
		} else {
			// There are no more service instances, cleanup the service.<serviceName> index
			_, serviceIndex, err := catalogServiceMaxIndex(tx, svc.ServiceName, entMeta, svc.PeerName)
			if err == nil && serviceIndex != nil {
				// we found service.<serviceName> index, garbage collect it
				if err := tx.Delete(tableIndex, serviceIndex); err != nil {
					return fmt.Errorf("[FAILED] deleting serviceIndex %s: %s", svc.ServiceName, err)
				}
			}

			if err := catalogUpdateServiceExtinctionIndex(tx, idx, entMeta, svc.PeerName); err != nil {
				return err
			}
			if err := freeServiceVirtualIP(tx, idx, psn, nil); err != nil {
				return fmt.Errorf("failed to clean up virtual IP for %q: %v", psn.String(), err)
			}

			if svc.PeerName == "" {
				if err := cleanupKindServiceName(tx, idx, psn.ServiceName, svc.ServiceKind); err != nil {
					return fmt.Errorf("failed to persist service name: %v", err)
				}
			}
		}
	} else {
		return fmt.Errorf("Could not find any service %s: %s", svc.ServiceName, err)
	}

	// Cleanup ConnectEnabled for this service if none exist.
	if svc.PeerName == "" && (svc.ServiceKind == structs.ServiceKindConnectProxy || svc.ServiceConnect.Native) {
		service := svc.ServiceName
		if svc.ServiceKind == structs.ServiceKindConnectProxy {
			service = svc.ServiceProxy.DestinationServiceName
		}
		sn := structs.ServiceName{Name: service, EnterpriseMeta: svc.EnterpriseMeta}
		connectEnabled, err := serviceHasConnectEnabledInstances(tx, sn.Name, &sn.EnterpriseMeta)
		if err != nil {
			return fmt.Errorf("failed to search for connect instances for service %q: %w", sn.Name, err)
		}
		if !connectEnabled {
			if err := cleanupKindServiceName(tx, idx, sn, structs.ServiceKindConnectEnabled); err != nil {
				return fmt.Errorf("failed to cleanup connect-enabled service name: %v", err)
			}
		}
	}

	if svc.PeerName == "" {
		sn := structs.ServiceName{Name: svc.ServiceName, EnterpriseMeta: svc.EnterpriseMeta}
		if err := cleanupGatewayWildcards(tx, idx, sn, false); err != nil {
			return fmt.Errorf("failed to clean up gateway-service associations for %q: %v", psn.String(), err)
		}
	}

	return nil
}

// freeServiceVirtualIP is used to free a virtual IP for a service after the last instance
// is removed.
func freeServiceVirtualIP(
	tx WriteTxn,
	idx uint64,
	psn structs.PeeredServiceName,
	excludeGateway *structs.ServiceName,
) error {
	supported, err := virtualIPsSupported(tx, nil)
	if err != nil {
		return err
	}
	if !supported {
		return nil
	}

	// Don't deregister the virtual IP if at least one instance of this service still exists.
	q := Query{
		Value:          psn.ServiceName.Name,
		EnterpriseMeta: psn.ServiceName.EnterpriseMeta,
		PeerName:       psn.Peer,
	}
	if remainingService, err := tx.First(tableServices, indexService, q); err == nil {
		if remainingService != nil {
			return nil
		}
	} else {
		return fmt.Errorf("failed service lookup for %q: %s", psn.ServiceName.Name, err)
	}

	// Don't deregister the virtual IP if at least one resolver/router/splitter config entry still
	// references this service.
	configEntryVIPKinds := []string{
		structs.ServiceResolver,
		structs.ServiceRouter,
		structs.ServiceSplitter,
		structs.ServiceDefaults,
		structs.ServiceIntentions,
	}
	for _, kind := range configEntryVIPKinds {
		_, entry, err := configEntryTxn(tx, nil, kind, psn.ServiceName.Name, &psn.ServiceName.EnterpriseMeta)
		if err != nil {
			return fmt.Errorf("failed config entry lookup for %s/%s: %s", kind, psn.ServiceName.Name, err)
		}
		if entry != nil {
			return nil
		}
	}

	// Don't deregister the virtual IP if at least one terminating gateway still references this service.
	termGatewaySupported, err := terminatingGatewayVirtualIPsSupported(tx, nil)
	if err != nil {
		return err
	}
	if termGatewaySupported {
		svcGateways, err := tx.Get(tableGatewayServices, indexService, psn.ServiceName)
		if err != nil {
			return fmt.Errorf("failed gateway lookup for %q: %s", psn.ServiceName.Name, err)
		}

		for service := svcGateways.Next(); service != nil; service = svcGateways.Next() {
			if svc, ok := service.(*structs.GatewayService); ok && svc != nil {
				ignoreGateway := excludeGateway == nil || !svc.Gateway.Matches(*excludeGateway)
				if ignoreGateway && svc.GatewayKind == structs.ServiceKindTerminatingGateway {
					return nil
				}
			}
		}
	}

	serviceVIP, err := tx.First(tableServiceVirtualIPs, indexID, psn)
	if err != nil {
		return fmt.Errorf("failed service virtual IP lookup: %s", err)
	}
	// Service has no virtual IP assigned, nothing to do.
	if serviceVIP == nil {
		return nil
	}

	// Delete the service virtual IP and add it to the freed IPs list.
	if err := tx.Delete(tableServiceVirtualIPs, serviceVIP); err != nil {
		return fmt.Errorf("failed updating freed virtual IP table: %v", err)
	}

	newEntry := FreeVirtualIP{IP: serviceVIP.(ServiceVirtualIP).IP}
	if err := tx.Insert(tableFreeVirtualIPs, newEntry); err != nil {
		return fmt.Errorf("failed updating freed virtual IP table: %v", err)
	}

	if err := updateVirtualIPMaxIndexes(tx, idx, psn.ServiceName.PartitionOrDefault(), psn.Peer); err != nil {
		return err
	}

	return nil
}

// EnsureCheck is used to store a check registration in the db.
func (s *Store) EnsureCheck(idx uint64, hc *structs.HealthCheck) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Call the check registration
	if err := s.ensureCheckTxn(tx, idx, false, hc); err != nil {
		return err
	}

	return tx.Commit()
}

// updateAllServiceIndexesOfNode updates the Raft index of all the services associated with this node
func updateAllServiceIndexesOfNode(tx WriteTxn, idx uint64, nodeID string, entMeta *acl.EnterpriseMeta, peerName string) error {
	if peerName == "" {
		peerName = structs.LocalPeerKeyword
	}
	services, err := tx.Get(tableServices, indexNode, Query{
		Value:          nodeID,
		EnterpriseMeta: *entMeta.WithWildcardNamespace(),
		PeerName:       peerName,
	})
	if err != nil {
		return fmt.Errorf("failed updating services for node %s: %s", nodeID, err)
	}
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		if err := catalogUpdateServiceIndexes(tx, idx, svc.ServiceName, &svc.EnterpriseMeta, svc.PeerName); err != nil {
			return err
		}
		if err := catalogUpdateServiceKindIndexes(tx, idx, svc.ServiceKind, &svc.EnterpriseMeta, svc.PeerName); err != nil {
			return err
		}
	}
	return nil
}

// ensureCheckCASTxn updates a check only if the existing index matches the given index.
// Returns a bool indicating if a write happened and any error.
func (s *Store) ensureCheckCASTxn(tx WriteTxn, idx uint64, hc *structs.HealthCheck) (bool, error) {
	// Retrieve the existing entry.
	_, existing, err := getNodeCheckTxn(tx, hc.Node, hc.CheckID, &hc.EnterpriseMeta, hc.PeerName)
	if err != nil {
		return false, fmt.Errorf("failed health check lookup: %s", err)
	}

	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	if hc.ModifyIndex == 0 && existing != nil {
		return false, nil
	}
	if hc.ModifyIndex != 0 && existing == nil {
		return false, nil
	}
	if existing != nil && hc.ModifyIndex != 0 && hc.ModifyIndex != existing.ModifyIndex {
		return false, nil
	}

	// Perform the update.
	if err := s.ensureCheckTxn(tx, idx, false, hc); err != nil {
		return false, err
	}

	return true, nil
}

// ensureCheckTxn is used as the inner method to handle inserting
// a health check into the state store. It ensures safety against inserting
// checks with no matching node or service.
func (s *Store) ensureCheckTxn(tx WriteTxn, idx uint64, preserveIndexes bool, hc *structs.HealthCheck) error {
	// Check if we have an existing health check
	existing, err := tx.First(tableChecks, indexID, NodeCheckQuery{
		EnterpriseMeta: hc.EnterpriseMeta,
		Node:           hc.Node,
		CheckID:        string(hc.CheckID),
		PeerName:       hc.PeerName,
	})
	if err != nil {
		return fmt.Errorf("failed health check lookup: %s", err)
	}

	// Set the indexes
	if existing != nil {
		existingCheck := existing.(*structs.HealthCheck)
		hc.CreateIndex = existingCheck.CreateIndex
		hc.ModifyIndex = existingCheck.ModifyIndex
	} else if !preserveIndexes {
		hc.CreateIndex = idx
	}

	// Use the default check status if none was provided
	if hc.Status == "" {
		hc.Status = api.HealthCritical
	}

	// Get the node
	node, err := tx.First(tableNodes, indexID, Query{
		Value:          hc.Node,
		EnterpriseMeta: hc.EnterpriseMeta,
		PeerName:       hc.PeerName,
	})
	if err != nil {
		return fmt.Errorf("failed node lookup: %s", err)
	}
	if node == nil {
		return ErrMissingNode
	}

	modified := true
	// If the check is associated with a service, check that we have
	// a registration for the service.
	if hc.ServiceID != "" {
		service, err := tx.First(tableServices, indexID, NodeServiceQuery{
			EnterpriseMeta: hc.EnterpriseMeta,
			Node:           hc.Node,
			Service:        hc.ServiceID,
			PeerName:       hc.PeerName,
		})
		if err != nil {
			return fmt.Errorf("failed service lookup: %s", err)
		}
		if service == nil {
			return ErrMissingService
		}

		// Copy in the service name and tags
		svc := service.(*structs.ServiceNode)
		hc.ServiceName = svc.ServiceName
		hc.ServiceTags = svc.ServiceTags
		if existing != nil && existing.(*structs.HealthCheck).IsSame(hc) {
			modified = false
		} else {
			if err = catalogUpdateServiceIndexes(tx, idx, svc.ServiceName, &svc.EnterpriseMeta, svc.PeerName); err != nil {
				return err
			}
			if err := catalogUpdateServiceKindIndexes(tx, idx, svc.ServiceKind, &svc.EnterpriseMeta, svc.PeerName); err != nil {
				return err
			}
		}
	} else {
		if existing != nil && existing.(*structs.HealthCheck).IsSame(hc) {
			modified = false
		} else {
			// Since the check has been modified, it impacts all services of node
			// Update the status for all the services associated with this node
			err = updateAllServiceIndexesOfNode(tx, idx, hc.Node, &hc.EnterpriseMeta, hc.PeerName)
			if err != nil {
				return err
			}
		}
	}

	// Delete any sessions for this check if the health is critical.
	if hc.Status == api.HealthCritical && hc.PeerName == "" {
		sessions, err := checkSessionsTxn(tx, hc)
		if err != nil {
			return err
		}

		// Delete the session in a separate loop so we don't trash the
		// iterator.
		for _, sess := range sessions {
			if err := s.deleteSessionTxn(tx, idx, sess.Session, &sess.EnterpriseMeta); err != nil {
				return fmt.Errorf("failed deleting session: %s", err)
			}
		}
	}
	if !modified {
		return nil
	}
	if !preserveIndexes {
		hc.ModifyIndex = idx
	}

	return catalogInsertCheck(tx, hc, idx)
}

// NodeCheck is used to retrieve a specific check associated with the given
// node.
func (s *Store) NodeCheck(nodeName string, checkID types.CheckID, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.HealthCheck, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return getNodeCheckTxn(tx, nodeName, checkID, entMeta, peerName)
}

// nodeCheckTxn is used as the inner method to handle reading a health check
// from the state store.
func getNodeCheckTxn(tx ReadTxn, nodeName string, checkID types.CheckID, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.HealthCheck, error) {
	// Get the table index.
	idx := catalogChecksMaxIndex(tx, entMeta, peerName)

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// Return the check.
	check, err := tx.First(tableChecks, indexID,
		NodeCheckQuery{
			EnterpriseMeta: *entMeta,
			Node:           nodeName,
			CheckID:        string(checkID),
			PeerName:       peerName,
		})
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}

	if check != nil {
		return idx, check.(*structs.HealthCheck), nil
	}
	return idx, nil, nil
}

// NodeChecks is used to retrieve checks associated with the
// given node from the state store.
func (s *Store) NodeChecks(ws memdb.WatchSet, nodeName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// Get the table index.
	idx := catalogChecksMaxIndex(tx, entMeta, peerName)

	// Return the checks.
	iter, err := catalogListChecksByNode(tx, Query{
		Value:          nodeName,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results structs.HealthChecks
	for check := iter.Next(); check != nil; check = iter.Next() {
		results = append(results, check.(*structs.HealthCheck))
	}
	return idx, results, nil
}

// ServiceChecks is used to get all checks associated with a
// given service ID. The query is performed against a service
// _name_ instead of a service ID.
func (s *Store) ServiceChecks(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := catalogChecksMaxIndex(tx, entMeta, peerName)

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	q := Query{
		Value:          serviceName,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	iter, err := tx.Get(tableChecks, indexService, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results structs.HealthChecks
	for check := iter.Next(); check != nil; check = iter.Next() {
		results = append(results, check.(*structs.HealthCheck))
	}
	return idx, results, nil
}

// ServiceChecksByNodeMeta is used to get all checks associated with a
// given service ID, filtered by the given node metadata values. The query
// is performed against a service _name_ instead of a service ID.
func (s *Store) ServiceChecksByNodeMeta(ws memdb.WatchSet, serviceName string, filters map[string]string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexForService(tx, serviceName, true, true, entMeta, peerName)

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	q := Query{
		Value:          serviceName,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	iter, err := tx.Get(tableChecks, indexService, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	return parseChecksByNodeMeta(tx, ws, idx, iter, filters, entMeta, peerName)
}

// ChecksInState is used to query the state store for all checks
// which are in the provided state.
func (s *Store) ChecksInState(ws memdb.WatchSet, state string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	idx, iter, err := checksInStateTxn(tx, ws, state, entMeta, peerName)
	if err != nil {
		return 0, nil, err
	}

	var results structs.HealthChecks
	for check := iter.Next(); check != nil; check = iter.Next() {
		results = append(results, check.(*structs.HealthCheck))
	}
	return idx, results, nil
}

// ChecksInStateByNodeMeta is used to query the state store for all checks
// which are in the provided state, filtered by the given node metadata values.
func (s *Store) ChecksInStateByNodeMeta(ws memdb.WatchSet, state string, filters map[string]string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	idx, iter, err := checksInStateTxn(tx, ws, state, entMeta, peerName)
	if err != nil {
		return 0, nil, err
	}

	return parseChecksByNodeMeta(tx, ws, idx, iter, filters, entMeta, peerName)
}

func checksInStateTxn(tx ReadTxn, ws memdb.WatchSet, state string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, memdb.ResultIterator, error) {
	// Get the table index.
	idx := catalogChecksMaxIndex(tx, entMeta, peerName)

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// Query all checks if HealthAny is passed, otherwise use the index.
	var iter memdb.ResultIterator
	var err error
	if state == api.HealthAny {
		q := Query{
			EnterpriseMeta: *entMeta,
			PeerName:       peerName,
		}
		iter, err = tx.Get(tableChecks, indexID+"_prefix", q)
	} else {
		q := Query{
			Value:          state,
			EnterpriseMeta: *entMeta,
			PeerName:       peerName,
		}
		iter, err = tx.Get(tableChecks, indexStatus, q)
	}
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	return idx, iter, err
}

// parseChecksByNodeMeta is a helper function used to deduplicate some
// repetitive code for returning health checks filtered by node metadata fields.
func parseChecksByNodeMeta(
	tx ReadTxn,
	ws memdb.WatchSet,
	idx uint64,
	iter memdb.ResultIterator,
	filters map[string]string,
	entMeta *acl.EnterpriseMeta,
	peerName string,
) (uint64, structs.HealthChecks, error) {
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// We don't want to track an unlimited number of nodes, so we pull a
	// top-level watch to use as a fallback.
	q := Query{
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	allNodes, err := tx.Get(tableNodes, indexID+"_prefix", q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	allNodesCh := allNodes.WatchCh()

	// Only take results for nodes that satisfy the node metadata filters.
	var results structs.HealthChecks
	for check := iter.Next(); check != nil; check = iter.Next() {
		healthCheck := check.(*structs.HealthCheck)
		watchCh, node, err := tx.FirstWatch(tableNodes, indexID, Query{
			Value:          healthCheck.Node,
			EnterpriseMeta: healthCheck.EnterpriseMeta,
			PeerName:       healthCheck.PeerName,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("failed node lookup: %s", err)
		}
		if node == nil {
			return 0, nil, ErrMissingNode
		}

		// Add even the filtered nodes so we wake up if the node metadata
		// changes.
		ws.AddWithLimit(watchLimit, watchCh, allNodesCh)
		if structs.SatisfiesMetaFilters(node.(*structs.Node).Meta, filters) {
			results = append(results, healthCheck)
		}
	}
	return idx, results, nil
}

// DeleteCheck is used to delete a health check registration.
func (s *Store) DeleteCheck(idx uint64, node string, checkID types.CheckID, entMeta *acl.EnterpriseMeta, peerName string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Call the check deletion
	if err := s.deleteCheckTxn(tx, idx, node, checkID, entMeta, peerName); err != nil {
		return err
	}

	return tx.Commit()
}

// deleteCheckCASTxn is used to try doing a check delete operation with a given
// raft index. If the CAS index specified is not equal to the last observed index for
// the given check, then the call is a noop, otherwise a normal check delete is invoked.
func (s *Store) deleteCheckCASTxn(
	tx WriteTxn,
	idx, cidx uint64,
	node string,
	checkID types.CheckID,
	entMeta *acl.EnterpriseMeta,
	peerName string,
) (bool, error) {
	// Try to retrieve the existing health check.
	_, hc, err := getNodeCheckTxn(tx, node, checkID, entMeta, peerName)
	if err != nil {
		return false, fmt.Errorf("check lookup failed: %s", err)
	}
	if hc == nil {
		return false, nil
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	if hc.ModifyIndex != cidx {
		return false, nil
	}

	// Call the actual deletion if the above passed.
	if err := s.deleteCheckTxn(tx, idx, node, checkID, entMeta, peerName); err != nil {
		return false, err
	}

	return true, nil
}

// NodeServiceQuery is a type used to query the checks table.
type NodeServiceQuery struct {
	Node     string
	Service  string
	PeerName string
	acl.EnterpriseMeta
}

func (q NodeServiceQuery) PeerOrEmpty() string {
	return q.PeerName
}

// NamespaceOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q NodeServiceQuery) NamespaceOrDefault() string {
	return q.EnterpriseMeta.NamespaceOrDefault()
}

// PartitionOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q NodeServiceQuery) PartitionOrDefault() string {
	return q.EnterpriseMeta.PartitionOrDefault()
}

// deleteCheckTxn is the inner method used to call a health
// check deletion within an existing transaction.
func (s *Store) deleteCheckTxn(tx WriteTxn, idx uint64, node string, checkID types.CheckID, entMeta *acl.EnterpriseMeta, peerName string) error {
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// Try to retrieve the existing health check.
	hc, err := tx.First(tableChecks, indexID,
		NodeCheckQuery{
			EnterpriseMeta: *entMeta,
			Node:           node,
			CheckID:        string(checkID),
			PeerName:       peerName,
		})
	if err != nil {
		return fmt.Errorf("check lookup failed: %s", err)
	}
	if hc == nil {
		return nil
	}
	existing := hc.(*structs.HealthCheck)
	if existing != nil {
		// When no service is linked to this service, update all services of node
		if existing.ServiceID != "" {
			if err := catalogUpdateServiceIndexes(tx, idx, existing.ServiceName, &existing.EnterpriseMeta, existing.PeerName); err != nil {
				return err
			}
			svcRaw, err := tx.First(tableServices, indexID,
				NodeServiceQuery{
					EnterpriseMeta: existing.EnterpriseMeta,
					Node:           existing.Node,
					Service:        existing.ServiceID,
					PeerName:       existing.PeerName,
				})
			if err != nil {
				return fmt.Errorf("failed retrieving service from state store: %v", err)
			}

			svc := svcRaw.(*structs.ServiceNode)
			if err := catalogUpdateServiceKindIndexes(tx, idx, svc.ServiceKind, &svc.EnterpriseMeta, svc.PeerName); err != nil {
				return err
			}
		} else {
			if err := updateAllServiceIndexesOfNode(tx, idx, existing.Node, &existing.EnterpriseMeta, existing.PeerName); err != nil {
				return fmt.Errorf("Failed to update services linked to deleted healthcheck: %s", err)
			}
			if err := catalogUpdateServicesIndexes(tx, idx, entMeta, existing.PeerName); err != nil {
				return err
			}
		}
	}

	// Delete the check from the DB and update the index.
	if err := tx.Delete(tableChecks, hc); err != nil {
		return fmt.Errorf("failed removing check: %s", err)
	}

	if err := catalogUpdateCheckIndexes(tx, idx, entMeta, peerName); err != nil {
		return err
	}

	if peerName == "" {
		// Delete any sessions for this check.
		sessions, err := checkSessionsTxn(tx, existing)
		if err != nil {
			return err
		}

		// Do the delete in a separate loop so we don't trash the iterator.
		for _, sess := range sessions {
			if err := s.deleteSessionTxn(tx, idx, sess.Session, &sess.EnterpriseMeta); err != nil {
				return fmt.Errorf("failed deleting session: %s", err)
			}
		}
	}

	return nil
}

// CombinedCheckServiceNodes is used to query all nodes and checks for both typical and Connect endpoints of a service
func (s *Store) CombinedCheckServiceNodes(ws memdb.WatchSet, service structs.ServiceName, peerName string) (uint64, structs.CheckServiceNodes, error) {
	var (
		resp   structs.CheckServiceNodes
		maxIdx uint64
	)
	idx, csn, err := s.CheckServiceNodes(ws, service.Name, &service.EnterpriseMeta, peerName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get downstream nodes for %q: %v", service, err)
	}
	if idx > maxIdx {
		maxIdx = idx
	}
	resp = append(resp, csn...)
	idx, csn, err = s.CheckConnectServiceNodes(ws, service.Name, &service.EnterpriseMeta, peerName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get downstream connect nodes for %q: %v", service, err)
	}
	if idx > maxIdx {
		maxIdx = idx
	}
	resp = append(resp, csn...)

	return maxIdx, resp, nil
}

// CheckServiceNodes is used to query all nodes and checks for a given service.
func (s *Store) CheckServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error) {
	return s.checkServiceNodes(ws, serviceName, false, entMeta, peerName)
}

// CheckConnectServiceNodes is used to query all nodes and checks for Connect
// compatible endpoints for a given service.
func (s *Store) CheckConnectServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error) {
	return s.checkServiceNodes(ws, serviceName, true, entMeta, peerName)
}

// CheckIngressServiceNodes is used to query all nodes and checks for ingress
// endpoints for a given service.
func (s *Store) CheckIngressServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	maxIdx, nodes, err := serviceGatewayNodes(tx, ws, serviceName, structs.ServiceKindIngressGateway, entMeta, structs.DefaultPeerKeyword)
	if err != nil {
		return 0, nil, fmt.Errorf("failed gateway nodes lookup: %v", err)
	}

	// TODO(ingress) : Deal with incorporating index from mapping table
	// Watch for index changes to the gateway nodes
	idx, chans := maxIndexAndWatchChsForServiceNodes(tx, nodes, false)
	for _, ch := range chans {
		ws.Add(ch)
	}
	maxIdx = lib.MaxUint64(maxIdx, idx)

	// TODO(ingress): Test namespace functionality here
	// De-dup services to lookup
	names := make(map[structs.ServiceName]struct{})
	for _, n := range nodes {
		names[n.CompoundServiceName().ServiceName] = struct{}{}
	}

	var results structs.CheckServiceNodes
	for sn := range names {
		idx, n, err := checkServiceNodesTxn(tx, ws, sn.Name, false, &sn.EnterpriseMeta, structs.DefaultPeerKeyword)
		if err != nil {
			return 0, nil, err
		}
		maxIdx = lib.MaxUint64(maxIdx, idx)
		results = append(results, n...)
	}
	return maxIdx, results, nil
}

func (s *Store) checkServiceNodes(ws memdb.WatchSet, serviceName string, connect bool, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return checkServiceNodesTxn(tx, ws, serviceName, connect, entMeta, peerName)
}

func checkServiceNodesTxn(tx ReadTxn, ws memdb.WatchSet, serviceName string, connect bool, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error) {
	index := indexService
	if connect {
		index = indexConnect
	}

	// TODO: accept non-pointer
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	q := Query{
		Value:          serviceName,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	iter, err := tx.Get(tableServices, index, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}
	// Note we decide if we want to watch this iterator or not down below. We need
	// to see if it returned anything first.

	// Return the results.
	var results structs.ServiceNodes

	// For connect queries we need a list of any proxy service names in the result
	// set. Rather than have different code path for connect and non-connect, we
	// use the same one in both cases. For non-empty non-connect results,
	// serviceNames will always have exactly one element which is the same as
	// serviceName. For Connect there might be multiple different service names -
	// one for each service name a proxy is registered under, and the target
	// service name IFF there is at least one Connect-native instance of that
	// service. Either way there is usually only one distinct name if proxies are
	// named consistently but could be multiple.
	serviceNames := make(map[structs.ServiceName]struct{}, 2)
	for service := iter.Next(); service != nil; service = iter.Next() {
		sn := service.(*structs.ServiceNode)
		results = append(results, sn)

		name := structs.NewServiceName(sn.ServiceName, &sn.EnterpriseMeta)
		serviceNames[name] = struct{}{}
	}

	// If we are querying for Connect nodes, the associated proxy might be a terminating-gateway.
	// Gateways are tracked in a separate table, and we append them to the result set.
	// We append rather than replace since it allows users to migrate a service
	// to the mesh with a mix of sidecars and gateways until all its instances have a sidecar.
	var idx uint64
	if connect && peerName == "" {
		// Look up gateway nodes associated with the service
		// TODO(peering): we'll have to do something here
		gwIdx, nodes, err := serviceGatewayNodes(tx, ws, serviceName, structs.ServiceKindTerminatingGateway, entMeta, structs.DefaultPeerKeyword)
		if err != nil {
			return 0, nil, fmt.Errorf("failed gateway nodes lookup: %v", err)
		}
		idx = lib.MaxUint64(idx, gwIdx)
		for i := 0; i < len(nodes); i++ {
			results = append(results, nodes[i])

			name := structs.NewServiceName(nodes[i].ServiceName, &nodes[i].EnterpriseMeta)
			serviceNames[name] = struct{}{}
		}
	}

	// watchOptimized tracks if we meet the necessary condition to optimize
	// WatchSet size. That is that every service name represented in the result
	// set must have a service-specific index we can watch instead of many radix
	// nodes for all the actual nodes touched. This saves us watching potentially
	// thousands of watch chans for large services which may need many goroutines.
	// It also avoids the performance cliff that is hit when watchLimit is hit
	// (~682 service instances). See
	// https://github.com/hashicorp/consul/issues/4984
	watchOptimized := false
	if len(serviceNames) > 0 {
		// Assume optimization will work since it really should at this point. For
		// safety we'll sanity check this below for each service name.
		watchOptimized = true

		// Fetch indexes for all names services in result set.
		for n := range serviceNames {
			// We know service values should exist since the serviceNames map is only
			// populated if there is at least one result above. so serviceExists arg
			// below is always true.
			svcIdx, svcCh := maxIndexAndWatchChForService(tx, n.Name, true, true, &n.EnterpriseMeta, peerName)
			// Take the max index represented
			idx = lib.MaxUint64(idx, svcIdx)
			if svcCh != nil {
				// Watch the service-specific index for changes in liu of all iradix nodes
				// for checks etc.
				ws.Add(svcCh)
			} else {
				// Nil svcCh shouldn't really happen since all existent services should
				// have a service-specific index but just in case it does due to a bug,
				// fall back to the more expensive old way of watching every radix node
				// we touch.
				watchOptimized = false
			}
		}
	} else {
		// If we have no results, we should use the index of the last service
		// extinction event so we don't go backwards when services deregister. We
		// use target serviceName here but it actually doesn't matter. No chan will
		// be returned as we can't use the optimization in this case (and don't need
		// to as there is only one chan to watch anyway).
		svcIdx, _ := maxIndexAndWatchChForService(tx, serviceName, false, true, entMeta, peerName)
		idx = lib.MaxUint64(idx, svcIdx)
	}

	// Create a nil watchset to pass below, we'll only pass the real one if we
	// need to. Nil watchers are safe/allowed and saves some allocation too.
	var fallbackWS memdb.WatchSet
	if !watchOptimized {
		// We weren't able to use the optimization of watching only service indexes
		// for some reason. That means we need to fallback to watching everything we
		// touch in the DB as normal. We plumb the caller's watchset through (note
		// it's a map so this is a by-reference assignment.)
		fallbackWS = ws
		// We also need to watch the iterator from earlier too.
		fallbackWS.Add(iter.WatchCh())
	} else if connect {
		// If this is a connect query then there is a subtlety to watch out for.
		// In addition to watching the proxy service indexes for changes above, we
		// need to still keep an eye on the connect service index in case a new
		// proxy with a new name registers - we are only watching proxy service
		// names we know about above so we'd miss that otherwise. Thankfully this
		// is only ever one extra chan to watch and will catch any changes to
		// proxy registrations for this target service.
		ws.Add(iter.WatchCh())
	}

	return parseCheckServiceNodes(tx, fallbackWS, idx, results, entMeta, peerName, err)
}

// CheckServiceTagNodes is used to query all nodes and checks for a given
// service, filtering out services that don't contain the given tag.
func (s *Store) CheckServiceTagNodes(ws memdb.WatchSet, serviceName string, tags []string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer value
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	q := Query{Value: serviceName, EnterpriseMeta: *entMeta, PeerName: peerName}
	iter, err := tx.Get(tableServices, indexService, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	// Return the results, filtering by tag.
	serviceExists := false
	var results structs.ServiceNodes
	for service := iter.Next(); service != nil; service = iter.Next() {
		svc := service.(*structs.ServiceNode)
		serviceExists = true
		if !serviceTagsFilter(svc, tags) {
			results = append(results, svc)
		}
	}

	// Get the table index.
	idx := maxIndexForService(tx, serviceName, serviceExists, true, entMeta, peerName)
	return parseCheckServiceNodes(tx, ws, idx, results, entMeta, peerName, err)
}

// GatewayServices is used to query all services associated with a gateway
func (s *Store) GatewayServices(ws memdb.WatchSet, gateway string, entMeta *acl.EnterpriseMeta) (uint64, structs.GatewayServices, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := tx.Get(tableGatewayServices, indexGateway, structs.NewServiceName(gateway, entMeta))
	if err != nil {
		return 0, nil, fmt.Errorf("failed gateway services lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	maxIdx, results, err := s.collectGatewayServices(tx, ws, iter)
	if err != nil {
		return 0, nil, err
	}
	idx := maxIndexTxn(tx, tableGatewayServices)

	return lib.MaxUint64(maxIdx, idx), results, nil
}

// TODO: Find a way to consolidate this with CheckIngressServiceNodes
// ServiceGateways is used to query all gateways associated with a service
func (s *Store) ServiceGateways(ws memdb.WatchSet, service string, kind structs.ServiceKind, entMeta acl.EnterpriseMeta) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// tableGatewayServices is not peer-aware, and the existence of TG/IG gateways is scrubbed during peer replication.
	maxIdx, nodes, err := serviceGatewayNodes(tx, ws, service, kind, &entMeta, structs.DefaultPeerKeyword)

	// Watch for index changes to the gateway nodes
	idx, chans := maxIndexAndWatchChsForServiceNodes(tx, nodes, false)
	for _, ch := range chans {
		ws.Add(ch)
	}
	maxIdx = lib.MaxUint64(maxIdx, idx)

	return parseCheckServiceNodes(tx, ws, maxIdx, nodes, &entMeta, structs.DefaultPeerKeyword, err)
}

func (s *Store) VirtualIPForService(psn structs.PeeredServiceName) (string, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	vip, err := tx.First(tableServiceVirtualIPs, indexID, psn)
	if err != nil {
		return "", fmt.Errorf("failed service virtual IP lookup: %s", err)
	}
	if vip == nil {
		return "", nil
	}

	return vip.(ServiceVirtualIP).IPWithOffset()
}

func (s *Store) ServiceVirtualIPs() (uint64, []ServiceVirtualIP, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return servicesVirtualIPsTxn(tx, nil)
}

func servicesVirtualIPsTxn(tx ReadTxn, ws memdb.WatchSet) (uint64, []ServiceVirtualIP, error) {
	iter, err := tx.Get(tableServiceVirtualIPs, indexID)
	if err != nil {
		return 0, nil, err
	}
	ws.Add(iter.WatchCh())

	var vips []ServiceVirtualIP
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		vip := raw.(ServiceVirtualIP)
		vips = append(vips, vip)
	}

	idx := maxIndexWatchTxn(tx, nil, tableServiceVirtualIPs)

	return idx, vips, nil
}

func (s *Store) ServiceManualVIPs(psn structs.PeeredServiceName) (*ServiceVirtualIP, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return serviceVIPsTxn(tx, psn)
}

func serviceVIPsTxn(tx ReadTxn, psn structs.PeeredServiceName) (*ServiceVirtualIP, error) {
	vip, err := tx.First(tableServiceVirtualIPs, indexID, psn)
	if err != nil {
		return nil, fmt.Errorf("failed service virtual IP lookup: %s", err)
	}
	if vip == nil {
		return nil, nil
	}

	entry := vip.(ServiceVirtualIP)
	return &entry, nil
}

// VirtualIPsForAllImportedServices returns a slice of ServiceVirtualIP for all
// VirtualIP-assignable services that have been imported by the partition represented in entMeta.
// Namespace is ignored.
func (s *Store) VirtualIPsForAllImportedServices(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []ServiceVirtualIP, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	q := Query{
		EnterpriseMeta: entMeta,
		// Wildcard peername is used by prefix index to fetch all remote peers for a partition.
		PeerName: "*",
	}
	iter, err := tx.Get(tableServiceVirtualIPs, indexID+"_prefix", q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service virtual IP lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	idx := maxIndexTxn(tx, partitionedIndexEntryName(tableServiceVirtualIPs+".imported", entMeta.PartitionOrDefault()))

	var vips []ServiceVirtualIP
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		vip := raw.(ServiceVirtualIP)
		vips = append(vips, vip)
	}

	return idx, vips, nil
}

func (s *Store) ServiceNamesOfKind(ws memdb.WatchSet, kind structs.ServiceKind) (uint64, []*KindServiceName, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	wildcardMeta := structs.WildcardEnterpriseMetaInPartition(structs.WildcardSpecifier)
	return serviceNamesOfKindTxn(tx, ws, kind, *wildcardMeta)
}

func serviceNamesOfKindTxn(tx ReadTxn, ws memdb.WatchSet, kind structs.ServiceKind, entMeta acl.EnterpriseMeta) (uint64, []*KindServiceName, error) {
	iter, err := tx.Get(tableKindServiceNames, indexKind, Query{Value: string(kind), EnterpriseMeta: entMeta})
	if err != nil {
		return 0, nil, err
	}

	// TODO(peering): Maybe delete this watch and rely on the max idx tables below, to avoid waking up on unrelated changes
	ws.Add(iter.WatchCh())

	var names []*KindServiceName
	for name := iter.Next(); name != nil; name = iter.Next() {
		ksn := name.(*KindServiceName)
		names = append(names, ksn)
	}

	var idx uint64
	switch {
	case entMeta.PartitionOrDefault() == structs.WildcardSpecifier:
		idx = kindServiceNamesMaxIndex(tx, ws, kind.Normalized())

	case entMeta.NamespaceOrDefault() == structs.WildcardSpecifier:
		idx = kindServiceNamesMaxIndex(tx, ws, partitionedIndexEntryName(kind.Normalized(), entMeta.PartitionOrDefault()))

	default:
		idx = kindServiceNamesMaxIndex(tx, ws, partitionedAndNamespacedIndexEntryName(kind.Normalized(), &entMeta))

	}
	return idx, names, nil
}

// parseCheckServiceNodes is used to parse through a given set of services,
// and query for an associated node and a set of checks. This is the inner
// method used to return a rich set of results from a more simple query.
//
// TODO: idx parameter is not used except as a return value. Remove it.
// TODO: err parameter is only used for early return. Remove it and check from the
// caller.
func parseCheckServiceNodes(
	tx ReadTxn, ws memdb.WatchSet, idx uint64,
	services structs.ServiceNodes,
	entMeta *acl.EnterpriseMeta,
	peerName string,
	err error,
) (uint64, structs.CheckServiceNodes, error) {
	if err != nil {
		return 0, nil, err
	}

	// Special-case the zero return value to nil, since this ends up in
	// external APIs.
	if len(services) == 0 {
		return idx, nil, nil
	}

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// We don't want to track an unlimited number of nodes, so we pull a
	// top-level watch to use as a fallback.
	allNodes, err := tx.Get(tableNodes, indexID+"_prefix", Query{
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	allNodesCh := allNodes.WatchCh()

	// We need a similar fallback for checks. Since services need the
	// status of node + service-specific checks, we pull in a top-level
	// watch over all checks.
	allChecks, err := tx.Get(tableChecks, indexID+"_prefix", Query{
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed checks lookup: %s", err)
	}
	allChecksCh := allChecks.WatchCh()

	results := make(structs.CheckServiceNodes, 0, len(services))
	for _, sn := range services {
		// Retrieve the node.
		watchCh, n, err := tx.FirstWatch(tableNodes, indexID, Query{
			Value:          sn.Node,
			EnterpriseMeta: sn.EnterpriseMeta,
			PeerName:       sn.PeerName,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("failed node lookup: %s", err)
		}
		ws.AddWithLimit(watchLimit, watchCh, allNodesCh)

		if n == nil {
			return 0, nil, ErrMissingNode
		}
		node := n.(*structs.Node)

		// First add the node-level checks. These always apply to any
		// service on the node.
		var checks structs.HealthChecks
		q := NodeServiceQuery{
			Node:           sn.Node,
			Service:        "", // node checks have no service
			EnterpriseMeta: *sn.EnterpriseMeta.WithWildcardNamespace(),
			PeerName:       sn.PeerName,
		}
		iter, err := tx.Get(tableChecks, indexNodeService, q)
		if err != nil {
			return 0, nil, err
		}
		ws.AddWithLimit(watchLimit, iter.WatchCh(), allChecksCh)
		for check := iter.Next(); check != nil; check = iter.Next() {
			checks = append(checks, check.(*structs.HealthCheck))
		}

		// Now add the service-specific checks.
		q = NodeServiceQuery{
			Node:           sn.Node,
			Service:        sn.ServiceID,
			EnterpriseMeta: sn.EnterpriseMeta,
			PeerName:       sn.PeerName,
		}
		iter, err = tx.Get(tableChecks, indexNodeService, q)
		if err != nil {
			return 0, nil, err
		}
		ws.AddWithLimit(watchLimit, iter.WatchCh(), allChecksCh)
		for check := iter.Next(); check != nil; check = iter.Next() {
			checks = append(checks, check.(*structs.HealthCheck))
		}

		// Append to the results.
		results = append(results, structs.CheckServiceNode{
			Node:    node,
			Service: sn.ToNodeService(),
			Checks:  checks,
		})
	}

	return idx, results, nil
}

// NodeInfo is used to generate a dump of a single node. The dump includes
// all services and checks which are registered against the node.
func (s *Store) NodeInfo(ws memdb.WatchSet, node string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.NodeDump, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Get the table index.
	idx := catalogMaxIndex(tx, entMeta, peerName, true)

	// Query the node by the passed node
	nodes, err := tx.Get(tableNodes, indexID, Query{
		Value:          node,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed node lookup: %s", err)
	}
	ws.Add(nodes.WatchCh())
	return parseNodes(tx, ws, idx, nodes, entMeta, peerName)
}

// NodeDump is used to generate a dump of all nodes. This call is expensive
// as it has to query every node, service, and check. The response can also
// be quite large since there is currently no filtering applied.
func (s *Store) NodeDump(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.NodeDump, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	if entMeta == nil {
		entMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Get the table index.
	idx := catalogMaxIndex(tx, entMeta, peerName, true)

	// Fetch all of the registered nodes
	q := Query{
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	nodes, err := tx.Get(tableNodes, indexID+"_prefix", q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed node lookup: %s", err)
	}
	ws.Add(nodes.WatchCh())
	return parseNodes(tx, ws, idx, nodes, entMeta, peerName)
}

func (s *Store) ServiceDump(ws memdb.WatchSet, kind structs.ServiceKind, useKind bool, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	if useKind {
		return serviceDumpKindTxn(tx, ws, kind, entMeta, peerName)
	} else {
		return serviceDumpAllTxn(tx, ws, entMeta, peerName)
	}
}

func serviceDumpAllTxn(tx ReadTxn, ws memdb.WatchSet, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error) {
	// Get the table index
	idx := catalogMaxIndexWatch(tx, ws, entMeta, "", true)

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	q := Query{
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	services, err := tx.Get(tableServices, indexID+"_prefix", q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}

	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		sn := service.(*structs.ServiceNode)
		results = append(results, sn)
	}

	return parseCheckServiceNodes(tx, nil, idx, results, entMeta, peerName, err)
}

func serviceDumpKindTxn(tx ReadTxn, ws memdb.WatchSet, kind structs.ServiceKind, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error) {
	// unlike when we are dumping all services here we only need to watch the kind specific index entry for changing (or nodes, checks)
	// updating any services, nodes or checks will bump the appropriate service kind index so there is no need to watch any of the individual
	// entries
	idx := catalogServiceKindMaxIndex(tx, ws, kind, entMeta, peerName)

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	q := Query{
		Value:          string(kind),
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	services, err := tx.Get(tableServices, indexKind, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}

	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		sn := service.(*structs.ServiceNode)
		results = append(results, sn)
	}

	return parseCheckServiceNodes(tx, nil, idx, results, entMeta, peerName, err)
}

// parseNodes takes an iterator over a set of nodes and returns a struct
// containing the nodes along with all of their associated services
// and/or health checks.
// TODO(peering): support parsing by peerName
func parseNodes(tx ReadTxn, ws memdb.WatchSet, idx uint64,
	iter memdb.ResultIterator, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.NodeDump, error) {

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// We don't want to track an unlimited number of services, so we pull a
	// top-level watch to use as a fallback.
	q := Query{
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	}
	allServices, err := tx.Get(tableServices, indexID+"_prefix", q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed services lookup: %s", err)
	}
	allServicesCh := allServices.WatchCh()

	// We need a similar fallback for checks.
	allChecks, err := tx.Get(tableChecks, indexID+"_prefix", entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed checks lookup: %s", err)
	}
	allChecksCh := allChecks.WatchCh()

	var results structs.NodeDump
	for n := iter.Next(); n != nil; n = iter.Next() {
		node := n.(*structs.Node)

		// Create the wrapped node
		dump := &structs.NodeInfo{
			ID:              node.ID,
			Node:            node.Node,
			Partition:       node.Partition,
			PeerName:        node.PeerName,
			Address:         node.Address,
			TaggedAddresses: node.TaggedAddresses,
			Meta:            node.Meta,
		}

		// Query the node services
		services, err := catalogServiceListByNode(tx, node.Node, entMeta, node.PeerName, true)
		if err != nil {
			return 0, nil, fmt.Errorf("failed services lookup: %s", err)
		}
		ws.AddWithLimit(watchLimit, services.WatchCh(), allServicesCh)
		for service := services.Next(); service != nil; service = services.Next() {
			ns := service.(*structs.ServiceNode).ToNodeService()
			dump.Services = append(dump.Services, ns)
		}

		// Query the service level checks
		checks, err := catalogListChecksByNode(tx, Query{
			Value:          node.Node,
			EnterpriseMeta: *entMeta,
			PeerName:       node.PeerName,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("failed node lookup: %s", err)
		}
		ws.AddWithLimit(watchLimit, checks.WatchCh(), allChecksCh)
		for check := checks.Next(); check != nil; check = checks.Next() {
			hc := check.(*structs.HealthCheck)
			dump.Checks = append(dump.Checks, hc)
		}

		// Add the result to the slice
		results = append(results, dump)
	}
	return idx, results, nil
}

// checkSessionsTxn returns the IDs of all sessions associated with a health check
func checkSessionsTxn(tx ReadTxn, hc *structs.HealthCheck) ([]*sessionCheck, error) {
	// TODO(peering): what are implications for imported health checks?
	mappings, err := tx.Get(tableSessionChecks, indexNodeCheck, MultiQuery{Value: []string{hc.Node, string(hc.CheckID)},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(hc.PartitionOrDefault())})
	if err != nil {
		return nil, fmt.Errorf("failed session checks lookup: %s", err)
	}

	var sessions []*sessionCheck
	for mapping := mappings.Next(); mapping != nil; mapping = mappings.Next() {
		sessions = append(sessions, mapping.(*sessionCheck))
	}
	return sessions, nil
}

// updateGatewayServices associates services with gateways as specified in a gateway config entry
func updateGatewayServices(tx WriteTxn, idx uint64, conf structs.ConfigEntry, entMeta *acl.EnterpriseMeta) error {
	var (
		noChange        bool
		gatewayServices structs.GatewayServices
		err             error
	)

	gateway := structs.NewServiceName(conf.GetName(), entMeta)
	switch conf.GetKind() {
	case structs.IngressGateway:
		noChange, gatewayServices, err = ingressConfigGatewayServices(tx, gateway, conf, entMeta)
	case structs.TerminatingGateway:
		noChange, gatewayServices, err = terminatingConfigGatewayServices(tx, gateway, conf, entMeta)
	default:
		return fmt.Errorf("config entry kind %q does not need gateway-services", conf.GetKind())
	}
	// Return early if there is an error OR we don't have any services to update
	if err != nil || noChange {
		return err
	}

	// Update terminating gateway service virtual IPs
	vipsSupported, err := terminatingGatewayVirtualIPsSupported(tx, nil)
	if err != nil {
		return err
	}
	if vipsSupported && conf.GetKind() == structs.TerminatingGateway {
		gatewayConf := conf.(*structs.TerminatingGatewayConfigEntry)
		if err := updateTerminatingGatewayVirtualIPs(tx, idx, gatewayConf, entMeta); err != nil {
			return err
		}
	}

	// Delete all associated with gateway first, to avoid keeping mappings that were removed
	sn := structs.NewServiceName(conf.GetName(), entMeta)

	if _, err := tx.DeleteAll(tableGatewayServices, indexGateway, sn); err != nil {
		return fmt.Errorf("failed to truncate gateway services table: %v", err)
	}
	if err := truncateGatewayServiceTopologyMappings(tx, idx, sn, conf.GetKind()); err != nil {
		return fmt.Errorf("failed to truncate mesh topology for gateway: %v", err)
	}

	for _, svc := range gatewayServices {
		// If the service is a wildcard we need to target all services within the namespace
		if svc.Service.Name == structs.WildcardSpecifier {
			if err := updateGatewayNamespace(tx, idx, svc, entMeta); err != nil {
				return fmt.Errorf("failed to associate gateway %q with wildcard: %v", gateway.String(), err)
			}
			// Skip service-specific update below if there was a wildcard update
			continue
		}

		// Since this service was specified on its own, and not with a wildcard,
		// if there is an existing entry, we overwrite it. The service entry is the source of truth.
		//
		// By extension, if TLS creds are provided with a wildcard but are not provided in
		// the service entry, the service does not inherit the creds from the wildcard.
		err = updateGatewayService(tx, idx, svc)
		if err != nil {
			return err
		}
	}

	if err := indexUpdateMaxTxn(tx, idx, tableGatewayServices); err != nil {
		return fmt.Errorf("failed updating gateway-services index: %v", err)
	}
	return nil
}

func getTermGatewayVirtualIPs(
	tx WriteTxn,
	idx uint64,
	services []structs.LinkedService,
	entMeta *acl.EnterpriseMeta,
) (map[string]structs.ServiceAddress, error) {
	addrs := make(map[string]structs.ServiceAddress, len(services))
	for _, s := range services {
		sn := structs.ServiceName{Name: s.Name, EnterpriseMeta: *entMeta}
		// Terminating Gateways cannot route to services in peered clusters
		psn := structs.PeeredServiceName{ServiceName: sn, Peer: structs.DefaultPeerKeyword}
		vip, err := assignServiceVirtualIP(tx, idx, psn)
		if err != nil {
			return nil, err
		}
		key := structs.ServiceGatewayVirtualIPTag(sn)
		addrs[key] = structs.ServiceAddress{Address: vip}
	}

	return addrs, nil
}

func updateTerminatingGatewayVirtualIPs(tx WriteTxn, idx uint64, conf *structs.TerminatingGatewayConfigEntry, entMeta *acl.EnterpriseMeta) error {
	// Build the current map of services with virtual IPs for this gateway
	services := conf.Services
	addrs, err := getTermGatewayVirtualIPs(tx, idx, services, entMeta)
	if err != nil {
		return err
	}

	// Find any deleted service entries by comparing the new config entry to the existing one.
	_, existing, err := configEntryTxn(tx, nil, conf.GetKind(), conf.GetName(), entMeta)
	if err != nil {
		return fmt.Errorf("failed to get config entry: %v", err)
	}
	var deletes []structs.ServiceName
	cfg, ok := existing.(*structs.TerminatingGatewayConfigEntry)
	if ok {
		for _, s := range cfg.Services {
			sn := structs.ServiceName{Name: s.Name, EnterpriseMeta: *entMeta}
			key := structs.ServiceGatewayVirtualIPTag(sn)
			if _, ok := addrs[key]; !ok {
				deletes = append(deletes, sn)
			}
		}
	}

	q := Query{Value: conf.GetName(), EnterpriseMeta: *entMeta}
	_, svcNodes, err := serviceNodesTxn(tx, nil, indexService, q)
	if err != nil {
		return err
	}

	// Update the tagged addrs for any existing instances of this terminating gateway.
	for _, s := range svcNodes {
		newAddrs := make(map[string]structs.ServiceAddress)
		for key, addr := range s.ServiceTaggedAddresses {
			if !strings.HasPrefix(key, structs.TaggedAddressVirtualIP+":") {
				newAddrs[key] = addr
			}
		}
		for key, addr := range addrs {
			newAddrs[key] = addr
		}

		// Don't need to update the service record if it's a no-op.
		if reflect.DeepEqual(newAddrs, s.ServiceTaggedAddresses) {
			continue
		}

		newSN := s.PartialClone()
		newSN.ServiceTaggedAddresses = newAddrs
		newSN.ModifyIndex = idx
		if err := catalogInsertService(tx, newSN); err != nil {
			return err
		}
	}

	// Check if we can delete any virtual IPs for the removed services.
	gatewayName := structs.NewServiceName(conf.GetName(), entMeta)
	for _, sn := range deletes {
		// If there's no existing service nodes, attempt to free the virtual IP.
		q := Query{Value: sn.Name, EnterpriseMeta: sn.EnterpriseMeta}
		_, nodes, err := serviceNodesTxn(tx, nil, indexConnect, q)
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			psn := structs.PeeredServiceName{Peer: structs.DefaultPeerKeyword, ServiceName: sn}
			if err := freeServiceVirtualIP(tx, idx, psn, &gatewayName); err != nil {
				return err
			}
		}
	}

	return nil
}

// ingressConfigGatewayServices constructs a list of GatewayService structs for
// insertion into the memdb table, specific to ingress gateways. The boolean
// returned indicates that there are no changes necessary to the memdb table.
func ingressConfigGatewayServices(
	tx ReadTxn,
	gateway structs.ServiceName,
	conf structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
) (bool, structs.GatewayServices, error) {
	entry, ok := conf.(*structs.IngressGatewayConfigEntry)
	if !ok {
		return false, nil, fmt.Errorf("unexpected config entry type: %T", conf)
	}

	// Check if service list matches the last known list for the config entry, if it does, skip the update
	_, c, err := configEntryTxn(tx, nil, conf.GetKind(), conf.GetName(), entMeta)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get config entry: %v", err)
	}
	if cfg, ok := c.(*structs.IngressGatewayConfigEntry); ok && cfg != nil {
		if reflect.DeepEqual(cfg.Listeners, entry.Listeners) {
			// Services are the same, nothing to update
			return true, nil, nil
		}
	}

	var gatewayServices structs.GatewayServices
	for _, listener := range entry.Listeners {
		for _, service := range listener.Services {
			mapping := &structs.GatewayService{
				Gateway:     gateway,
				Service:     service.ToServiceName(),
				GatewayKind: structs.ServiceKindIngressGateway,
				Hosts:       service.Hosts,
				Port:        listener.Port,
				Protocol:    listener.Protocol,
			}

			gatewayServices = append(gatewayServices, mapping)
		}
	}
	return false, gatewayServices, nil
}

// terminatingConfigGatewayServices constructs a list of GatewayService structs
// for insertion into the memdb table, specific to terminating gateways. The
// boolean returned indicates that there are no changes necessary to the memdb
// table.
func terminatingConfigGatewayServices(
	tx ReadTxn,
	gateway structs.ServiceName,
	conf structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
) (bool, structs.GatewayServices, error) {
	entry, ok := conf.(*structs.TerminatingGatewayConfigEntry)
	if !ok {
		return false, nil, fmt.Errorf("unexpected config entry type: %T", conf)
	}

	// Check if service list matches the last known list for the config entry, if it does, skip the update
	_, c, err := configEntryTxn(tx, nil, conf.GetKind(), conf.GetName(), entMeta)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get config entry: %v", err)
	}
	if cfg, ok := c.(*structs.TerminatingGatewayConfigEntry); ok && cfg != nil {
		if reflect.DeepEqual(cfg.Services, entry.Services) {
			// Services are the same, nothing to update
			return true, nil, nil
		}
	}

	var gatewayServices structs.GatewayServices
	for _, svc := range entry.Services {
		kind, err := GatewayServiceKind(tx, svc.Name, &svc.EnterpriseMeta)
		if err != nil {
			return false, nil, fmt.Errorf("failed to get gateway service kind for service %s: %v", svc.Name, err)
		}
		mapping := &structs.GatewayService{
			Gateway:     gateway,
			Service:     structs.NewServiceName(svc.Name, &svc.EnterpriseMeta),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			KeyFile:     svc.KeyFile,
			CertFile:    svc.CertFile,
			CAFile:      svc.CAFile,
			SNI:         svc.SNI,
			ServiceKind: kind,
		}

		gatewayServices = append(gatewayServices, mapping)
	}
	return false, gatewayServices, nil
}

func GatewayServiceKind(tx ReadTxn, name string, entMeta *acl.EnterpriseMeta) (structs.GatewayServiceKind, error) {
	serviceIter, err := tx.First(tableServices, indexService, Query{
		Value:          name,
		EnterpriseMeta: *entMeta,
	})
	if err != nil {
		return structs.GatewayServiceKindUnknown, err
	}
	if serviceIter != nil {
		return structs.GatewayServiceKindService, err
	}

	_, entry, err := configEntryTxn(tx, nil, structs.ServiceDefaults, name, entMeta)
	if err != nil {
		return structs.GatewayServiceKindUnknown, err
	}
	if entry != nil {
		sd, ok := entry.(*structs.ServiceConfigEntry)
		if !ok {
			return structs.GatewayServiceKindUnknown, fmt.Errorf("invalid config entry type %T", entry)
		}
		if sd.Destination != nil {
			return structs.GatewayServiceKindDestination, nil
		}
	}
	return structs.GatewayServiceKindUnknown, nil
}

// updateGatewayNamespace is used to target all services within a namespace
func updateGatewayNamespace(tx WriteTxn, idx uint64, service *structs.GatewayService, entMeta *acl.EnterpriseMeta) error {
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	q := Query{Value: string(structs.ServiceKindTypical), EnterpriseMeta: *entMeta}
	services, err := tx.Get(tableServices, indexKind, q)
	if err != nil {
		return fmt.Errorf("failed querying services: %s", err)
	}

	// Iterate over services in namespace and insert mapping for each
	for svc := services.Next(); svc != nil; svc = services.Next() {
		sn := svc.(*structs.ServiceNode)

		// Only associate non-consul services with gateways
		if sn.ServiceName == "consul" {
			continue
		}

		hasConnectInstance, hasNonConnectInstance, err := serviceHasConnectInstances(tx, sn.ServiceName, entMeta)
		if err != nil {
			return err
		}

		if service.GatewayKind == structs.ServiceKindIngressGateway && !hasConnectInstance {
			continue
		}
		if service.GatewayKind == structs.ServiceKindTerminatingGateway && !hasNonConnectInstance {
			continue
		}

		existing, err := tx.First(tableGatewayServices, indexID, service.Gateway, sn.CompoundServiceName().ServiceName, service.Port)
		if err != nil {
			return fmt.Errorf("gateway service lookup failed: %s", err)
		}
		if existing != nil {
			// If there's an existing service associated with this gateway then we skip it.
			// This means the service was specified on its own, and the service entry overrides the wildcard entry.
			continue
		}

		mapping := service.Clone()

		mapping.Service = structs.NewServiceName(sn.ServiceName, &service.Service.EnterpriseMeta)
		mapping.FromWildcard = true

		err = updateGatewayService(tx, idx, mapping)
		if err != nil {
			return err
		}
	}
	entries, err := tx.Get(tableConfigEntries, indexID+"_prefix", ConfigEntryKindQuery{Kind: structs.ServiceDefaults, EnterpriseMeta: *entMeta})
	if err != nil {
		return fmt.Errorf("failed querying entries: %s", err)
	}
	for entry := entries.Next(); entry != nil; entry = entries.Next() {
		e := entry.(*structs.ServiceConfigEntry)
		if e.Destination == nil {
			continue
		}

		sn := structs.ServiceName{
			Name:           e.Name,
			EnterpriseMeta: e.EnterpriseMeta,
		}
		existing, err := tx.First(tableGatewayServices, indexID, service.Gateway, sn, service.Port)
		if err != nil {
			return fmt.Errorf("gateway service lookup failed: %s", err)
		}
		if existing != nil {
			// If there's an existing service associated with this gateway then we skip it.
			// This means the service was specified on its own, and the service entry overrides the wildcard entry.
			continue
		}

		mapping := service.Clone()

		mapping.Service = structs.NewServiceName(e.Name, &service.Service.EnterpriseMeta)
		mapping.ServiceKind = structs.GatewayServiceKindDestination
		mapping.FromWildcard = true

		err = updateGatewayService(tx, idx, mapping)
		if err != nil {
			return err
		}
	}

	// Also store a mapping for the wildcard so that the TLS creds can be pulled
	// for new services registered in its namespace
	err = updateGatewayService(tx, idx, service)
	if err != nil {
		return err
	}
	return nil
}

// serviceHasConnectInstances returns whether the service has at least one connect instance,
// and at least one non-connect instance.
func serviceHasConnectInstances(tx WriteTxn, serviceName string, entMeta *acl.EnterpriseMeta) (bool, bool, error) {
	hasConnectInstance := false
	query := Query{
		Value:          serviceName,
		EnterpriseMeta: *entMeta,
	}
	svc, err := tx.First(tableServices, indexConnect, query)
	if err != nil {
		return false, false, fmt.Errorf("failed service lookup: %s", err)
	}
	if svc != nil {
		hasConnectInstance = true
	}

	hasNonConnectInstance := false
	iter, err := tx.Get(tableServices, indexService, query)
	if err != nil {
		return false, false, fmt.Errorf("failed service lookup: %s", err)
	}
	for service := iter.Next(); service != nil; service = iter.Next() {
		sn := service.(*structs.ServiceNode)
		if !sn.ServiceConnect.Native {
			hasNonConnectInstance = true
			break
		}
	}

	return hasConnectInstance, hasNonConnectInstance, nil
}

// serviceHasConnectEnabledInstances returns whether the given service name
// has a corresponding connect-proxy or connect-native instance.
// This function is mostly a clone of `serviceHasConnectInstances`, but it has
// an early return to improve performance and returns true if at least one
// connect-native instance exists.
func serviceHasConnectEnabledInstances(tx WriteTxn, serviceName string, entMeta *acl.EnterpriseMeta) (bool, error) {
	query := Query{
		Value:          serviceName,
		EnterpriseMeta: *entMeta,
	}

	svc, err := tx.First(tableServices, indexConnect, query)
	if err != nil {
		return false, fmt.Errorf("failed service lookup: %w", err)
	}
	if svc != nil {
		return true, nil
	}
	return false, nil
}

// updateGatewayService associates services with gateways after an eligible event
// ie. Registering a service in a namespace targeted by a gateway
func updateGatewayService(tx WriteTxn, idx uint64, mapping *structs.GatewayService) error {
	// Check if mapping already exists in table if it's already in the table
	// Avoid insert if nothing changed
	existing, err := tx.First(tableGatewayServices, indexID, mapping.Gateway, mapping.Service, mapping.Port)
	if err != nil {
		return fmt.Errorf("gateway service lookup failed: %s", err)
	}
	if gs, ok := existing.(*structs.GatewayService); ok && gs != nil {
		mapping.CreateIndex = gs.CreateIndex
		if gs.IsSame(mapping) {
			return nil
		}
	} else {
		// We have a new mapping
		mapping.CreateIndex = idx
	}
	mapping.ModifyIndex = idx

	if err := tx.Insert(tableGatewayServices, mapping); err != nil {
		return fmt.Errorf("failed inserting gateway service mapping: %s", err)
	}

	if err := indexUpdateMaxTxn(tx, idx, tableGatewayServices); err != nil {
		return fmt.Errorf("failed updating gateway-services index: %v", err)
	}

	if err := insertGatewayServiceTopologyMapping(tx, idx, mapping); err != nil {
		return fmt.Errorf("failed to reconcile mesh topology for gateway: %v", err)
	}
	return nil
}

// checkWildcardForGatewaysAndUpdate checks whether a service matches a
// wildcard definition in gateway config entries and if so adds it the the
// gateway-services table.
func checkGatewayWildcardsAndUpdate(tx WriteTxn, idx uint64, svc *structs.ServiceName, ns *structs.NodeService, kind structs.GatewayServiceKind) error {
	sn := structs.ServiceName{Name: structs.WildcardSpecifier, EnterpriseMeta: svc.EnterpriseMeta}
	svcGateways, err := tx.Get(tableGatewayServices, indexService, sn)
	if err != nil {
		return fmt.Errorf("failed gateway lookup for %q: %s", svc.Name, err)
	}

	hasConnectInstance, hasNonConnectInstance, err := serviceHasConnectInstances(tx, svc.Name, &svc.EnterpriseMeta)
	if err != nil {
		return err
	}
	// If we were passed a NodeService, this might be the first registered instance of the service
	// so we need to count it as either a connect or non-connect instance.
	if ns != nil {
		if ns.Connect.Native || ns.Kind == structs.ServiceKindConnectProxy {
			hasConnectInstance = true
		} else {
			hasNonConnectInstance = true
		}
	}

	for service := svcGateways.Next(); service != nil; service = svcGateways.Next() {
		if wildcardSvc, ok := service.(*structs.GatewayService); ok && wildcardSvc != nil {
			if wildcardSvc.GatewayKind == structs.ServiceKindIngressGateway && !hasConnectInstance {
				continue
			}
			if wildcardSvc.GatewayKind == structs.ServiceKindTerminatingGateway && !hasNonConnectInstance && kind != structs.GatewayServiceKindDestination {
				continue
			}

			// Copy the wildcard mapping and modify it
			gatewaySvc := wildcardSvc.Clone()

			gatewaySvc.Service = structs.NewServiceName(svc.Name, &svc.EnterpriseMeta)
			gatewaySvc.FromWildcard = true
			gatewaySvc.ServiceKind = kind

			if err = updateGatewayService(tx, idx, gatewaySvc); err != nil {
				return fmt.Errorf("Failed to associate service %q with gateway %q", gatewaySvc.Service.String(), gatewaySvc.Gateway.String())
			}
		}
	}
	return nil
}

// checkGatewayAndUpdate checks whether a service matches a
// wildcard definition in gateway config entries and if so adds it the the
// gateway-services table.
func checkGatewayAndUpdate(tx WriteTxn, idx uint64, svc *structs.ServiceName, kind structs.GatewayServiceKind) error {
	sn := structs.ServiceName{Name: svc.Name, EnterpriseMeta: svc.EnterpriseMeta}
	svcGateways, err := tx.First(tableGatewayServices, indexService, sn)
	if err != nil {
		return fmt.Errorf("failed gateway lookup for %q: %s", svc.Name, err)
	}

	if service, ok := svcGateways.(*structs.GatewayService); ok && service != nil {
		// Copy the wildcard mapping and modify it
		gatewaySvc := service.Clone()

		gatewaySvc.Service = structs.NewServiceName(svc.Name, &svc.EnterpriseMeta)
		gatewaySvc.ServiceKind = kind

		if err = updateGatewayService(tx, idx, gatewaySvc); err != nil {
			return fmt.Errorf("Failed to associate service %q with gateway %q", gatewaySvc.Service.String(), gatewaySvc.Gateway.String())
		}
	}

	return nil
}

func cleanupGatewayWildcards(tx WriteTxn, idx uint64, sn structs.ServiceName, cleaningUpDestination bool) error {
	// Clean up association between service name and gateways if needed
	gateways, err := tx.Get(tableGatewayServices, indexService, sn)
	if err != nil {
		return fmt.Errorf("failed gateway lookup for %q: %s", sn.Name, err)
	}

	mappings := make([]*structs.GatewayService, 0)
	for mapping := gateways.Next(); mapping != nil; mapping = gateways.Next() {
		if gs, ok := mapping.(*structs.GatewayService); ok && gs != nil {
			mappings = append(mappings, gs)
		}
	}

	// Check whether there are any connect or non-connect instances remaining for this service.
	// If there are no connect instances left, ingress gateways with a wildcard entry can remove
	// their association with it (same with terminating gateways if there are no non-connect
	// instances left).
	hasConnectInstance, hasNonConnectInstance, err := serviceHasConnectInstances(tx, sn.Name, &sn.EnterpriseMeta)
	if err != nil {
		return err
	}

	// If we're deleting a service instance but this service is defined as a destination via config entry,
	// keep the mapping around.
	hasDestination := false
	if !cleaningUpDestination {
		q := configentry.NewKindName(structs.ServiceDefaults, sn.Name, &sn.EnterpriseMeta)
		existing, err := tx.First(tableConfigEntries, indexID, q)
		if err != nil {
			return fmt.Errorf("failed config entry lookup: %s", err)
		}
		if existing != nil {
			if entry, ok := existing.(*structs.ServiceConfigEntry); ok && entry.Destination != nil {
				hasDestination = true
			}
		}
	}

	// Do the updates in a separate loop so we don't trash the iterator.
	for _, m := range mappings {
		// Only delete if association was created by a wildcard specifier.
		// Otherwise the service was specified in the config entry, and the association should be maintained
		// for when the service is re-registered
		if m.FromWildcard {
			if m.GatewayKind == structs.ServiceKindIngressGateway && hasConnectInstance {
				continue
			}
			if m.GatewayKind == structs.ServiceKindTerminatingGateway && (hasNonConnectInstance || hasDestination) {
				continue
			}

			if err := tx.Delete(tableGatewayServices, m); err != nil {
				return fmt.Errorf("failed to truncate gateway services table: %v", err)
			}
			if err := indexUpdateMaxTxn(tx, idx, tableGatewayServices); err != nil {
				return fmt.Errorf("failed updating gateway-services index: %v", err)
			}
			if err := deleteGatewayServiceTopologyMapping(tx, idx, m); err != nil {
				return fmt.Errorf("failed to reconcile mesh topology for gateway: %v", err)
			}
		} else {
			kind, err := GatewayServiceKind(tx, m.Service.Name, &m.Service.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("failed to get gateway service kind for service %s: %v", sn.Name, err)
			}
			checkGatewayAndUpdate(tx, idx, &structs.ServiceName{Name: m.Service.Name, EnterpriseMeta: m.Service.EnterpriseMeta}, kind)
		}
	}
	return nil
}

func (s *Store) DumpGatewayServices(ws memdb.WatchSet) (uint64, structs.GatewayServices, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	iter, err := tx.Get(tableGatewayServices, indexID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to dump gateway-services: %s", err)
	}
	ws.Add(iter.WatchCh())

	maxIdx, results, err := s.collectGatewayServices(tx, ws, iter)
	if err != nil {
		return 0, nil, err
	}
	idx := maxIndexTxn(tx, tableGatewayServices)

	return lib.MaxUint64(maxIdx, idx), results, nil
}

func (s *Store) collectGatewayServices(tx ReadTxn, ws memdb.WatchSet, iter memdb.ResultIterator) (uint64, structs.GatewayServices, error) {
	var maxIdx uint64
	var results structs.GatewayServices

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		gs := obj.(*structs.GatewayService)
		maxIdx = lib.MaxUint64(maxIdx, gs.ModifyIndex)

		if gs.Service.Name != structs.WildcardSpecifier {
			idx, matches, err := checkProtocolMatch(tx, ws, gs)
			if err != nil {
				return 0, nil, fmt.Errorf("failed checking protocol: %s", err)
			}
			maxIdx = lib.MaxUint64(maxIdx, idx)

			if matches {
				results = append(results, gs)
			}
		}
	}
	return maxIdx, results, nil
}

// TODO: How to handle index rolling back when a config entry is
// deleted that references a service?
// We might need something like the service_last_extinction index?
func serviceGatewayNodes(tx ReadTxn, ws memdb.WatchSet, service string, kind structs.ServiceKind, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.ServiceNodes, error) {
	if peerName != "" {
		return 0, nil, nil
	}

	// Look up gateway name associated with the service
	gws, err := tx.Get(tableGatewayServices, indexService, structs.NewServiceName(service, entMeta))
	if err != nil {
		return 0, nil, fmt.Errorf("failed gateway lookup: %s", err)
	}

	// Adding this channel to the WatchSet means that the watch will fire if a config entry targeting the service is added.
	// Otherwise, if there's no associated gateway, then no watch channel would be returned
	ws.Add(gws.WatchCh())

	var ret structs.ServiceNodes
	var maxIdx uint64

	for gateway := gws.Next(); gateway != nil; gateway = gws.Next() {
		mapping := gateway.(*structs.GatewayService)
		// TODO(ingress): Test this conditional
		if mapping.GatewayKind != kind {
			continue
		}
		maxIdx = lib.MaxUint64(maxIdx, mapping.ModifyIndex)

		// Look up nodes for gateway
		q := Query{
			Value:          mapping.Gateway.Name,
			EnterpriseMeta: mapping.Gateway.EnterpriseMeta,
		}
		gwServices, err := tx.Get(tableServices, indexService, q)
		if err != nil {
			return 0, nil, fmt.Errorf("failed service lookup: %s", err)
		}

		var exists bool
		for svc := gwServices.Next(); svc != nil; svc = gwServices.Next() {
			sn := svc.(*structs.ServiceNode)
			ret = append(ret, sn)

			// Tracking existence to know whether we should check extinction index for service
			exists = true
		}

		// This prevents the index from sliding back if case all instances of the gateway service are deregistered
		svcIdx := maxIndexForService(tx, mapping.Gateway.Name, exists, false, &mapping.Gateway.EnterpriseMeta, structs.DefaultPeerKeyword)
		maxIdx = lib.MaxUint64(maxIdx, svcIdx)

		// Ensure that blocking queries wake up if the gateway-service mapping exists, but the gateway does not exist yet
		if !exists {
			ws.Add(gwServices.WatchCh())
		}
	}
	return maxIdx, ret, nil
}

// metricsProtocolForIngressGateway determines the protocol that should be used when fetching metrics for an ingress gateway
// Since ingress gateways may have listeners with different protocols, favor capturing all traffic by only returning HTTP
// when all listeners are HTTP-like.
func metricsProtocolForIngressGateway(tx ReadTxn, ws memdb.WatchSet, sn structs.ServiceName) (uint64, string, error) {
	idx, conf, err := configEntryTxn(tx, ws, structs.IngressGateway, sn.Name, &sn.EnterpriseMeta)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get ingress-gateway config entry for %q: %v", sn.String(), err)
	}
	if conf == nil {
		return 0, "", nil
	}
	entry, ok := conf.(*structs.IngressGatewayConfigEntry)
	if !ok {
		return 0, "", fmt.Errorf("unexpected config entry type: %T", conf)
	}
	counts := make(map[string]int)
	for _, l := range entry.Listeners {
		if structs.IsProtocolHTTPLike(l.Protocol) {
			counts["http"] += 1
		} else {
			counts["tcp"] += 1
		}
	}
	protocol := "tcp"
	if counts["tcp"] == 0 && counts["http"] > 0 {
		protocol = "http"
	}
	return idx, protocol, nil
}

// checkProtocolMatch filters out any GatewayService entries added from a wildcard with a protocol
// that doesn't match the one configured in their discovery chain.
func checkProtocolMatch(tx ReadTxn, ws memdb.WatchSet, svc *structs.GatewayService) (uint64, bool, error) {
	if svc.GatewayKind != structs.ServiceKindIngressGateway || !svc.FromWildcard {
		return 0, true, nil
	}

	idx, protocol, err := protocolForService(tx, ws, svc.Service)
	if err != nil {
		return 0, false, err
	}

	return idx, svc.Protocol == protocol, nil
}

// TODO(freddy) Split this up. The upstream/downstream logic is very similar.
// TODO(freddy) Add comprehensive state store test
func (s *Store) ServiceTopology(
	ws memdb.WatchSet,
	dc, service string,
	kind structs.ServiceKind,
	defaultAllow acl.EnforcementDecision,
	entMeta *acl.EnterpriseMeta,
) (uint64, *structs.ServiceTopology, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	sn := structs.NewServiceName(service, entMeta)

	var (
		maxIdx           uint64
		protocol         string
		err              error
		fullyTransparent bool
		hasTransparent   bool
		connectNative    bool
	)
	switch kind {
	case structs.ServiceKindIngressGateway:
		maxIdx, protocol, err = metricsProtocolForIngressGateway(tx, ws, sn)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to fetch protocol for service %s: %v", sn.String(), err)
		}

	case structs.ServiceKindTypical:
		maxIdx, protocol, err = protocolForService(tx, ws, sn)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to fetch protocol for service %s: %v", sn.String(), err)
		}

		// Fetch connect endpoints for the target service in order to learn if its proxies are configured as
		// transparent proxies.
		if entMeta == nil {
			entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
		}
		q := Query{
			Value:          service,
			EnterpriseMeta: *entMeta,
			PeerName:       structs.TODOPeerKeyword,
		}

		idx, proxies, err := serviceNodesTxn(tx, ws, indexConnect, q)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to fetch connect endpoints for service %s: %v", sn.String(), err)
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		if len(proxies) == 0 {
			break
		}

		fullyTransparent = true
		for _, proxy := range proxies {
			switch proxy.ServiceProxy.Mode {
			case structs.ProxyModeTransparent:
				hasTransparent = true

			default:
				// Only consider the target proxy to be transparent when all instances are in that mode.
				// This is done because the flag is used to display warnings about proxies needing to enable
				// transparent proxy mode. If ANY instance isn't in the right mode then the warming applies.
				fullyTransparent = false
			}
			if proxy.ServiceConnect.Native {
				connectNative = true
			}
		}

	default:
		return 0, nil, fmt.Errorf("unsupported kind %q", kind)
	}

	idx, upstreamNames, err := upstreamsFromRegistrationTxn(tx, ws, sn)
	if err != nil {
		return 0, nil, err
	}
	if idx > maxIdx {
		maxIdx = idx
	}

	var upstreamSources = make(map[string]string)
	for _, un := range upstreamNames {
		upstreamSources[un.String()] = structs.TopologySourceRegistration
	}

	upstreamDecisions := make(map[string]structs.IntentionDecisionSummary)

	// Only transparent proxies / connect native services have upstreams from intentions
	if hasTransparent || connectNative {
		idx, intentionUpstreams, err := s.intentionTopologyTxn(tx, ws, sn, false, defaultAllow, structs.IntentionTargetService)
		if err != nil {
			return 0, nil, err
		}
		if idx > maxIdx {
			maxIdx = idx
		}

		for _, svc := range intentionUpstreams {
			if _, ok := upstreamSources[svc.Name.String()]; ok {
				// Avoid duplicating entry
				continue
			}
			upstreamDecisions[svc.Name.String()] = svc.Decision
			upstreamNames = append(upstreamNames, svc.Name)

			var source string
			switch {
			case svc.Decision.HasExact:
				source = structs.TopologySourceSpecificIntention
			case svc.Decision.DefaultAllow:
				source = structs.TopologySourceDefaultAllow
			default:
				source = structs.TopologySourceWildcardIntention
			}
			upstreamSources[svc.Name.String()] = source
		}
	}

	matchEntry := structs.IntentionMatchEntry{
		Namespace: entMeta.NamespaceOrDefault(),
		Partition: entMeta.PartitionOrDefault(),
		Name:      service,
	}
	_, srcIntentions, err := compatIntentionMatchOneTxn(tx, ws, matchEntry, structs.IntentionMatchSource, structs.IntentionTargetService)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to query intentions for %s", sn.String())
	}

	for _, un := range upstreamNames {
		opts := IntentionDecisionOpts{
			Target:           un.Name,
			Namespace:        un.NamespaceOrDefault(),
			Partition:        un.PartitionOrDefault(),
			Intentions:       srcIntentions,
			MatchType:        structs.IntentionMatchDestination,
			DefaultDecision:  defaultAllow,
			AllowPermissions: false,
		}
		decision, err := s.IntentionDecision(opts)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get intention decision from (%s) to (%s): %v",
				sn.String(), un.String(), err)
		}
		upstreamDecisions[un.String()] = decision
	}

	idx, unfilteredUpstreams, err := s.combinedServiceNodesTxn(tx, ws, upstreamNames, structs.DefaultPeerKeyword)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get upstreams for %q: %v", sn.String(), err)
	}
	if idx > maxIdx {
		maxIdx = idx
	}

	var upstreams structs.CheckServiceNodes
	for _, upstream := range unfilteredUpstreams {
		sn := upstream.Service.CompoundServiceName()
		if upstream.Service.Kind == structs.ServiceKindConnectProxy {
			sn = structs.NewServiceName(upstream.Service.Proxy.DestinationServiceName, &upstream.Service.EnterpriseMeta)
		}

		// Avoid returning upstreams from intentions when none of the proxy instances of the target are in transparent mode or connect native.
		if !hasTransparent && !connectNative && upstreamSources[sn.String()] != structs.TopologySourceRegistration {
			continue
		}
		upstreams = append(upstreams, upstream)
	}

	var foundUpstreams = make(map[structs.ServiceName]struct{})
	for _, csn := range upstreams {
		foundUpstreams[csn.Service.CompoundServiceName()] = struct{}{}
	}

	// Check upstream names that had no service instances to see if they are routing config.
	for _, un := range upstreamNames {
		if _, ok := foundUpstreams[un]; ok {
			continue
		}

		for _, kind := range serviceGraphKinds {
			idx, entry, err := configEntryTxn(tx, ws, kind, un.Name, &un.EnterpriseMeta)
			if err != nil {
				return 0, nil, err
			}
			if entry != nil {
				upstreamSources[un.String()] = structs.TopologySourceRoutingConfig
			}
			if idx > maxIdx {
				maxIdx = idx
			}
		}
	}

	idx, downstreamNames, err := s.downstreamsForServiceTxn(tx, ws, dc, sn)
	if err != nil {
		return 0, nil, err
	}
	if idx > maxIdx {
		maxIdx = idx
	}

	var downstreamSources = make(map[string]string)
	for _, dn := range downstreamNames {
		downstreamSources[dn.String()] = structs.TopologySourceRegistration
	}

	idx, intentionDownstreams, err := s.intentionTopologyTxn(tx, ws, sn, true, defaultAllow, structs.IntentionTargetService)
	if err != nil {
		return 0, nil, err
	}
	if idx > maxIdx {
		maxIdx = idx
	}

	downstreamDecisions := make(map[string]structs.IntentionDecisionSummary)
	for _, svc := range intentionDownstreams {
		if _, ok := downstreamSources[svc.Name.String()]; ok {
			// Avoid duplicating entry
			continue
		}
		downstreamNames = append(downstreamNames, svc.Name)
		downstreamDecisions[svc.Name.String()] = svc.Decision

		var source string
		switch {
		case svc.Decision.HasExact:
			source = structs.TopologySourceSpecificIntention
		case svc.Decision.DefaultAllow:
			source = structs.TopologySourceDefaultAllow
		default:
			source = structs.TopologySourceWildcardIntention
		}
		downstreamSources[svc.Name.String()] = source
	}

	_, dstIntentions, err := compatIntentionMatchOneTxn(tx, ws, matchEntry, structs.IntentionMatchDestination, structs.IntentionTargetService)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to query intentions for %s", sn.String())
	}
	for _, dn := range downstreamNames {
		opts := IntentionDecisionOpts{
			Target:           dn.Name,
			Namespace:        dn.NamespaceOrDefault(),
			Partition:        dn.PartitionOrDefault(),
			Intentions:       dstIntentions,
			MatchType:        structs.IntentionMatchSource,
			DefaultDecision:  defaultAllow,
			AllowPermissions: false,
		}
		decision, err := s.IntentionDecision(opts)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get intention decision from (%s) to (%s): %v",
				dn.String(), sn.String(), err)
		}
		downstreamDecisions[dn.String()] = decision
	}

	idx, unfilteredDownstreams, err := s.combinedServiceNodesTxn(tx, ws, downstreamNames, structs.DefaultPeerKeyword)

	if err != nil {
		return 0, nil, fmt.Errorf("failed to get downstreams for %q: %v", sn.String(), err)
	}
	if idx > maxIdx {
		maxIdx = idx
	}

	// Store downstreams with at least one instance in transparent proxy mode.
	// This is to avoid returning downstreams from intentions when none of the downstreams are transparent proxies.
	tproxyMap := make(map[structs.ServiceName]struct{})
	for _, downstream := range unfilteredDownstreams {
		if downstream.Service.Proxy.Mode == structs.ProxyModeTransparent {
			sn := structs.NewServiceName(downstream.Service.Proxy.DestinationServiceName, &downstream.Service.EnterpriseMeta)
			tproxyMap[sn] = struct{}{}
		}
	}

	var downstreams structs.CheckServiceNodes
	for _, downstream := range unfilteredDownstreams {
		sn := downstream.Service.CompoundServiceName()
		if downstream.Service.Kind == structs.ServiceKindConnectProxy {
			sn = structs.NewServiceName(downstream.Service.Proxy.DestinationServiceName, &downstream.Service.EnterpriseMeta)
		}
		if _, ok := tproxyMap[sn]; !ok && !downstream.Service.Connect.Native && downstreamSources[sn.String()] != structs.TopologySourceRegistration {
			// If downstream is not a transparent proxy or connect native, remove references
			delete(downstreamSources, sn.String())
			delete(downstreamDecisions, sn.String())
			continue
		}
		downstreams = append(downstreams, downstream)
	}

	resp := &structs.ServiceTopology{
		TransparentProxy:    fullyTransparent,
		MetricsProtocol:     protocol,
		Upstreams:           upstreams,
		Downstreams:         downstreams,
		UpstreamDecisions:   upstreamDecisions,
		DownstreamDecisions: downstreamDecisions,
		UpstreamSources:     upstreamSources,
		DownstreamSources:   downstreamSources,
	}
	return maxIdx, resp, nil
}

// combinedServiceNodesTxn returns typical and connect endpoints for a list of services.
// This enabled aggregating checks statuses across both.
func (s *Store) combinedServiceNodesTxn(tx ReadTxn, ws memdb.WatchSet, names []structs.ServiceName, peerName string) (uint64, structs.CheckServiceNodes, error) {
	var (
		maxIdx uint64
		resp   structs.CheckServiceNodes
	)
	for _, u := range names {
		// Collect typical then connect instances
		idx, csn, err := checkServiceNodesTxn(tx, ws, u.Name, false, &u.EnterpriseMeta, peerName)
		if err != nil {
			return 0, nil, err
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		resp = append(resp, csn...)

		idx, csn, err = checkServiceNodesTxn(tx, ws, u.Name, true, &u.EnterpriseMeta, peerName)
		if err != nil {
			return 0, nil, err
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		resp = append(resp, csn...)
	}
	return maxIdx, resp, nil
}

// downstreamsForServiceTxn will find all downstream services that could route traffic to the input service.
// There are two factors at play. Upstreams defined in a proxy registration, and the discovery chain for those upstreams.
func (s *Store) downstreamsForServiceTxn(tx ReadTxn, ws memdb.WatchSet, dc string, service structs.ServiceName) (uint64, []structs.ServiceName, error) {
	// First fetch services that have discovery chains that eventually route to the target service
	idx, sources, err := s.discoveryChainSourcesTxn(tx, ws, dc, service)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get sources for discovery chain target %q: %v", service.String(), err)
	}

	var maxIdx uint64
	if idx > maxIdx {
		maxIdx = idx
	}

	var (
		resp []structs.ServiceName
		seen = make(map[structs.ServiceName]bool)
	)
	for _, s := range sources {
		// We then follow these sources one level down to the services defining them as an upstream.
		idx, downstreams, err := downstreamsFromRegistrationTxn(tx, ws, s)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get registration downstreams for %q: %v", s.String(), err)
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		for _, d := range downstreams {
			if !seen[d] {
				resp = append(resp, d)
				seen[d] = true
			}
		}
	}
	return maxIdx, resp, nil
}

// upstreamsFromRegistrationTxn returns the ServiceNames of the upstreams defined across instances of the input
func upstreamsFromRegistrationTxn(tx ReadTxn, ws memdb.WatchSet, sn structs.ServiceName) (uint64, []structs.ServiceName, error) {
	return linkedFromRegistrationTxn(tx, ws, sn, false)
}

// downstreamsFromRegistrationTxn returns the ServiceNames of downstream services based on registrations across instances of the input
func downstreamsFromRegistrationTxn(tx ReadTxn, ws memdb.WatchSet, sn structs.ServiceName) (uint64, []structs.ServiceName, error) {
	return linkedFromRegistrationTxn(tx, ws, sn, true)
}

func linkedFromRegistrationTxn(tx ReadTxn, ws memdb.WatchSet, service structs.ServiceName, downstreams bool) (uint64, []structs.ServiceName, error) {
	// To fetch upstreams we query services that have the input listed as a downstream
	// To fetch downstreams we query services that have the input listed as an upstream
	index := indexDownstream
	if downstreams {
		index = indexUpstream
	}

	iter, err := tx.Get(tableMeshTopology, index, service)
	if err != nil {
		return 0, nil, fmt.Errorf("%q lookup failed: %v", tableMeshTopology, err)
	}
	ws.Add(iter.WatchCh())

	var (
		idx  uint64
		resp []structs.ServiceName
	)
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		entry := raw.(*upstreamDownstream)
		if entry.ModifyIndex > idx {
			idx = entry.ModifyIndex
		}

		linked := entry.Upstream
		if downstreams {
			linked = entry.Downstream
		}
		resp = append(resp, linked)
	}

	// TODO (freddy) This needs a tombstone to avoid the index sliding back on mapping deletion
	//  Using the table index here means that blocking queries will wake up more often than they should
	tableIdx := maxIndexTxn(tx, tableMeshTopology)
	if tableIdx > idx {
		idx = tableIdx
	}
	return idx, resp, nil
}

// updateMeshTopology creates associations between the input service and its upstreams in the topology table
func updateMeshTopology(tx WriteTxn, idx uint64, node string, svc *structs.NodeService, existing interface{}) error {
	// TODO(peering): make this peering aware
	oldUpstreams := make(map[structs.ServiceName]bool)
	if e, ok := existing.(*structs.ServiceNode); ok {
		for _, u := range e.ServiceProxy.Upstreams {
			upstreamMeta := acl.NewEnterpriseMetaWithPartition(e.PartitionOrDefault(), u.DestinationNamespace)
			sn := structs.NewServiceName(u.DestinationName, &upstreamMeta)

			oldUpstreams[sn] = true
		}
	}

	// Despite the name "destination", this service name is downstream of the proxy
	downstream := structs.NewServiceName(svc.Proxy.DestinationServiceName, &svc.EnterpriseMeta)
	inserted := make(map[structs.ServiceName]bool)
	for _, u := range svc.Proxy.Upstreams {
		if u.DestinationType == structs.UpstreamDestTypePreparedQuery {
			continue
		}

		// TODO (freddy): Account for upstream datacenter
		upstreamMeta := acl.NewEnterpriseMetaWithPartition(svc.PartitionOrDefault(), u.DestinationNamespace)
		upstream := structs.NewServiceName(u.DestinationName, &upstreamMeta)

		obj, err := tx.First(tableMeshTopology, indexID, upstream, downstream)
		if err != nil {
			return fmt.Errorf("%q lookup failed: %v", tableMeshTopology, err)
		}
		sid := svc.CompoundServiceID()
		uid := structs.UniqueID(node, sid.String())

		var mapping *upstreamDownstream
		if existing, ok := obj.(*upstreamDownstream); ok {
			mapping := existing.DeepCopy()
			mapping.Refs[uid] = struct{}{}
			mapping.ModifyIndex = idx

			inserted[upstream] = true
		}
		if mapping == nil {
			mapping = &upstreamDownstream{
				Upstream:   upstream,
				Downstream: downstream,
				Refs:       map[string]struct{}{uid: {}},
				RaftIndex: structs.RaftIndex{
					CreateIndex: idx,
					ModifyIndex: idx,
				},
			}
		}
		if err := tx.Insert(tableMeshTopology, mapping); err != nil {
			return fmt.Errorf("failed inserting %s mapping: %s", tableMeshTopology, err)
		}
		if err := indexUpdateMaxTxn(tx, idx, tableMeshTopology); err != nil {
			return fmt.Errorf("failed updating %s index: %v", tableMeshTopology, err)
		}
		inserted[upstream] = true
	}

	for u := range oldUpstreams {
		if !inserted[u] {
			if _, err := tx.DeleteAll(tableMeshTopology, indexID, u, downstream); err != nil {
				return fmt.Errorf("failed to truncate %s table: %v", tableMeshTopology, err)
			}
			if err := indexUpdateMaxTxn(tx, idx, tableMeshTopology); err != nil {
				return fmt.Errorf("failed updating %s index: %v", tableMeshTopology, err)
			}
		}
	}
	return nil
}

// cleanupMeshTopology removes a service from the mesh topology table
// This is only safe to call when there are no more known instances of this proxy
func cleanupMeshTopology(tx WriteTxn, idx uint64, service *structs.ServiceNode) error {
	if service.PeerName != "" {
		return nil
	}

	if service.ServiceKind != structs.ServiceKindConnectProxy {
		return nil
	}
	sn := structs.NewServiceName(service.ServiceProxy.DestinationServiceName, &service.EnterpriseMeta)

	sid := service.CompoundServiceID()
	uid := structs.UniqueID(service.Node, sid.String())

	iter, err := tx.Get(tableMeshTopology, indexDownstream, sn)
	if err != nil {
		return fmt.Errorf("%q lookup failed: %v", tableMeshTopology, err)
	}

	mappings := make([]*upstreamDownstream, 0)
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		mappings = append(mappings, raw.(*upstreamDownstream))
	}

	// Do the updates in a separate loop so we don't trash the iterator.
	for _, m := range mappings {
		copy := m.DeepCopy()

		// Bail early if there's no reference to the proxy ID we're deleting
		if _, ok := copy.Refs[uid]; !ok {
			continue
		}

		delete(copy.Refs, uid)
		if len(copy.Refs) == 0 {
			if err := tx.Delete(tableMeshTopology, m); err != nil {
				return fmt.Errorf("failed to truncate %s table: %v", tableMeshTopology, err)
			}
			if err := indexUpdateMaxTxn(tx, idx, tableMeshTopology); err != nil {
				return fmt.Errorf("failed updating %s index: %v", tableMeshTopology, err)
			}
			continue

		}
		if err := tx.Insert(tableMeshTopology, copy); err != nil {
			return fmt.Errorf("failed inserting %s mapping: %s", tableMeshTopology, err)
		}
	}
	return nil
}

func insertGatewayServiceTopologyMapping(tx WriteTxn, idx uint64, gs *structs.GatewayService) error {
	// Only ingress gateways are standalone items in the mesh topology viz
	if gs.GatewayKind != structs.ServiceKindIngressGateway || gs.Service.Name == structs.WildcardSpecifier {
		return nil
	}

	mapping := upstreamDownstream{
		Upstream:   gs.Service,
		Downstream: gs.Gateway,
		Refs:       make(map[string]struct{}),
		RaftIndex:  gs.RaftIndex,
	}
	if err := tx.Insert(tableMeshTopology, &mapping); err != nil {
		return fmt.Errorf("failed inserting %s mapping: %s", tableMeshTopology, err)
	}
	if err := indexUpdateMaxTxn(tx, idx, tableMeshTopology); err != nil {
		return fmt.Errorf("failed updating %s index: %v", tableMeshTopology, err)
	}

	return nil
}

func deleteGatewayServiceTopologyMapping(tx WriteTxn, idx uint64, gs *structs.GatewayService) error {
	// Only ingress gateways are standalone items in the mesh topology viz
	if gs.GatewayKind != structs.ServiceKindIngressGateway {
		return nil
	}

	if _, err := tx.DeleteAll(tableMeshTopology, indexID, gs.Service, gs.Gateway); err != nil {
		return fmt.Errorf("failed to truncate %s table: %v", tableMeshTopology, err)
	}
	if err := indexUpdateMaxTxn(tx, idx, tableMeshTopology); err != nil {
		return fmt.Errorf("failed updating %s index: %v", tableMeshTopology, err)
	}

	return nil
}

func truncateGatewayServiceTopologyMappings(tx WriteTxn, idx uint64, gateway structs.ServiceName, kind string) error {
	// Only ingress gateways are standalone items in the mesh topology viz
	if kind != string(structs.ServiceKindIngressGateway) {
		return nil
	}

	if _, err := tx.DeleteAll(tableMeshTopology, indexDownstream, gateway); err != nil {
		return fmt.Errorf("failed to truncate %s table: %v", tableMeshTopology, err)
	}
	if err := indexUpdateMaxTxn(tx, idx, tableMeshTopology); err != nil {
		return fmt.Errorf("failed updating %s index: %v", tableMeshTopology, err)
	}

	return nil
}

func upsertKindServiceName(tx WriteTxn, idx uint64, kind structs.ServiceKind, name structs.ServiceName) error {
	// TODO(peering): make this peering aware
	q := KindServiceNameQuery{Name: name.Name, Kind: kind, EnterpriseMeta: name.EnterpriseMeta}
	existing, err := tx.First(tableKindServiceNames, indexID, q)
	if err != nil {
		return err
	}

	// Service name is already known. Nothing to do.
	if existing != nil {
		return nil
	}

	ksn := KindServiceName{
		Kind:    kind,
		Service: name,
		RaftIndex: structs.RaftIndex{
			CreateIndex: idx,
			ModifyIndex: idx,
		},
	}
	if err := tx.Insert(tableKindServiceNames, &ksn); err != nil {
		return fmt.Errorf("failed inserting %s/%s into %s: %s", kind, name.String(), tableKindServiceNames, err)
	}
	return updateKindServiceNamesIndex(tx, idx, kind, name.EnterpriseMeta)
}

func cleanupKindServiceName(tx WriteTxn, idx uint64, name structs.ServiceName, kind structs.ServiceKind) error {
	q := KindServiceNameQuery{Name: name.Name, Kind: kind, EnterpriseMeta: name.EnterpriseMeta}
	if _, err := tx.DeleteAll(tableKindServiceNames, indexID, q); err != nil {
		return fmt.Errorf("failed to delete %s from %s: %s", name, tableKindServiceNames, err)
	}
	return updateKindServiceNamesIndex(tx, idx, kind, name.EnterpriseMeta)
}

// CatalogDump returns all the contents of the node, service and check tables.
// In Enterprise, this will return entries across all partitions and namespaces.
// TODO(peering) make this peering aware?
func (s *Store) CatalogDump() (*structs.CatalogContents, error) {
	tx := s.db.Txn(false)
	contents := &structs.CatalogContents{}

	nodes, err := tx.Get(tableNodes, indexID)
	if err != nil {
		return nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		n := node.(*structs.Node)
		contents.Nodes = append(contents.Nodes, n)
	}

	services, err := tx.Get(tableServices, indexID)
	if err != nil {
		return nil, fmt.Errorf("failed services lookup: %s", err)
	}
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		contents.Services = append(contents.Services, svc)
	}

	checks, err := tx.Get(tableChecks, indexID)
	if err != nil {
		return nil, fmt.Errorf("failed checks lookup: %s", err)
	}
	for check := checks.Next(); check != nil; check = checks.Next() {
		c := check.(*structs.HealthCheck)
		contents.Checks = append(contents.Checks, c)
	}

	return contents, nil
}
