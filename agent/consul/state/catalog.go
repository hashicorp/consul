package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-memdb"
	uuid "github.com/hashicorp/go-uuid"
)

const (
	servicesTableName = "services"

	// serviceLastExtinctionIndexName keeps track of the last raft index when the last instance
	// of any service was unregistered. This is used by blocking queries on missing services.
	serviceLastExtinctionIndexName = "service_last_extinction"
)

// nodesTableSchema returns a new table schema used for storing node
// information.
func nodesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "nodes",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Node",
					Lowercase: true,
				},
			},
			"uuid": &memdb.IndexSchema{
				Name:         "uuid",
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			"meta": &memdb.IndexSchema{
				Name:         "meta",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringMapFieldIndex{
					Field:     "Meta",
					Lowercase: false,
				},
			},
		},
	}
}

// servicesTableSchema returns a new table schema used to store information
// about services.
func servicesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "services",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "ServiceID",
							Lowercase: true,
						},
					},
				},
			},
			"node": &memdb.IndexSchema{
				Name:         "node",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Node",
					Lowercase: true,
				},
			},
			"service": &memdb.IndexSchema{
				Name:         "service",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ServiceName",
					Lowercase: true,
				},
			},
			"connect": &memdb.IndexSchema{
				Name:         "connect",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &IndexConnectService{},
			},
			"kind": &memdb.IndexSchema{
				Name:         "kind",
				AllowMissing: false,
				Unique:       false,
				Indexer:      &IndexServiceKind{},
			},
		},
	}
}

// checksTableSchema returns a new table schema used for storing and indexing
// health check information. Health checks have a number of different attributes
// we want to filter by, so this table is a bit more complex.
func checksTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "checks",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "CheckID",
							Lowercase: true,
						},
					},
				},
			},
			"status": &memdb.IndexSchema{
				Name:         "status",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Status",
					Lowercase: false,
				},
			},
			"service": &memdb.IndexSchema{
				Name:         "service",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ServiceName",
					Lowercase: true,
				},
			},
			"node": &memdb.IndexSchema{
				Name:         "node",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Node",
					Lowercase: true,
				},
			},
			"node_service_check": &memdb.IndexSchema{
				Name:         "node_service_check",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.FieldSetIndex{
							Field: "ServiceID",
						},
					},
				},
			},
			"node_service": &memdb.IndexSchema{
				Name:         "node_service",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "ServiceID",
							Lowercase: true,
						},
					},
				},
			},
		},
	}
}

func init() {
	registerSchema(nodesTableSchema)
	registerSchema(servicesTableSchema)
	registerSchema(checksTableSchema)
}

