package state

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/go-memdb"
)

var (
	// ErrMissingNode is the error returned when trying an operation
	// which requires a node registration but none exists.
	ErrMissingNode = errors.New("Missing node registration")

	// ErrMissingService is the error we return if trying an
	// operation which requires a service but none exists.
	ErrMissingService = errors.New("Missing service registration")

	// ErrMissingSessionID is returned when a session registration
	// is attempted with an empty session ID.
	ErrMissingSessionID = errors.New("Missing session ID")

	// ErrMissingACLID is returned when a session set is called on
	// a session with an empty ID.
	ErrMissingACLID = errors.New("Missing ACL ID")
)

// StateStore is where we store all of Consul's state, including
// records of node registrations, services, checks, key/value
// pairs and more. The DB is entirely in-memory and is constructed
// from the Raft log through the FSM.
type StateStore struct {
	logger *log.Logger
	db     *memdb.MemDB
}

// IndexEntry keeps a record of the last index per-table.
type IndexEntry struct {
	Key   string
	Value uint64
}

// sessionCheck is used to create a many-to-one table such that
// each check registered by a session can be mapped back to the
// session table. This is only used internally in the state
// store and thus it is not exported.
type sessionCheck struct {
	Node    string
	CheckID string
	Session string
}

// NewStateStore creates a new in-memory state storage layer.
func NewStateStore(logOutput io.Writer) (*StateStore, error) {
	// Create the in-memory DB
	db, err := memdb.NewMemDB(stateStoreSchema())
	if err != nil {
		return nil, fmt.Errorf("Failed setting up state store: %s", err)
	}

	// Create and return the state store
	s := &StateStore{
		logger: log.New(logOutput, "", log.LstdFlags),
		db:     db,
	}
	return s, nil
}

// maxIndex is a helper used to retrieve the highest known index
// amongst a set of tables in the db.
func (s *StateStore) maxIndex(tables ...string) uint64 {
	tx := s.db.Txn(false)
	defer tx.Abort()

	var lindex uint64
	for _, table := range tables {
		ti, err := tx.First("index", "id", table)
		if err != nil {
			panic(fmt.Sprintf("unknown index: %s", table))
		}
		if idx, ok := ti.(*IndexEntry); ok && idx.Value > lindex {
			lindex = idx.Value
		}
	}
	return lindex
}

