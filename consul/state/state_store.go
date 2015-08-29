package state

import (
	"errors"
	"fmt"
	"io"
	"log"

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
		idx := ti.(*IndexEntry).Value
		if idx > lindex {
			lindex = idx
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
func (s *StateStore) Nodes() (structs.Nodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Retrieve all of the nodes
	nodes, err := tx.Get("nodes", "id")
	if err != nil {
		return nil, fmt.Errorf("failed nodes lookup: %s", err)
	}

	// Create and return the nodes list.
	// TODO: Optimize by returning an iterator.
	var results structs.Nodes
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		results = append(results, node.(*structs.Node))
	}
	return results, nil
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
		entry.CreateIndex = existing.(*structs.NodeService).CreateIndex
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
func (s *StateStore) NodeServices(nodeID string) (*structs.NodeServices, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Query the node
	n, err := tx.First("nodes", "id", nodeID)
	if err != nil {
		return nil, fmt.Errorf("node lookup failed: %s", err)
	}
	if n == nil {
		return nil, nil
	}
	node := n.(*structs.Node)

	// Read all of the services
	services, err := tx.Get("services", "node", nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed querying services for node %q: %s", nodeID, err)
	}

	// Initialize the node services struct
	ns := &structs.NodeServices{
		Node:     node,
		Services: make(map[string]*structs.NodeService),
	}
	ns.CreateIndex = node.CreateIndex
	ns.CreateIndex = node.CreateIndex

	// Add all of the services to the map
	for service := services.Next(); service != nil; service = services.Next() {
		sn := service.(*structs.ServiceNode)

		// Track the highest index
		if sn.CreateIndex > ns.CreateIndex {
			ns.CreateIndex = sn.CreateIndex
		}
		if sn.ModifyIndex > ns.ModifyIndex {
			ns.ModifyIndex = sn.ModifyIndex
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

	return ns, nil
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
func (s *StateStore) NodeChecks(nodeID string) (structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return s.parseChecks(tx.Get("checks", "node", nodeID))
}

// parseChecks is a helper function used to deduplicate some
// repetitive code for returning health checks.
func (s *StateStore) parseChecks(iter memdb.ResultIterator, err error) (structs.HealthChecks, error) {
	if err != nil {
		return nil, fmt.Errorf("failed health check lookup: %s", err)
	}

	// Gather the health checks and return them properly type casted
	var results structs.HealthChecks
	for hc := iter.Next(); hc != nil; hc = iter.Next() {
		results = append(results, hc.(*structs.HealthCheck))
	}
	return results, nil
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