const (
	// minUUIDLookupLen is used as a minimum length of a node name required before
	// we test to see if the name is actually a UUID and perform an ID-based node
	// lookup.
	minUUIDLookupLen = 2
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
	iter, err := s.tx.Get("nodes", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Services is used to pull the full list of services for a given node for use
// during snapshots.
func (s *Snapshot) Services(node string) (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("services", "node", node)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Checks is used to pull the full list of checks for a given node for use
// during snapshots.
func (s *Snapshot) Checks(node string) (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("checks", "node", node)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Registration is used to make sure a node, service, and check registration is
// performed within a single transaction to avoid race conditions on state
// updates.
func (s *Restore) Registration(idx uint64, req *structs.RegisterRequest) error {
	if err := s.store.ensureRegistrationTxn(s.tx, idx, req); err != nil {
		return err
	}
	return nil
}

// EnsureRegistration is used to make sure a node, service, and check
// registration is performed within a single transaction to avoid race
// conditions on state updates.
func (s *Store) EnsureRegistration(idx uint64, req *structs.RegisterRequest) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.ensureRegistrationTxn(tx, idx, req); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (s *Store) ensureCheckIfNodeMatches(tx *memdb.Txn, idx uint64, node string, check *structs.HealthCheck) error {
	if check.Node != node {
		return fmt.Errorf("check node %q does not match node %q",
			check.Node, node)
	}
	if err := s.ensureCheckTxn(tx, idx, check); err != nil {
		return fmt.Errorf("failed inserting check: %s on node %q", err, check.Node)
	}
	return nil
}

// ensureRegistrationTxn is used to make sure a node, service, and check
// registration is performed within a single transaction to avoid race
// conditions on state updates.
func (s *Store) ensureRegistrationTxn(tx *memdb.Txn, idx uint64, req *structs.RegisterRequest) error {
	// Create a node structure.
	node := &structs.Node{
		ID:              req.ID,
		Node:            req.Node,
		Address:         req.Address,
		Datacenter:      req.Datacenter,
		TaggedAddresses: req.TaggedAddresses,
		Meta:            req.NodeMeta,
	}

	// Since this gets called for all node operations (service and check
	// updates) and churn on the node itself is basically none after the
	// node updates itself the first time, it's worth seeing if we need to
	// modify the node at all so we prevent watch churn and useless writes
	// and modify index bumps on the node.
	{
		existing, err := tx.First("nodes", "id", node.Node)
		if err != nil {
			return fmt.Errorf("node lookup failed: %s", err)
		}
		if existing == nil || req.ChangesNode(existing.(*structs.Node)) {
			if err := s.ensureNodeTxn(tx, idx, node); err != nil {
				return fmt.Errorf("failed inserting node: %s", err)
			}
		}
	}

	// Add the service, if any. We perform a similar check as we do for the
	// node info above to make sure we actually need to update the service
	// definition in order to prevent useless churn if nothing has changed.
	if req.Service != nil {
		existing, err := tx.First("services", "id", req.Node, req.Service.ID)
		if err != nil {
			return fmt.Errorf("failed service lookup: %s", err)
		}
		if existing == nil || !(existing.(*structs.ServiceNode).ToNodeService()).IsSame(req.Service) {
			if err := s.ensureServiceTxn(tx, idx, req.Node, req.Service); err != nil {
				return fmt.Errorf("failed inserting service: %s", err)

			}
		}
	}

	// Add the checks, if any.
	if req.Check != nil {
		if err := s.ensureCheckIfNodeMatches(tx, idx, req.Node, req.Check); err != nil {
			return err
		}
	}
	for _, check := range req.Checks {
		if err := s.ensureCheckIfNodeMatches(tx, idx, req.Node, check); err != nil {
			return err
		}
	}

	return nil
}

// EnsureNode is used to upsert node registration or modification.
func (s *Store) EnsureNode(idx uint64, node *structs.Node) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the node upsert
	if err := s.ensureNodeTxn(tx, idx, node); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// ensureNoNodeWithSimilarNameTxn checks that no other node has conflict in its name
// If allowClashWithoutID then, getting a conflict on another node without ID will be allowed
func (s *Store) ensureNoNodeWithSimilarNameTxn(tx *memdb.Txn, node *structs.Node, allowClashWithoutID bool) error {
	// Retrieve all of the nodes
	enodes, err := tx.Get("nodes", "id")
	if err != nil {
		return fmt.Errorf("Cannot lookup all nodes: %s", err)
	}
	for nodeIt := enodes.Next(); nodeIt != nil; nodeIt = enodes.Next() {
		enode := nodeIt.(*structs.Node)
		if strings.EqualFold(node.Node, enode.Node) && node.ID != enode.ID {
			// Look up the existing node's Serf health check to see if it's failed.
			// If it is, the node can be renamed.
			enodeCheck, err := tx.First("checks", "id", enode.Node, string(structs.SerfCheckID))
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
func (s *Store) ensureNodeCASTxn(tx *memdb.Txn, idx uint64, node *structs.Node) (bool, error) {
	// Retrieve the existing entry.
	existing, err := getNodeTxn(tx, node.Node)
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
	if err := s.ensureNodeTxn(tx, idx, node); err != nil {
		return false, err
	}

	return true, nil
}

// ensureNodeTxn is the inner function called to actually create a node
// registration or modify an existing one in the state store. It allows
// passing in a memdb transaction so it may be part of a larger txn.
func (s *Store) ensureNodeTxn(tx *memdb.Txn, idx uint64, node *structs.Node) error {
	// See if there's an existing node with this UUID, and make sure the
	// name is the same.
	var n *structs.Node
	if node.ID != "" {
		existing, err := getNodeIDTxn(tx, node.ID)
		if err != nil {
			return fmt.Errorf("node lookup failed: %s", err)
		}
		if existing != nil {
			n = existing
			if n.Node != node.Node {
				// Lets first get all nodes and check whether name do match, we do not allow clash on nodes without ID
				dupNameError := s.ensureNoNodeWithSimilarNameTxn(tx, node, false)
				if dupNameError != nil {
					return fmt.Errorf("Error while renaming Node ID: %q (%s): %s", node.ID, node.Address, dupNameError)
				}
				// We are actually renaming a node, remove its reference first
				err := s.deleteNodeTxn(tx, idx, n.Node)
				if err != nil {
					return fmt.Errorf("Error while renaming Node ID: %q (%s) from %s to %s",
						node.ID, node.Address, n.Node, node.Node)
				}
			}
		} else {
			// We allow to "steal" another node name that would have no ID
			// It basically means that we allow upgrading a node without ID and add the ID
			dupNameError := s.ensureNoNodeWithSimilarNameTxn(tx, node, true)
			if dupNameError != nil {
				return fmt.Errorf("Error while renaming Node ID: %q: %s", node.ID, dupNameError)
			}
		}
	}
	// TODO: else Node.ID == "" should be forbidden in future Consul releases
	// See https://github.com/hashicorp/consul/pull/3983 for context

	// Check for an existing node by name to support nodes with no IDs.
	if n == nil {
		existing, err := tx.First("nodes", "id", node.Node)
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
	} else {
		node.CreateIndex = idx
		node.ModifyIndex = idx
	}

	// Insert the node and update the index.
	if err := tx.Insert("nodes", node); err != nil {
		return fmt.Errorf("failed inserting node: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"nodes", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	// Update the node's service indexes as the node information is included
	// in health queries and we would otherwise miss node updates in some cases
	// for those queries.
	if err := s.updateAllServiceIndexesOfNode(tx, idx, node.Node); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// GetNode is used to retrieve a node registration by node name ID.
func (s *Store) GetNode(id string) (uint64, *structs.Node, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "nodes")

	// Retrieve the node from the state store
	node, err := getNodeTxn(tx, id)
	if err != nil {
		return 0, nil, fmt.Errorf("node lookup failed: %s", err)
	}
	return idx, node, nil
}

func getNodeTxn(tx *memdb.Txn, nodeName string) (*structs.Node, error) {
	node, err := tx.First("nodes", "id", nodeName)
	if err != nil {
		return nil, fmt.Errorf("node lookup failed: %s", err)
	}
	if node != nil {
		return node.(*structs.Node), nil
	}
	return nil, nil
}

func getNodeIDTxn(tx *memdb.Txn, id types.NodeID) (*structs.Node, error) {
	strnode := string(id)
	uuidValue, err := uuid.ParseUUID(strnode)
	if err != nil {
		return nil, fmt.Errorf("node lookup by ID failed, wrong UUID: %v for '%s'", err, strnode)
	}

	node, err := tx.First("nodes", "uuid", uuidValue)
	if err != nil {
		return nil, fmt.Errorf("node lookup by ID failed: %s", err)
	}
	if node != nil {
		return node.(*structs.Node), nil
	}
	return nil, nil
}

// GetNodeID is used to retrieve a node registration by node ID.
func (s *Store) GetNodeID(id types.NodeID) (uint64, *structs.Node, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "nodes")

	// Retrieve the node from the state store
	node, err := getNodeIDTxn(tx, id)
	return idx, node, err
}

// Nodes is used to return all of the known nodes.
func (s *Store) Nodes(ws memdb.WatchSet) (uint64, structs.Nodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "nodes")

	// Retrieve all of the nodes
	nodes, err := tx.Get("nodes", "id")
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
func (s *Store) NodesByMeta(ws memdb.WatchSet, filters map[string]string) (uint64, structs.Nodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "nodes")

	// Retrieve all of the nodes
	var args []interface{}
	for key, value := range filters {
		args = append(args, key, value)
		break
	}
	nodes, err := tx.Get("nodes", "meta", args...)
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
func (s *Store) DeleteNode(idx uint64, nodeName string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the node deletion.
	if err := s.deleteNodeTxn(tx, idx, nodeName); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// deleteNodeCASTxn is used to try doing a node delete operation with a given
// raft index. If the CAS index specified is not equal to the last observed index for
// the given check, then the call is a noop, otherwise a normal check delete is invoked.
func (s *Store) deleteNodeCASTxn(tx *memdb.Txn, idx, cidx uint64, nodeName string) (bool, error) {
	// Look up the node.
	node, err := getNodeTxn(tx, nodeName)
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
	if err := s.deleteNodeTxn(tx, idx, nodeName); err != nil {
		return false, err
	}

	return true, nil
}

// deleteNodeTxn is the inner method used for removing a node from
// the store within a given transaction.
func (s *Store) deleteNodeTxn(tx *memdb.Txn, idx uint64, nodeName string) error {
	// Look up the node.
	node, err := tx.First("nodes", "id", nodeName)
	if err != nil {
		return fmt.Errorf("node lookup failed: %s", err)
	}
	if node == nil {
		return nil
	}

	// Delete all services associated with the node and update the service index.
	services, err := tx.Get("services", "node", nodeName)
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}
	var sids []string
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		sids = append(sids, svc.ServiceID)
		if err := tx.Insert("index", &IndexEntry{serviceIndexName(svc.ServiceName), idx}); err != nil {
			return fmt.Errorf("failed updating index: %s", err)
		}
		if err := tx.Insert("index", &IndexEntry{serviceKindIndexName(svc.ServiceKind), idx}); err != nil {
			return fmt.Errorf("failed updating index: %s", err)
		}
	}

	// Do the delete in a separate loop so we don't trash the iterator.
	for _, sid := range sids {
		if err := s.deleteServiceTxn(tx, idx, nodeName, sid); err != nil {
			return err
		}
	}

	// Delete all checks associated with the node. This will invalidate
	// sessions as necessary.
	checks, err := tx.Get("checks", "node", nodeName)
	if err != nil {
		return fmt.Errorf("failed check lookup: %s", err)
	}
	var cids []types.CheckID
	for check := checks.Next(); check != nil; check = checks.Next() {
		cids = append(cids, check.(*structs.HealthCheck).CheckID)
	}

	// Do the delete in a separate loop so we don't trash the iterator.
	for _, cid := range cids {
		if err := s.deleteCheckTxn(tx, idx, nodeName, cid); err != nil {
			return err
		}
	}

	// Delete any coordinates associated with this node.
	coords, err := tx.Get("coordinates", "node", nodeName)
	if err != nil {
		return fmt.Errorf("failed coordinate lookup: %s", err)
	}
	for coord := coords.Next(); coord != nil; coord = coords.Next() {
		if err := tx.Delete("coordinates", coord); err != nil {
			return fmt.Errorf("failed deleting coordinate: %s", err)
		}
		if err := tx.Insert("index", &IndexEntry{"coordinates", idx}); err != nil {
			return fmt.Errorf("failed updating index: %s", err)
		}
	}

	// Delete the node and update the index.
	if err := tx.Delete("nodes", node); err != nil {
		return fmt.Errorf("failed deleting node: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"nodes", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// Invalidate any sessions for this node.
	sessions, err := tx.Get("sessions", "node", nodeName)
	if err != nil {
		return fmt.Errorf("failed session lookup: %s", err)
	}
	var ids []string
	for sess := sessions.Next(); sess != nil; sess = sessions.Next() {
		ids = append(ids, sess.(*structs.Session).ID)
	}

	// Do the delete in a separate loop so we don't trash the iterator.
	for _, id := range ids {
		if err := s.deleteSessionTxn(tx, idx, id); err != nil {
			return fmt.Errorf("failed session delete: %s", err)
		}
	}

	return nil
}

// EnsureService is called to upsert creation of a given NodeService.
func (s *Store) EnsureService(idx uint64, node string, svc *structs.NodeService) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the service registration upsert
	if err := s.ensureServiceTxn(tx, idx, node, svc); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// ensureServiceCASTxn updates a service only if the existing index matches the given index.
// Returns a bool indicating if a write happened and any error.
func (s *Store) ensureServiceCASTxn(tx *memdb.Txn, idx uint64, node string, svc *structs.NodeService) (bool, error) {
	// Retrieve the existing service.
	existing, err := tx.First("services", "id", node, svc.ID)
	if err != nil {
		return false, fmt.Errorf("failed service lookup: %s", err)
	}

	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	if svc.ModifyIndex == 0 && existing != nil {
		return false, nil
	}
	if svc.ModifyIndex != 0 && existing == nil {
		return false, nil
	}
	e, ok := existing.(*structs.ServiceNode)
	if ok && svc.ModifyIndex != 0 && svc.ModifyIndex != e.ModifyIndex {
		return false, nil
	}

	// Perform the update.
	if err := s.ensureServiceTxn(tx, idx, node, svc); err != nil {
		return false, err
	}

	return true, nil
}

// ensureServiceTxn is used to upsert a service registration within an
// existing memdb transaction.
func (s *Store) ensureServiceTxn(tx *memdb.Txn, idx uint64, node string, svc *structs.NodeService) error {
	// Check for existing service
	existing, err := tx.First("services", "id", node, svc.ID)
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}

	if err = structs.ValidateMetadata(svc.Meta, false); err != nil {
		return fmt.Errorf("Invalid Service Meta for node %s and serviceID %s: %v", node, svc.ID, err)
	}
	// Create the service node entry and populate the indexes. Note that
	// conversion doesn't populate any of the node-specific information.
	// That's always populated when we read from the state store.
	entry := svc.ToServiceNode(node)
	// Get the node
	n, err := tx.First("nodes", "id", node)
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
	} else {
		entry.CreateIndex = idx
	}
	entry.ModifyIndex = idx

	// Insert the service and update the index
	if err := tx.Insert("services", entry); err != nil {
		return fmt.Errorf("failed inserting service: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"services", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{serviceIndexName(svc.Service), idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{serviceKindIndexName(svc.Kind), idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// Services returns all services along with a list of associated tags.
func (s *Store) Services(ws memdb.WatchSet) (uint64, structs.Services, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "services")

	// List all the services.
	services, err := tx.Get("services", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying services: %s", err)
	}
	ws.Add(services.WatchCh())

	// Rip through the services and enumerate them and their unique set of
	// tags.
	unique := make(map[string]map[string]struct{})
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		tags, ok := unique[svc.ServiceName]
		if !ok {
			unique[svc.ServiceName] = make(map[string]struct{})
			tags = unique[svc.ServiceName]
		}
		for _, tag := range svc.ServiceTags {
			tags[tag] = struct{}{}
		}
	}

	// Generate the output structure.
	var results = make(structs.Services)
	for service, tags := range unique {
		results[service] = make([]string, 0)
		for tag := range tags {
			results[service] = append(results[service], tag)
		}
	}
	return idx, results, nil
}

// ServicesByNodeMeta returns all services, filtered by the given node metadata.
func (s *Store) ServicesByNodeMeta(ws memdb.WatchSet, filters map[string]string) (uint64, structs.Services, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "services", "nodes")

	// Retrieve all of the nodes with the meta k/v pair
	var args []interface{}
	for key, value := range filters {
		args = append(args, key, value)
		break
	}
	nodes, err := tx.Get("nodes", "meta", args...)
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	ws.Add(nodes.WatchCh())

	// We don't want to track an unlimited number of services, so we pull a
	// top-level watch to use as a fallback.
	allServices, err := tx.Get("services", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed services lookup: %s", err)
	}
	allServicesCh := allServices.WatchCh()

	// Populate the services map
	unique := make(map[string]map[string]struct{})
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		n := node.(*structs.Node)
		if len(filters) > 1 && !structs.SatisfiesMetaFilters(n.Meta, filters) {
			continue
		}

		// List all the services on the node
		services, err := tx.Get("services", "node", n.Node)
		if err != nil {
			return 0, nil, fmt.Errorf("failed querying services: %s", err)
		}
		ws.AddWithLimit(watchLimit, services.WatchCh(), allServicesCh)

		// Rip through the services and enumerate them and their unique set of
		// tags.
		for service := services.Next(); service != nil; service = services.Next() {
			svc := service.(*structs.ServiceNode)
			tags, ok := unique[svc.ServiceName]
			if !ok {
				unique[svc.ServiceName] = make(map[string]struct{})
				tags = unique[svc.ServiceName]
			}
			for _, tag := range svc.ServiceTags {
				tags[tag] = struct{}{}
			}
		}
	}

	// Generate the output structure.
	var results = make(structs.Services)
	for service, tags := range unique {
		results[service] = make([]string, 0)
		for tag := range tags {
			results[service] = append(results[service], tag)
		}
	}
	return idx, results, nil
}

// maxIndexForService return the maximum Raft Index for a service
// If the index is not set for the service, it will return the missing
// service index.
// The service_last_extinction is set to the last raft index when a service
// was unregistered (or 0 if no services were ever unregistered). This
// allows blocking queries to
//   * return when the last instance of a service is removed
//   * block until an instance for this service is available, or another
//     service is unregistered.
func maxIndexForService(tx *memdb.Txn, serviceName string, serviceExists, checks bool) uint64 {
	idx, _ := maxIndexAndWatchChForService(tx, serviceName, serviceExists, checks)
	return idx
}

// maxIndexAndWatchChForService return the maximum Raft Index for a service. If
// the index is not set for the service, it will return the missing service
// index. The service_last_extinction is set to the last raft index when a
// service was unregistered (or 0 if no services were ever unregistered). This
// allows blocking queries to
//   * return when the last instance of a service is removed
//   * block until an instance for this service is available, or another
//     service is unregistered.
//
// It also _may_ return a watch chan to add to a WatchSet. It will only return
// one if the service exists, and has a service index. If it doesn't then nil is
// returned for the chan. This allows for blocking watchers to _only_ watch this
// one chan in the common case, falling back to watching all touched MemDB
// indexes in more complicated cases.
func maxIndexAndWatchChForService(tx *memdb.Txn, serviceName string, serviceExists, checks bool) (uint64, <-chan struct{}) {
	if !serviceExists {
		res, err := tx.First("index", "id", serviceLastExtinctionIndexName)
		if missingIdx, ok := res.(*IndexEntry); ok && err == nil {
			// Not safe to only watch the extinction index as it's not updated when
			// new instances come along so return nil watchCh.
			return missingIdx.Value, nil
		}
	}

	ch, res, err := tx.FirstWatch("index", "id", serviceIndexName(serviceName))
	if idx, ok := res.(*IndexEntry); ok && err == nil {
		return idx.Value, ch
	}
	if checks {
		return maxIndexTxn(tx, "nodes", "services", "checks"), nil
	}

	return maxIndexTxn(tx, "nodes", "services"), nil
}

// ConnectServiceNodes returns the nodes associated with a Connect
// compatible destination for the given service name. This will include
// both proxies and native integrations.
func (s *Store) ConnectServiceNodes(ws memdb.WatchSet, serviceName string) (uint64, structs.ServiceNodes, error) {
	return s.serviceNodes(ws, serviceName, true)
}

// ServiceNodes returns the nodes associated with a given service name.
func (s *Store) ServiceNodes(ws memdb.WatchSet, serviceName string) (uint64, structs.ServiceNodes, error) {
	return s.serviceNodes(ws, serviceName, false)
}

func (s *Store) serviceNodes(ws memdb.WatchSet, serviceName string, connect bool) (uint64, structs.ServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Function for lookup
	var f func() (memdb.ResultIterator, error)
	if !connect {
		f = func() (memdb.ResultIterator, error) {
			return tx.Get("services", "service", serviceName)
		}
	} else {
		f = func() (memdb.ResultIterator, error) {
			return tx.Get("services", "connect", serviceName)
		}
	}

	// List all the services.
	services, err := f()
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}
	ws.Add(services.WatchCh())

	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		results = append(results, service.(*structs.ServiceNode))
	}

	// Fill in the node details.
	results, err = s.parseServiceNodes(tx, ws, results)
	if err != nil {
		return 0, nil, fmt.Errorf("failed parsing service nodes: %s", err)
	}

	// Get the table index.
	idx := maxIndexForService(tx, serviceName, len(results) > 0, false)

	return idx, results, nil
}

// ServiceTagNodes returns the nodes associated with a given service, filtering
// out services that don't contain the given tags.
func (s *Store) ServiceTagNodes(ws memdb.WatchSet, service string, tags []string) (uint64, structs.ServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// List all the services.
	services, err := tx.Get("services", "service", service)
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
	results, err = s.parseServiceNodes(tx, ws, results)
	if err != nil {
		return 0, nil, fmt.Errorf("failed parsing service nodes: %s", err)
	}
	// Get the table index.
	idx := maxIndexForService(tx, service, serviceExists, false)

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
func (s *Store) ServiceAddressNodes(ws memdb.WatchSet, address string) (uint64, structs.ServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// List all the services.
	services, err := tx.Get("services", "id")
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
	results, err = s.parseServiceNodes(tx, ws, results)
	if err != nil {
		return 0, nil, fmt.Errorf("failed parsing service nodes: %s", err)
	}
	return 0, results, nil
}

// parseServiceNodes iterates over a services query and fills in the node details,
// returning a ServiceNodes slice.
func (s *Store) parseServiceNodes(tx *memdb.Txn, ws memdb.WatchSet, services structs.ServiceNodes) (structs.ServiceNodes, error) {
	// We don't want to track an unlimited number of nodes, so we pull a
	// top-level watch to use as a fallback.
	allNodes, err := tx.Get("nodes", "id")
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
		watchCh, n, err := tx.FirstWatch("nodes", "id", sn.Node)
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
		s.NodeMeta = node.Meta

		results = append(results, s)
	}
	return results, nil
}

// NodeService is used to retrieve a specific service associated with the given
// node.
func (s *Store) NodeService(nodeName string, serviceID string) (uint64, *structs.NodeService, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "services")

	// Query the service
	service, err := s.getNodeServiceTxn(tx, nodeName, serviceID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying service for node %q: %s", nodeName, err)
	}

	return idx, service, nil
}

func (s *Store) getNodeServiceTxn(tx *memdb.Txn, nodeName, serviceID string) (*structs.NodeService, error) {
	// Query the service
	service, err := tx.First("services", "id", nodeName, serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed querying service for node %q: %s", nodeName, err)
	}

	if service != nil {
		return service.(*structs.ServiceNode).ToNodeService(), nil
	}

	return nil, nil
}

// NodeServices is used to query service registrations by node name or UUID.
func (s *Store) NodeServices(ws memdb.WatchSet, nodeNameOrID string) (uint64, *structs.NodeServices, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "nodes", "services")

	// Query the node by node name
	watchCh, n, err := tx.FirstWatch("nodes", "id", nodeNameOrID)
	if err != nil {
		return 0, nil, fmt.Errorf("node lookup failed: %s", err)
	}

	if n != nil {
		ws.Add(watchCh)
	} else {
		if len(nodeNameOrID) < minUUIDLookupLen {
			ws.Add(watchCh)
			return 0, nil, nil
		}

		// Attempt to lookup the node by its node ID
		iter, err := tx.Get("nodes", "uuid_prefix", resizeNodeLookupKey(nodeNameOrID))
		if err != nil {
			ws.Add(watchCh)
			// TODO(sean@): We could/should log an error re: the uuid_prefix lookup
			// failing once a logger has been introduced to the catalog.
			return 0, nil, nil
		}

		n = iter.Next()
		if n == nil {
			// No nodes matched, even with the Node ID: add a watch on the node name.
			ws.Add(watchCh)
			return 0, nil, nil
		}

		idWatchCh := iter.WatchCh()
		if iter.Next() != nil {
			// More than one match present: Watch on the node name channel and return
			// an empty result (node lookups can not be ambiguous).
			ws.Add(watchCh)
			return 0, nil, nil
		}

		ws.Add(idWatchCh)
	}

	node := n.(*structs.Node)
	nodeName := node.Node

	// Read all of the services
	services, err := tx.Get("services", "node", nodeName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying services for node %q: %s", nodeName, err)
	}
	ws.Add(services.WatchCh())

	// Initialize the node services struct
	ns := &structs.NodeServices{
		Node:     node,
		Services: make(map[string]*structs.NodeService),
	}

	// Add all of the services to the map.
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode).ToNodeService()
		ns.Services[svc.ID] = svc
	}

	return idx, ns, nil
}