// EnsureNode is used to upsert node registration or modification.
func (s *StateStore) EnsureNode(idx uint64, node *structs.Node) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the node upsert
	if err := s.ensureNodeTxn(idx, node, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// ensureNodeTxn is the inner function called to actually create a node
// registration or modify an existing one in the state store. It allows
// passing in a memdb transaction so it may be part of a larger txn.
func (s *StateStore) ensureNodeTxn(idx uint64, node *structs.Node, tx *memdb.Txn) error {
	// Check for an existing node
	existing, err := tx.First("nodes", "id", node.Node)
	if err != nil {
		return fmt.Errorf("node lookup failed: %s", err)
	}

	// Get the indexes
	if existing != nil {
		node.CreateIndex = existing.(*structs.Node).CreateIndex
		node.ModifyIndex = idx
	} else {
		node.CreateIndex = idx
		node.ModifyIndex = idx
	}

	// Insert the node and update the index
	if err := tx.Insert("nodes", node); err != nil {
		return fmt.Errorf("failed inserting node: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"nodes", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// GetNode is used to retrieve a node registration by node ID.
func (s *StateStore) GetNode(id string) (*structs.Node, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Retrieve the node from the state store
	node, err := tx.First("nodes", "id", id)
	if err != nil {
		return nil, fmt.Errorf("node lookup failed: %s", err)
	}
	if node != nil {
		return node.(*structs.Node), nil
	}
	return nil, nil
}

// Nodes is used to return all of the known nodes.
func (s *StateStore) Nodes() (uint64, structs.Nodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Retrieve all of the nodes
	nodes, err := tx.Get("nodes", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}

	// Create and return the nodes list, tracking the highest
	// index we see.
	var lindex uint64
	var results structs.Nodes
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		n := node.(*structs.Node)
		if n.ModifyIndex > lindex {
			lindex = n.ModifyIndex
		}
		results = append(results, node.(*structs.Node))
	}
	return lindex, results, nil
}

// DeleteNode is used to delete a given node by its ID.
func (s *StateStore) DeleteNode(idx uint64, nodeID string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the node deletion.
	if err := s.deleteNodeTxn(idx, nodeID, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// deleteNodeTxn is the inner method used for removing a node from
// the store within a given transaction.
func (s *StateStore) deleteNodeTxn(idx uint64, nodeID string, tx *memdb.Txn) error {
	// Look up the node
	node, err := tx.First("nodes", "id", nodeID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %s", err)
	}

	// Delete all services associated with the node and update the service index
	services, err := tx.Get("services", "node", nodeID)
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		if err := s.deleteServiceTxn(idx, nodeID, svc.ServiceID, tx); err != nil {
			return err
		}
	}

	// Delete all checks associated with the node and update the check index
	checks, err := tx.Get("checks", "node", nodeID)
	if err != nil {
		return fmt.Errorf("failed check lookup: %s", err)
	}
	for check := checks.Next(); check != nil; check = checks.Next() {
		chk := check.(*structs.HealthCheck)
		if err := s.deleteCheckTxn(idx, nodeID, chk.CheckID, tx); err != nil {
			return err
		}
	}

	// Delete the node and update the index
	if err := tx.Delete("nodes", node); err != nil {
		return fmt.Errorf("failed deleting node: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"nodes", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// TODO: session invalidation
	// TODO: watch trigger
	return nil
}

// EnsureService is called to upsert creation of a given NodeService.
func (s *StateStore) EnsureService(idx uint64, node string, svc *structs.NodeService) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the service registration upsert
	if err := s.ensureServiceTxn(idx, node, svc, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// ensureServiceTxn is used to upsert a service registration within an
// existing memdb transaction.
func (s *StateStore) ensureServiceTxn(idx uint64, node string, svc *structs.NodeService, tx *memdb.Txn) error {
	// Check for existing service
	existing, err := tx.First("services", "id", node, svc.Service)
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}

	// Create the service node entry
	entry := &structs.ServiceNode{
		Node:           node,
		ServiceID:      svc.ID,
		ServiceName:    svc.Service,
		ServiceTags:    svc.Tags,
		ServiceAddress: svc.Address,
		ServicePort:    svc.Port,
	}

	// Populate the indexes
	if existing != nil {
		entry.CreateIndex = existing.(*structs.ServiceNode).CreateIndex
		entry.ModifyIndex = idx
	} else {
		entry.CreateIndex = idx
		entry.ModifyIndex = idx
	}

	// Insert the service and update the index
	if err := tx.Insert("services", entry); err != nil {
		return fmt.Errorf("failed inserting service: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"services", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// NodeServices is used to query service registrations by node ID.
func (s *StateStore) NodeServices(nodeID string) (uint64, *structs.NodeServices, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query the node
	n, err := tx.First("nodes", "id", nodeID)
	if err != nil {
		return 0, nil, fmt.Errorf("node lookup failed: %s", err)
	}
	if n == nil {
		return 0, nil, nil
	}
	node := n.(*structs.Node)

	// Read all of the services
	services, err := tx.Get("services", "node", nodeID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying services for node %q: %s", nodeID, err)
	}

	// Initialize the node services struct
	ns := &structs.NodeServices{
		Node:     node,
		Services: make(map[string]*structs.NodeService),
	}

	// Add all of the services to the map, tracking the highest index
	var lindex uint64
	for service := services.Next(); service != nil; service = services.Next() {
		sn := service.(*structs.ServiceNode)

		// Track the highest index
		if sn.CreateIndex > lindex {
			lindex = sn.CreateIndex
		}

		// Create the NodeService
		svc := &structs.NodeService{
			ID:      sn.ServiceID,
			Service: sn.ServiceName,
			Tags:    sn.ServiceTags,
			Address: sn.ServiceAddress,
			Port:    sn.ServicePort,
		}
		svc.CreateIndex = sn.CreateIndex
		svc.ModifyIndex = sn.ModifyIndex

		// Add the service to the result
		ns.Services[svc.ID] = svc
	}

	return lindex, ns, nil
}

// DeleteService is used to delete a given service associated with a node.
func (s *StateStore) DeleteService(idx uint64, nodeID, serviceID string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the service deletion
	if err := s.deleteServiceTxn(idx, nodeID, serviceID, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// deleteServiceTxn is the inner method called to remove a service
// registration within an existing transaction.
func (s *StateStore) deleteServiceTxn(idx uint64, nodeID, serviceID string, tx *memdb.Txn) error {
	// Look up the service
	service, err := tx.First("services", "id", nodeID, serviceID)
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}

	// Delete any checks associated with the service
	checks, err := tx.Get("checks", "node_service", nodeID, serviceID)
	if err != nil {
		return fmt.Errorf("failed service check lookup: %s", err)
	}
	for check := checks.Next(); check != nil; check = checks.Next() {
		if err := tx.Delete("checks", check); err != nil {
			return fmt.Errorf("failed deleting service check: %s", err)
		}
	}
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

	// TODO: session invalidation
	// TODO: watch trigger
	return nil
}

// EnsureCheck is used to store a check registration in the db.
func (s *StateStore) EnsureCheck(idx uint64, hc *structs.HealthCheck) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the check registration
	if err := s.ensureCheckTxn(idx, hc, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// ensureCheckTransaction is used as the inner method to handle inserting
// a health check into the state store. It ensures safety against inserting
// checks with no matching node or service.
func (s *StateStore) ensureCheckTxn(idx uint64, hc *structs.HealthCheck, tx *memdb.Txn) error {
	// Check if we have an existing health check
	existing, err := tx.First("checks", "id", hc.Node, hc.CheckID)
	if err != nil {
		return fmt.Errorf("failed health check lookup: %s", err)
	}

	// Set the indexes
	if existing != nil {
		hc.CreateIndex = existing.(*structs.HealthCheck).CreateIndex
		hc.ModifyIndex = idx
	} else {
		hc.CreateIndex = idx
		hc.ModifyIndex = idx
	}

	// Use the default check status if none was provided
	if hc.Status == "" {
		hc.Status = structs.HealthCritical
	}

	// Get the node
	node, err := tx.First("nodes", "id", hc.Node)
	if err != nil {
		return fmt.Errorf("failed node lookup: %s", err)
	}
	if node == nil {
		return ErrMissingNode
	}

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

		// Copy in the service name
		hc.ServiceName = service.(*structs.ServiceNode).ServiceName
	}

	// TODO: invalidate sessions if status == critical

	// Persist the check registration in the db
	if err := tx.Insert("checks", hc); err != nil {
		return fmt.Errorf("failed inserting service: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"checks", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// TODO: trigger watches

	return nil
}

// NodeChecks is used to retrieve checks associated with the
// given node from the state store.
func (s *StateStore) NodeChecks(nodeID string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return s.parseChecks(tx.Get("checks", "node", nodeID))
}

// ServiceChecks is used to get all checks associated with a
// given service ID. The query is performed against a service
// _name_ instead of a service ID.
func (s *StateStore) ServiceChecks(serviceName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return s.parseChecks(tx.Get("checks", "service", serviceName))
}

// ChecksInState is used to query the state store for all checks
// which are in the provided state.
func (s *StateStore) ChecksInState(state string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query all checks if HealthAny is passed
	if state == structs.HealthAny {
		return s.parseChecks(tx.Get("checks", "status"))
	}

	// Any other state we need to query for explicitly
	return s.parseChecks(tx.Get("checks", "status", state))
}

// parseChecks is a helper function used to deduplicate some
// repetitive code for returning health checks.
func (s *StateStore) parseChecks(iter memdb.ResultIterator, err error) (uint64, structs.HealthChecks, error) {
	if err != nil {
		return 0, nil, fmt.Errorf("failed health check lookup: %s", err)
	}

	// Gather the health checks and return them properly type casted.
	// Track the highest index along the way.
	var results structs.HealthChecks
	var lindex uint64
	for hc := iter.Next(); hc != nil; hc = iter.Next() {
		check := hc.(*structs.HealthCheck)
		if check.ModifyIndex > lindex {
			lindex = check.ModifyIndex
		}
		results = append(results, check)
	}
	return lindex, results, nil
}

// DeleteCheck is used to delete a health check registration.
func (s *StateStore) DeleteCheck(idx uint64, node, id string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the check deletion
	if err := s.deleteCheckTxn(idx, node, id, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// deleteCheckTxn is the inner method used to call a health
// check deletion within an existing transaction.
func (s *StateStore) deleteCheckTxn(idx uint64, node, id string, tx *memdb.Txn) error {
	// Try to retrieve the existing health check
	check, err := tx.First("checks", "id", node, id)
	if err != nil {
		return fmt.Errorf("check lookup failed: %s", err)
	}

	// Delete the check from the DB and update the index
	if err := tx.Delete("checks", check); err != nil {
		return fmt.Errorf("failed removing check: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"checks", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// TODO: invalidate sessions
	// TODO: watch triggers
	return nil
}

// CheckServiceNodes is used to query all nodes and checks for a given service
// ID. The results are compounded into a CheckServiceNodes, and the index
// returned is the maximum index observed over any node, check, or service
// in the result set.
func (s *StateStore) CheckServiceNodes(serviceID string) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query the state store for the service.
	services, err := tx.Get("services", "service", serviceID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}
	return s.parseCheckServiceNodes(tx, services, err)
}

// parseCheckServiceNodes is used to parse through a given set of services,
// and query for an associated node and a set of checks. This is the inner
// method used to return a rich set of results from a more simple query.
func (s *StateStore) parseCheckServiceNodes(
	tx *memdb.Txn, iter memdb.ResultIterator,
	err error) (uint64, structs.CheckServiceNodes, error) {
	if err != nil {
		return 0, nil, err
	}

	var results structs.CheckServiceNodes
	var lindex uint64
	for service := iter.Next(); service != nil; service = iter.Next() {
		// Compute the index
		svc := service.(*structs.ServiceNode)
		if svc.ModifyIndex > lindex {
			lindex = svc.ModifyIndex
		}

		// Retrieve the node
		n, err := tx.First("nodes", "id", svc.Node)
		if err != nil {
			return 0, nil, fmt.Errorf("failed node lookup: %s", err)
		}
		if n == nil {
			return 0, nil, ErrMissingNode
		}
		node := n.(*structs.Node)
		if node.ModifyIndex > lindex {
			lindex = node.ModifyIndex
		}

		// Get the checks
		idx, checks, err := s.parseChecks(tx.Get("checks", "node_service", svc.Node, svc.ServiceID))
		if err != nil {
			return 0, nil, err
		}
		if idx > lindex {
			lindex = idx
		}

		// Append to the results
		results = append(results, structs.CheckServiceNode{
			Node: node,
			Service: &structs.NodeService{
				ID:      svc.ServiceID,
				Service: svc.ServiceName,
				Address: svc.ServiceAddress,
				Port:    svc.ServicePort,
				Tags:    svc.ServiceTags,
			},
			Checks: checks,
		})
	}

	return lindex, results, nil
}

// NodeInfo is used to generate a dump of a single node. The dump includes
// all services and checks which are registered against the node.
func (s *StateStore) NodeInfo(nodeID string) (uint64, structs.NodeDump, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query the node by the passed node ID
	nodes, err := tx.Get("nodes", "id", nodeID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed node lookup: %s", err)
	}
	return s.parseNodes(tx, nodes)
}

// NodeDump is used to generate a dump of all nodes. This call is expensive
// as it has to query every node, service, and check. The response can also
// be quite large since there is currently no filtering applied.
func (s *StateStore) NodeDump() (uint64, structs.NodeDump, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Fetch all of the registered nodes
	nodes, err := tx.Get("nodes", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed node lookup: %s", err)
	}
	return s.parseNodes(tx, nodes)
}

// parseNodes takes an iterator over a set of nodes and returns a struct
// containing the nodes along with all of their associated services
// and/or health checks.
func (s *StateStore) parseNodes(
	tx *memdb.Txn,
	iter memdb.ResultIterator) (uint64, structs.NodeDump, error) {

	var results structs.NodeDump
	var lindex uint64
	for n := iter.Next(); n != nil; n = iter.Next() {
		node := n.(*structs.Node)
		if node.ModifyIndex > lindex {
			lindex = node.ModifyIndex
		}

		// Create the wrapped node
		dump := &structs.NodeInfo{
			Node:    node.Node,
			Address: node.Address,
		}

		// Query the node services
		services, err := tx.Get("services", "node", node.Node)
		if err != nil {
			return 0, nil, fmt.Errorf("failed services lookup: %s", err)
		}
		for service := services.Next(); service != nil; service = services.Next() {
			svc := service.(*structs.ServiceNode)
			if svc.ModifyIndex > lindex {
				lindex = svc.ModifyIndex
			}
			ns := &structs.NodeService{
				ID:      svc.ServiceID,
				Service: svc.ServiceName,
				Address: svc.ServiceAddress,
				Port:    svc.ServicePort,
				Tags:    svc.ServiceTags,
			}
			ns.CreateIndex = svc.CreateIndex
			ns.ModifyIndex = svc.ModifyIndex
			dump.Services = append(dump.Services, ns)
		}

		// Query the node checks
		checks, err := tx.Get("checks", "node", node.Node)
		if err != nil {
			return 0, nil, fmt.Errorf("failed node lookup: %s", err)
		}
		for check := checks.Next(); check != nil; check = checks.Next() {
			chk := check.(*structs.HealthCheck)
			if chk.ModifyIndex > lindex {
				lindex = chk.ModifyIndex
			}
			dump.Checks = append(dump.Checks, chk)
		}

		// Add the result to the slice
		results = append(results, dump)
	}
	return lindex, results, nil
}

// KVSSet is used to store a key/value pair.
func (s *StateStore) KVSSet(idx uint64, entry *structs.DirEntry) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	return s.kvsSetTxn(idx, entry, tx)
}

// kvsSetTxn is used to insert or update a key/value pair in the state
// store. It is the inner method used and handles only the actual storage.
func (s *StateStore) kvsSetTxn(
	idx uint64, entry *structs.DirEntry,
	tx *memdb.Txn) error {

	// Retrieve an existing KV pair
	existing, err := tx.First("kvs", "id", entry.Key)
	if err != nil {
		return fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Set the indexes
	if existing != nil {
		entry.CreateIndex = existing.(*structs.DirEntry).CreateIndex
		entry.ModifyIndex = idx
	} else {
		entry.CreateIndex = idx
		entry.ModifyIndex = idx
	}

	// Store the kv pair in the state store and update the index
	if err := tx.Insert("kvs", entry); err != nil {
		return fmt.Errorf("failed inserting kvs entry: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"kvs", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}

// KVSGet is used to retrieve a key/value pair from the state store.
func (s *StateStore) KVSGet(key string) (*structs.DirEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	entry, err := tx.First("kvs", "id", key)
	if err != nil {
		return nil, fmt.Errorf("failed kvs lookup: %s", err)
	}
	if entry != nil {
		return entry.(*structs.DirEntry), nil
	}
	return nil, nil
}

// KVSList is used to list out all keys under a given prefix. If the
// prefix is left empty, all keys in the KVS will be returned. The
// returned index is the max index of the returned kvs entries.
func (s *StateStore) KVSList(prefix string) (uint64, []string, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query the prefix and list the available keys
	entries, err := tx.Get("kvs", "id_prefix", prefix)
	if err != nil {
		return 0, nil, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Gather all of the keys found in the store
	var keys []string
	var lindex uint64
	for entry := entries.Next(); entry != nil; entry = entries.Next() {
		e := entry.(*structs.DirEntry)
		keys = append(keys, e.Key)
		if e.ModifyIndex > lindex {
			lindex = e.ModifyIndex
		}
	}
	return lindex, keys, nil
}

// KVSListKeys is used to query the KV store for keys matching the given prefix.
// An optional separator may be specified, which can be used to slice off a part
// of the response so that only a subset of the prefix is returned. In this
// mode, the keys which are omitted are still counted in the returned index.
func (s *StateStore) KVSListKeys(prefix, sep string) (uint64, []string, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Fetch keys using the specified prefix
	entries, err := tx.Get("kvs", "id_prefix", prefix)
	if err != nil {
		return 0, nil, fmt.Errorf("failed kvs lookup: %s", err)
	}

	prefixLen := len(prefix)
	sepLen := len(sep)

	var keys []string
	var lindex uint64
	var last string
	for entry := entries.Next(); entry != nil; entry = entries.Next() {
		e := entry.(*structs.DirEntry)

		// Accumulate the high index
		if e.ModifyIndex > lindex {
			lindex = e.ModifyIndex
		}

		// Always accumulate if no separator provided
		if sepLen == 0 {
			keys = append(keys, e.Key)
			continue
		}

		// Parse and de-duplicate the returned keys based on the
		// key separator, if provided.
		after := e.Key[prefixLen:]
		sepIdx := strings.Index(after, sep)
		if sepIdx > -1 {
			key := e.Key[:prefixLen+sepIdx+sepLen]
			if key != last {
				keys = append(keys, key)
				last = key
			}
		} else {
			keys = append(keys, e.Key)
		}
	}
	return lindex, keys, nil
}

// KVSDelete is used to perform a shallow delete on a single key in the
// the state store.
func (s *StateStore) KVSDelete(idx uint64, key string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Perform the actual delete
	if err := s.kvsDeleteTxn(idx, key, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// kvsDeleteTxn is the inner method used to perform the actual deletion
// of a key/value pair within an existing transaction.
func (s *StateStore) kvsDeleteTxn(idx uint64, key string, tx *memdb.Txn) error {
	// Look up the entry in the state store
	entry, err := tx.First("kvs", "id", key)
	if err != nil {
		return fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Delete the entry and update the index
	if err := tx.Delete("kvs", entry); err != nil {
		return fmt.Errorf("failed deleting kvs entry: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"kvs", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// KVSDeleteCAS is used to try doing a KV delete operation with a given
// raft index. If the CAS index specified is not equal to the last
// observed index for the given key, then the call is a noop, otherwise
// a normal KV delete is invoked.
func (s *StateStore) KVSDeleteCAS(idx, cidx uint64, key string) (bool, error) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Retrieve the existing kvs entry, if any exists
	entry, err := tx.First("kvs", "id", key)
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	e, ok := entry.(*structs.DirEntry)
	if !ok || e.ModifyIndex != cidx {
		return entry == nil, nil
	}

	// Call the actual deletion if the above passed
	if err := s.kvsDeleteTxn(idx, key, tx); err != nil {
		return false, err
	}

	tx.Commit()
	return true, nil
}

// KVSSetCAS is used to do a check-and-set operation on a KV entry. The
// ModifyIndex in the provided entry is used to determine if we should
// write the entry to the state store or bail. Returns a bool indicating
// if a write happened and any error.
func (s *StateStore) KVSSetCAS(idx uint64, entry *structs.DirEntry) (bool, error) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Retrieve the existing entry
	existing, err := tx.First("kvs", "id", entry.Key)
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	if entry.ModifyIndex == 0 && existing != nil {
		return false, nil
	}
	if entry.ModifyIndex != 0 && existing == nil {
		return false, nil
	}
	e, ok := existing.(*structs.DirEntry)
	if ok && entry.ModifyIndex != 0 && entry.ModifyIndex != e.ModifyIndex {
		return false, nil
	}

	// If we made it this far, we should perform the set.
	return true, s.kvsSetTxn(idx, entry, tx)
}

// KVSDeleteTree is used to do a recursive delete on a key prefix
// in the state store. If any keys are modified, the last index is
// set, otherwise this is a no-op.
func (s *StateStore) KVSDeleteTree(idx uint64, prefix string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Get an iterator over all of the keys with the given prefix
	entries, err := tx.Get("kvs", "id_prefix", prefix)
	if err != nil {
		return fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Go over all of the keys and remove them. We call the delete
	// directly so that we only update the index once.
	var modified bool
	for entry := entries.Next(); entry != nil; entry = entries.Next() {
		err := tx.Delete("kvs", entry.(*structs.DirEntry))
		if err != nil {
			return fmt.Errorf("failed deleting kvs entry: %s", err)
		}
		modified = true
	}

	// Update the index
	if modified {
		if err := tx.Insert("index", &IndexEntry{"kvs", idx}); err != nil {
			return fmt.Errorf("failed updating index: %s", err)
		}
	}

	tx.Commit()
	return nil
}

// SessionCreate is used to register a new session in the state store.
func (s *StateStore) SessionCreate(idx uint64, sess *structs.Session) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the session creation
	if err := s.sessionCreateTxn(idx, sess, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// sessionCreateTxn is the inner method used for creating session entries in
// an open transaction. Any health checks registered with the session will be
// checked for failing status. Returns any error encountered.
func (s *StateStore) sessionCreateTxn(idx uint64, sess *structs.Session, tx *memdb.Txn) error {
	// Check that we have a session ID
	if sess.ID == "" {
		return ErrMissingSessionID
	}

	// Verify the session behavior is valid
	switch sess.Behavior {
	case "":
		// Release by default to preserve backwards compatibility
		sess.Behavior = structs.SessionKeysRelease
	case structs.SessionKeysRelease:
	case structs.SessionKeysDelete:
	default:
		return fmt.Errorf("Invalid session behavior: %s", sess.Behavior)
	}

	// Assign the indexes. ModifyIndex likely will not be used but
	// we set it here anyways for sanity.
	sess.CreateIndex = idx
	sess.ModifyIndex = idx

	// Check that the node exists
	node, err := tx.First("nodes", "id", sess.Node)
	if err != nil {
		return fmt.Errorf("failed node lookup: %s", err)
	}
	if node == nil {
		return ErrMissingNode
	}

	// Go over the session checks and ensure they exist.
	for _, checkID := range sess.Checks {
		check, err := tx.First("checks", "id", sess.Node, checkID)
		if err != nil {
			return fmt.Errorf("failed check lookup: %s", err)
		}
		if check == nil {
			return fmt.Errorf("Missing check '%s' registration", checkID)
		}

		// Check that the check is not in critical state
		status := check.(*structs.HealthCheck).Status
		if status == structs.HealthCritical {
			return fmt.Errorf("Check '%s' is in %s state", status)
		}
	}

	// Insert the session
	if err := tx.Insert("sessions", sess); err != nil {
		return fmt.Errorf("failed inserting session: %s", err)
	}

	// Insert the check mappings
	for _, checkID := range sess.Checks {
		check := &sessionCheck{
			Node:    sess.Node,
			CheckID: checkID,
			Session: sess.ID,
		}
		if err := tx.Insert("session_checks", check); err != nil {
			return fmt.Errorf("failed inserting session check mapping: %s", err)
		}
	}

	// Update the index
	if err := tx.Insert("index", &IndexEntry{"sessions", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// GetSession is used to retrieve an active session from the state store.
func (s *StateStore) GetSession(sessionID string) (*structs.Session, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Look up the session by its ID
	session, err := tx.First("sessions", "id", sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed session lookup: %s", err)
	}
	if session != nil {
		return session.(*structs.Session), nil
	}
	return nil, nil
}

// SessionList returns a slice containing all of the active sessions.
func (s *StateStore) SessionList() (uint64, []*structs.Session, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query all of the active sessions
	sessions, err := tx.Get("sessions", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed session lookup: %s", err)
	}

	// Go over the sessions and create a slice of them
	var result []*structs.Session
	var lindex uint64
	for session := sessions.Next(); session != nil; session = sessions.Next() {
		sess := session.(*structs.Session)
		result = append(result, sess)

		// Compute the highest index
		if sess.ModifyIndex > lindex {
			lindex = sess.ModifyIndex
		}
	}
	return lindex, result, nil
}

// NodeSessions returns a set of active sessions associated
// with the given node ID. The returned index is the highest
// index seen from the result set.
func (s *StateStore) NodeSessions(nodeID string) (uint64, []*structs.Session, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get all of the sessions which belong to the node
	sessions, err := tx.Get("sessions", "node", nodeID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed session lookup: %s", err)
	}

	// Go over all of the sessions and return them as a slice
	var result []*structs.Session
	var lindex uint64
	for session := sessions.Next(); session != nil; session = sessions.Next() {
		sess := session.(*structs.Session)
		result = append(result, sess)

		// Compute the highest index
		if sess.ModifyIndex > lindex {
			lindex = sess.ModifyIndex
		}
	}
	return lindex, result, nil
}

// SessionDestroy is used to remove an active session. This will
// implicitly invalidate the session and invoke the specified
// session destroy behavior.
func (s *StateStore) SessionDestroy(idx uint64, sessionID string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the session deletion
	if err := s.sessionDestroyTxn(idx, sessionID, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// sessionDestroyTxn is the inner method, which is used to do the actual
// session deletion and handle session invalidation, watch triggers, etc.
func (s *StateStore) sessionDestroyTxn(idx uint64, sessionID string, tx *memdb.Txn) error {
	// Look up the session
	sess, err := tx.First("sessions", "id", sessionID)
	if err != nil {
		return fmt.Errorf("failed session lookup: %s", err)
	}
	if sess == nil {
		return nil
	}

	// Delete the session and write the new index
	if err := tx.Delete("sessions", sess); err != nil {
		return fmt.Errorf("failed deleting session: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"sessions", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// TODO: invalidate session

	return nil
}

// ACLSet is used to insert an ACL rule into the state store.
func (s *StateStore) ACLSet(idx uint64, acl *structs.ACL) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call set on the ACL
	if err := s.aclSetTxn(idx, acl, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// aclSetTxn is the inner method used to insert an ACL rule with the
// proper indexes into the state store.
func (s *StateStore) aclSetTxn(idx uint64, acl *structs.ACL, tx *memdb.Txn) error {
	// Check that the ID is set
	if acl.ID == "" {
		return ErrMissingACLID
	}

	// Check for an existing ACL
	existing, err := tx.First("acls", "id", acl.ID)
	if err != nil {
		return fmt.Errorf("failed acl lookup: %s", err)
	}

	// Set the indexes
	if existing != nil {
		acl.CreateIndex = existing.(*structs.ACL).CreateIndex
		acl.ModifyIndex = idx
	} else {
		acl.CreateIndex = idx
		acl.ModifyIndex = idx
	}

	// Insert the ACL
	if err := tx.Insert("acls", acl); err != nil {
		return fmt.Errorf("failed inserting acl: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"acls", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// ACLGet is used to look up an existing ACL by ID.
func (s *StateStore) ACLGet(aclID string) (*structs.ACL, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query for the existing ACL
	acl, err := tx.First("acls", "id", aclID)
	if err != nil {
		return nil, fmt.Errorf("failed acl lookup: %s", err)
	}
	if acl != nil {
		return acl.(*structs.ACL), nil
	}
	return nil, nil
}

// ACLDelete is used to remove an existing ACL from the state store. If
// the ACL does not exist this is a no-op and no error is returned.
func (s *StateStore) ACLDelete(idx uint64, aclID string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the ACL delete
	if err := s.aclDeleteTxn(idx, aclID, tx); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// aclDeleteTxn is used to delete an ACL from the state store within
// an existing transaction.
func (s *StateStore) aclDeleteTxn(idx uint64, aclID string, tx *memdb.Txn) error {
	// Look up the existing ACL
	acl, err := tx.First("acls", "id", aclID)
	if err != nil {
		return fmt.Errorf("failed acl lookup: %s", err)
	}
	if acl == nil {
		return nil
	}

	// Delete the ACL from the state store and update indexes
	if err := tx.Delete("acls", acl); err != nil {
		return fmt.Errorf("failed deleting acl: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"acls", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

// ACLList is used to list out all of the ACLs in the state store.
func (s *StateStore) ACLList() (uint64, []*structs.ACL, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query all of the ACLs in the state store
	acls, err := tx.Get("acls", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed acl lookup: %s", err)
	}

	// Go over all of the ACLs and build the response
	var result []*structs.ACL
	var lindex uint64
	for acl := acls.Next(); acl != nil; acl = acls.Next() {
		a := acl.(*structs.ACL)
		result = append(result, a)

		// Accumulate the highest index
		if a.ModifyIndex > lindex {
			lindex = a.ModifyIndex
		}
	}
	return lindex, result, nil
}