// DeleteService is used to delete a given service associated with a node.
func (s *Store) DeleteService(idx uint64, nodeName, serviceID string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the service deletion
	if err := s.deleteServiceTxn(tx, idx, nodeName, serviceID); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func serviceIndexName(name string) string {
	return fmt.Sprintf("service.%s", name)
}

func serviceKindIndexName(kind structs.ServiceKind) string {
	switch kind {
	case structs.ServiceKindTypical:
		// needs a special case here
		return "service_kind.typical"
	default:
		return "service_kind." + string(kind)
	}
}

// deleteServiceCASTxn is used to try doing a service delete operation with a given
// raft index. If the CAS index specified is not equal to the last observed index for
// the given service, then the call is a noop, otherwise a normal delete is invoked.
func (s *Store) deleteServiceCASTxn(tx *memdb.Txn, idx, cidx uint64, nodeName, serviceID string) (bool, error) {
	// Look up the service.
	service, err := s.getNodeServiceTxn(tx, nodeName, serviceID)
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
	if err := s.deleteServiceTxn(tx, idx, nodeName, serviceID); err != nil {
		return false, err
	}

	return true, nil
}

// deleteServiceTxn is the inner method called to remove a service
// registration within an existing transaction.
func (s *Store) deleteServiceTxn(tx *memdb.Txn, idx uint64, nodeName, serviceID string) error {
	// Look up the service.
	service, err := tx.First("services", "id", nodeName, serviceID)
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}
	if service == nil {
		return nil
	}

	// Delete any checks associated with the service. This will invalidate
	// sessions as necessary.
	checks, err := tx.Get("checks", "node_service", nodeName, serviceID)
	if err != nil {
		return fmt.Errorf("failed service check lookup: %s", err)
	}
	var cids []types.CheckID
	for check := checks.Next(); check != nil; check = checks.Next() {
		cids = append(cids, check.(*structs.HealthCheck).CheckID)
	}

	// Do the delete in a separate loop so we don't trash the iterator.
	for _, cid := range cids {
		if err := s.deleteCheckTxn(tx, idx, nodeName, cid); err != nil {
			return err
		}
	}

	// Update the index.
	if err := tx.Insert("index", &IndexEntry{"checks", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// Delete the service and update the index
	if err := tx.Delete("services", service); err != nil {
		return fmt.Errorf("failed deleting service: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"services", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	svc := service.(*structs.ServiceNode)
	if err := tx.Insert("index", &IndexEntry{serviceKindIndexName(svc.ServiceKind), idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	if remainingService, err := tx.First("services", "service", svc.ServiceName); err == nil {
		if remainingService != nil {
			// We have at least one remaining service, update the index
			if err := tx.Insert("index", &IndexEntry{serviceIndexName(svc.ServiceName), idx}); err != nil {
				return fmt.Errorf("failed updating index: %s", err)
			}
		} else {
			// There are no more service instances, cleanup the service.<serviceName> index
			serviceIndex, err := tx.First("index", "id", serviceIndexName(svc.ServiceName))
			if err == nil && serviceIndex != nil {
				// we found service.<serviceName> index, garbage collect it
				if errW := tx.Delete("index", serviceIndex); errW != nil {
					return fmt.Errorf("[FAILED] deleting serviceIndex %s: %s", svc.ServiceName, err)
				}
			}

			if err := tx.Insert("index", &IndexEntry{serviceLastExtinctionIndexName, idx}); err != nil {
				return fmt.Errorf("failed updating missing service index: %s", err)
			}

		}
	} else {
		return fmt.Errorf("Could not find any service %s: %s", svc.ServiceName, err)
	}
	return nil
}

// EnsureCheck is used to store a check registration in the db.
func (s *Store) EnsureCheck(idx uint64, hc *structs.HealthCheck) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the check registration
	if err := s.ensureCheckTxn(tx, idx, hc); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// updateAllServiceIndexesOfNode updates the Raft index of all the services associated with this node
func (s *Store) updateAllServiceIndexesOfNode(tx *memdb.Txn, idx uint64, nodeID string) error {
	services, err := tx.Get("services", "node", nodeID)
	if err != nil {
		return fmt.Errorf("failed updating services for node %s: %s", nodeID, err)
	}
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode).ToNodeService()
		if err := tx.Insert("index", &IndexEntry{serviceIndexName(svc.Service), idx}); err != nil {
			return fmt.Errorf("failed updating index: %s", err)
		}
		if err := tx.Insert("index", &IndexEntry{serviceKindIndexName(svc.Kind), idx}); err != nil {
			return fmt.Errorf("failed updating index: %s", err)
		}
	}
	return nil
}

// ensureCheckCASTxn updates a check only if the existing index matches the given index.
// Returns a bool indicating if a write happened and any error.
func (s *Store) ensureCheckCASTxn(tx *memdb.Txn, idx uint64, hc *structs.HealthCheck) (bool, error) {
	// Retrieve the existing entry.
	_, existing, err := s.getNodeCheckTxn(tx, hc.Node, hc.CheckID)
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
	if err := s.ensureCheckTxn(tx, idx, hc); err != nil {
		return false, err
	}

	return true, nil
}

// ensureCheckTransaction is used as the inner method to handle inserting
// a health check into the state store. It ensures safety against inserting
// checks with no matching node or service.
func (s *Store) ensureCheckTxn(tx *memdb.Txn, idx uint64, hc *structs.HealthCheck) error {
	// Check if we have an existing health check
	existing, err := tx.First("checks", "id", hc.Node, string(hc.CheckID))
	if err != nil {
		return fmt.Errorf("failed health check lookup: %s", err)
	}

	// Set the indexes
	if existing != nil {
		existingCheck := existing.(*structs.HealthCheck)
		hc.CreateIndex = existingCheck.CreateIndex
		hc.ModifyIndex = existingCheck.ModifyIndex
	} else {
		hc.CreateIndex = idx
		hc.ModifyIndex = idx
	}

	// Use the default check status if none was provided
	if hc.Status == "" {
		hc.Status = api.HealthCritical
	}

	// Get the node
	node, err := tx.First("nodes", "id", hc.Node)
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
		service, err := tx.First("services", "id", hc.Node, hc.ServiceID)
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
			// Check has been modified, we trigger a index service change
			if err = tx.Insert("index", &IndexEntry{serviceIndexName(svc.ServiceName), idx}); err != nil {
				return fmt.Errorf("failed updating index: %s", err)
			}
			if err = tx.Insert("index", &IndexEntry{serviceKindIndexName(svc.ServiceKind), idx}); err != nil {
				return fmt.Errorf("failed updating index: %s", err)
			}
		}
	} else {
		if existing != nil && existing.(*structs.HealthCheck).IsSame(hc) {
			modified = false
		} else {
			// Since the check has been modified, it impacts all services of node
			// Update the status for all the services associated with this node
			err = s.updateAllServiceIndexesOfNode(tx, idx, hc.Node)
			if err != nil {
				return err
			}
		}
	}

	// Delete any sessions for this check if the health is critical.
	if hc.Status == api.HealthCritical {
		mappings, err := tx.Get("session_checks", "node_check", hc.Node, string(hc.CheckID))
		if err != nil {
			return fmt.Errorf("failed session checks lookup: %s", err)
		}

		var ids []string
		for mapping := mappings.Next(); mapping != nil; mapping = mappings.Next() {
			ids = append(ids, mapping.(*sessionCheck).Session)
		}

		// Delete the session in a separate loop so we don't trash the
		// iterator.
		for _, id := range ids {
			if err := s.deleteSessionTxn(tx, idx, id); err != nil {
				return fmt.Errorf("failed deleting session: %s", err)
			}
		}
	}
	if modified {
		// We update the modify index, ONLY if something has changed, thus
		// With constant output, no change is seen when watching a service
		// With huge number of nodes where anti-entropy updates continuously
		// the checks, but not the values within the check
		hc.ModifyIndex = idx
	}

	// Persist the check registration in the db.
	if err := tx.Insert("checks", hc); err != nil {
		return fmt.Errorf("failed inserting check: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"checks", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// NodeCheck is used to retrieve a specific check associated with the given
// node.
func (s *Store) NodeCheck(nodeName string, checkID types.CheckID) (uint64, *structs.HealthCheck, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return s.getNodeCheckTxn(tx, nodeName, checkID)
}

// nodeCheckTxn is used as the inner method to handle reading a health check
// from the state store.
func (s *Store) getNodeCheckTxn(tx *memdb.Txn, nodeName string, checkID types.CheckID) (uint64, *structs.HealthCheck, error) {
	// Get the table index.
	idx := maxIndexTxn(tx, "checks")

	// Return the check.
	check, err := tx.First("checks", "id", nodeName, string(checkID))
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
func (s *Store) NodeChecks(ws memdb.WatchSet, nodeName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "checks")

	// Return the checks.
	iter, err := tx.Get("checks", "node", nodeName)
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
func (s *Store) ServiceChecks(ws memdb.WatchSet, serviceName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "checks")

	// Return the checks.
	iter, err := tx.Get("checks", "service", serviceName)
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
func (s *Store) ServiceChecksByNodeMeta(ws memdb.WatchSet, serviceName string,
	filters map[string]string) (uint64, structs.HealthChecks, error) {

	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexForService(tx, serviceName, true, true)
	// Return the checks.
	iter, err := tx.Get("checks", "service", serviceName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	return s.parseChecksByNodeMeta(tx, ws, idx, iter, filters)
}

// ChecksInState is used to query the state store for all checks
// which are in the provided state.
func (s *Store) ChecksInState(ws memdb.WatchSet, state string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "checks")

	// Query all checks if HealthAny is passed, otherwise use the index.
	var iter memdb.ResultIterator
	var err error
	if state == api.HealthAny {
		iter, err = tx.Get("checks", "status")
	} else {
		iter, err = tx.Get("checks", "status", state)
	}
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

// ChecksInStateByNodeMeta is used to query the state store for all checks
// which are in the provided state, filtered by the given node metadata values.
func (s *Store) ChecksInStateByNodeMeta(ws memdb.WatchSet, state string, filters map[string]string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "nodes", "checks")

	// Query all checks if HealthAny is passed, otherwise use the index.
	var iter memdb.ResultIterator
	var err error
	if state == api.HealthAny {
		iter, err = tx.Get("checks", "status")
		if err != nil {
			return 0, nil, fmt.Errorf("failed check lookup: %s", err)
		}
	} else {
		iter, err = tx.Get("checks", "status", state)
		if err != nil {
			return 0, nil, fmt.Errorf("failed check lookup: %s", err)
		}
	}
	ws.Add(iter.WatchCh())

	return s.parseChecksByNodeMeta(tx, ws, idx, iter, filters)
}

// parseChecksByNodeMeta is a helper function used to deduplicate some
// repetitive code for returning health checks filtered by node metadata fields.
func (s *Store) parseChecksByNodeMeta(tx *memdb.Txn, ws memdb.WatchSet,
	idx uint64, iter memdb.ResultIterator, filters map[string]string) (uint64, structs.HealthChecks, error) {

	// We don't want to track an unlimited number of nodes, so we pull a
	// top-level watch to use as a fallback.
	allNodes, err := tx.Get("nodes", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	allNodesCh := allNodes.WatchCh()

	// Only take results for nodes that satisfy the node metadata filters.
	var results structs.HealthChecks
	for check := iter.Next(); check != nil; check = iter.Next() {
		healthCheck := check.(*structs.HealthCheck)
		watchCh, node, err := tx.FirstWatch("nodes", "id", healthCheck.Node)
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
func (s *Store) DeleteCheck(idx uint64, node string, checkID types.CheckID) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the check deletion
	if err := s.deleteCheckTxn(tx, idx, node, checkID); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// deleteCheckCASTxn is used to try doing a check delete operation with a given
// raft index. If the CAS index specified is not equal to the last observed index for
// the given check, then the call is a noop, otherwise a normal check delete is invoked.
func (s *Store) deleteCheckCASTxn(tx *memdb.Txn, idx, cidx uint64, node string, checkID types.CheckID) (bool, error) {
	// Try to retrieve the existing health check.
	_, hc, err := s.getNodeCheckTxn(tx, node, checkID)
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
	if err := s.deleteCheckTxn(tx, idx, node, checkID); err != nil {
		return false, err
	}

	return true, nil
}

// deleteCheckTxn is the inner method used to call a health
// check deletion within an existing transaction.
func (s *Store) deleteCheckTxn(tx *memdb.Txn, idx uint64, node string, checkID types.CheckID) error {
	// Try to retrieve the existing health check.
	hc, err := tx.First("checks", "id", node, string(checkID))
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
			if err = tx.Insert("index", &IndexEntry{serviceIndexName(existing.ServiceName), idx}); err != nil {
				return fmt.Errorf("failed updating index: %s", err)
			}

			svcRaw, err := tx.First("services", "id", existing.Node, existing.ServiceID)
			if err != nil {
				return fmt.Errorf("failed retrieving service from state store: %v", err)
			}

			svc := svcRaw.(*structs.ServiceNode)
			if err := tx.Insert("index", &IndexEntry{serviceKindIndexName(svc.ServiceKind), idx}); err != nil {
				return fmt.Errorf("failed updating index: %s", err)
			}
		} else {
			err = s.updateAllServiceIndexesOfNode(tx, idx, existing.Node)
			if err != nil {
				return fmt.Errorf("Failed to update services linked to deleted healthcheck: %s", err)
			}
			if err := tx.Insert("index", &IndexEntry{"services", idx}); err != nil {
				return fmt.Errorf("failed updating index: %s", err)
			}
		}
	}

	// Delete the check from the DB and update the index.
	if err := tx.Delete("checks", hc); err != nil {
		return fmt.Errorf("failed removing check: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"checks", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// Delete any sessions for this check.
	mappings, err := tx.Get("session_checks", "node_check", node, string(checkID))
	if err != nil {
		return fmt.Errorf("failed session checks lookup: %s", err)
	}
	var ids []string
	for mapping := mappings.Next(); mapping != nil; mapping = mappings.Next() {
		ids = append(ids, mapping.(*sessionCheck).Session)
	}

	// Do the delete in a separate loop so we don't trash the iterator.
	for _, id := range ids {
		if err := s.deleteSessionTxn(tx, idx, id); err != nil {
			return fmt.Errorf("failed deleting session: %s", err)
		}
	}

	return nil
}

// CheckServiceNodes is used to query all nodes and checks for a given service.
func (s *Store) CheckServiceNodes(ws memdb.WatchSet, serviceName string) (uint64, structs.CheckServiceNodes, error) {
	return s.checkServiceNodes(ws, serviceName, false)
}

// CheckConnectServiceNodes is used to query all nodes and checks for Connect
// compatible endpoints for a given service.
func (s *Store) CheckConnectServiceNodes(ws memdb.WatchSet, serviceName string) (uint64, structs.CheckServiceNodes, error) {
	return s.checkServiceNodes(ws, serviceName, true)
}

func (s *Store) checkServiceNodes(ws memdb.WatchSet, serviceName string, connect bool) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Function for lookup
	var f func() (memdb.ResultIterator, error)
	if !connect {
		f = func() (memdb.ResultIterator, error) {
			return tx.Get("services", "service", serviceName)
		}
	} else {
		f = func() (memdb.ResultIterator, error) {
			return tx.Get("services", "connect", serviceName)
		}
	}

	// Query the state store for the service.
	iter, err := f()
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
	serviceNames := make(map[string]struct{}, 2)
	for service := iter.Next(); service != nil; service = iter.Next() {
		sn := service.(*structs.ServiceNode)
		results = append(results, sn)
		serviceNames[sn.ServiceName] = struct{}{}
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
	idx := uint64(0)
	if len(serviceNames) > 0 {
		// Assume optimization will work since it really should at this point. For
		// safety we'll sanity check this below for each service name.
		watchOptimized = true

		// Fetch indexes for all names services in result set.
		for svcName := range serviceNames {
			// We know service values should exist since the serviceNames map is only
			// populated if there is at least one result above. so serviceExists arg
			// below is always true.
			svcIdx, svcCh := maxIndexAndWatchChForService(tx, svcName, true, true)
			// Take the max index represented
			if idx < svcIdx {
				idx = svcIdx
			}
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
		// extinction event so we don't go backwards when services de-register. We
		// use target serviceName here but it actually doesn't matter. No chan will
		// be returned as we can't use the optimization in this case (and don't need
		// to as there is only one chan to watch anyway).
		idx, _ = maxIndexAndWatchChForService(tx, serviceName, false, true)
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

	return s.parseCheckServiceNodes(tx, fallbackWS, idx, serviceName, results, err)
}

// CheckServiceTagNodes is used to query all nodes and checks for a given
// service, filtering out services that don't contain the given tag.
func (s *Store) CheckServiceTagNodes(ws memdb.WatchSet, serviceName string, tags []string) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query the state store for the service.
	iter, err := tx.Get("services", "service", serviceName)
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
	idx := maxIndexForService(tx, serviceName, serviceExists, true)
	return s.parseCheckServiceNodes(tx, ws, idx, serviceName, results, err)
}

// parseCheckServiceNodes is used to parse through a given set of services,
// and query for an associated node and a set of checks. This is the inner
// method used to return a rich set of results from a more simple query.
func (s *Store) parseCheckServiceNodes(
	tx *memdb.Txn, ws memdb.WatchSet, idx uint64,
	serviceName string, services structs.ServiceNodes,
	err error) (uint64, structs.CheckServiceNodes, error) {
	if err != nil {
		return 0, nil, err
	}

	// Special-case the zero return value to nil, since this ends up in
	// external APIs.
	if len(services) == 0 {
		return idx, nil, nil
	}

	// We don't want to track an unlimited number of nodes, so we pull a
	// top-level watch to use as a fallback.
	allNodes, err := tx.Get("nodes", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}
	allNodesCh := allNodes.WatchCh()

	// We need a similar fallback for checks. Since services need the
	// status of node + service-specific checks, we pull in a top-level
	// watch over all checks.
	allChecks, err := tx.Get("checks", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed checks lookup: %s", err)
	}
	allChecksCh := allChecks.WatchCh()

	results := make(structs.CheckServiceNodes, 0, len(services))
	for _, sn := range services {
		// Retrieve the node.
		watchCh, n, err := tx.FirstWatch("nodes", "id", sn.Node)
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
		iter, err := tx.Get("checks", "node_service_check", sn.Node, false)
		if err != nil {
			return 0, nil, err
		}
		ws.AddWithLimit(watchLimit, iter.WatchCh(), allChecksCh)
		for check := iter.Next(); check != nil; check = iter.Next() {
			checks = append(checks, check.(*structs.HealthCheck))
		}

		// Now add the service-specific checks.
		iter, err = tx.Get("checks", "node_service", sn.Node, sn.ServiceID)
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
func (s *Store) NodeInfo(ws memdb.WatchSet, node string) (uint64, structs.NodeDump, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "nodes", "services", "checks")

	// Query the node by the passed node
	nodes, err := tx.Get("nodes", "id", node)
	if err != nil {
		return 0, nil, fmt.Errorf("failed node lookup: %s", err)
	}
	ws.Add(nodes.WatchCh())
	return s.parseNodes(tx, ws, idx, nodes)
}

// NodeDump is used to generate a dump of all nodes. This call is expensive
// as it has to query every node, service, and check. The response can also
// be quite large since there is currently no filtering applied.
func (s *Store) NodeDump(ws memdb.WatchSet) (uint64, structs.NodeDump, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, "nodes", "services", "checks")

	// Fetch all of the registered nodes
	nodes, err := tx.Get("nodes", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed node lookup: %s", err)
	}
	ws.Add(nodes.WatchCh())
	return s.parseNodes(tx, ws, idx, nodes)
}

func (s *Store) ServiceDump(ws memdb.WatchSet, kind structs.ServiceKind, useKind bool) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	if useKind {
		return s.serviceDumpKindTxn(tx, ws, kind)
	} else {
		return s.serviceDumpAllTxn(tx, ws)
	}
}

func (s *Store) serviceDumpAllTxn(tx *memdb.Txn, ws memdb.WatchSet) (uint64, structs.CheckServiceNodes, error) {
	// Get the table index
	idx := maxIndexWatchTxn(tx, ws, "nodes", "services", "checks")

	services, err := tx.Get("services", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}

	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		sn := service.(*structs.ServiceNode)
		results = append(results, sn)
	}

	return s.parseCheckServiceNodes(tx, nil, idx, "", results, err)
}

func (s *Store) serviceDumpKindTxn(tx *memdb.Txn, ws memdb.WatchSet, kind structs.ServiceKind) (uint64, structs.CheckServiceNodes, error) {
	// unlike when we are dumping all services here we only need to watch the kind specific index entry for changing (or nodes, checks)
	// updating any services, nodes or checks will bump the appropriate service kind index so there is no need to watch any of the individual
	// entries
	idx := maxIndexWatchTxn(tx, ws, serviceKindIndexName(kind))

	// Query the state store for the service.
	services, err := tx.Get("services", "kind", string(kind))
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}

	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		sn := service.(*structs.ServiceNode)
		results = append(results, sn)
	}

	return s.parseCheckServiceNodes(tx, nil, idx, "", results, err)
}

// parseNodes takes an iterator over a set of nodes and returns a struct
// containing the nodes along with all of their associated services
// and/or health checks.
func (s *Store) parseNodes(tx *memdb.Txn, ws memdb.WatchSet, idx uint64,
	iter memdb.ResultIterator) (uint64, structs.NodeDump, error) {

	// We don't want to track an unlimited number of services, so we pull a
	// top-level watch to use as a fallback.
	allServices, err := tx.Get("services", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed services lookup: %s", err)
	}
	allServicesCh := allServices.WatchCh()

	// We need a similar fallback for checks.
	allChecks, err := tx.Get("checks", "id")
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
			Address:         node.Address,
			TaggedAddresses: node.TaggedAddresses,
			Meta:            node.Meta,
		}

		// Query the node services
		services, err := tx.Get("services", "node", node.Node)
		if err != nil {
			return 0, nil, fmt.Errorf("failed services lookup: %s", err)
		}
		ws.AddWithLimit(watchLimit, services.WatchCh(), allServicesCh)
		for service := services.Next(); service != nil; service = services.Next() {
			ns := service.(*structs.ServiceNode).ToNodeService()
			dump.Services = append(dump.Services, ns)
		}

		// Query the node checks
		checks, err := tx.Get("checks", "node", node.Node)
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
