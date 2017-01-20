package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-memdb"
)

// Nodes is used to pull the full list of nodes for use during snapshots.
func (s *StateSnapshot) Nodes() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("nodes", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Services is used to pull the full list of services for a given node for use
// during snapshots.
func (s *StateSnapshot) Services(node string) (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("services", "node", node)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Checks is used to pull the full list of checks for a given node for use
// during snapshots.
func (s *StateSnapshot) Checks(node string) (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("checks", "node", node)
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Registration is used to make sure a node, service, and check registration is
// performed within a single transaction to avoid race conditions on state
// updates.
func (s *StateRestore) Registration(idx uint64, req *structs.RegisterRequest) error {
	if err := s.store.ensureRegistrationTxn(s.tx, idx, s.watches, req); err != nil {
		return err
	}
	return nil
}

// EnsureRegistration is used to make sure a node, service, and check
// registration is performed within a single transaction to avoid race
// conditions on state updates.
func (s *StateStore) EnsureRegistration(idx uint64, req *structs.RegisterRequest) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	watches := NewDumbWatchManager(s.tableWatches)
	if err := s.ensureRegistrationTxn(tx, idx, watches, req); err != nil {
		return err
	}

	tx.Defer(func() { watches.Notify() })
	tx.Commit()
	return nil
}

// ensureRegistrationTxn is used to make sure a node, service, and check
// registration is performed within a single transaction to avoid race
// conditions on state updates.
func (s *StateStore) ensureRegistrationTxn(tx *memdb.Txn, idx uint64, watches *DumbWatchManager,
	req *structs.RegisterRequest) error {
	// Create a node structure.
	node := &structs.Node{
		ID:              req.ID,
		Node:            req.Node,
		Address:         req.Address,
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
			if err := s.ensureNodeTxn(tx, idx, watches, node); err != nil {
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
			if err := s.ensureServiceTxn(tx, idx, watches, req.Node, req.Service); err != nil {
				return fmt.Errorf("failed inserting service: %s", err)

			}
		}
	}

	// TODO (slackpad) In Consul 0.8 ban checks that don't have the same
	// node as the top-level registration. This is just weird to be able to
	// update unrelated nodes' checks from in here. In 0.7.2 we banned this
	// up in the ACL check since that's guarded behind an opt-in flag until
	// Consul 0.8.

	// Add the checks, if any.
	if req.Check != nil {
		if err := s.ensureCheckTxn(tx, idx, watches, req.Check); err != nil {
			return fmt.Errorf("failed inserting check: %s", err)
		}
	}
	for _, check := range req.Checks {
		if err := s.ensureCheckTxn(tx, idx, watches, check); err != nil {
			return fmt.Errorf("failed inserting check: %s", err)
		}
	}

	return nil
}

// EnsureNode is used to upsert node registration or modification.
func (s *StateStore) EnsureNode(idx uint64, node *structs.Node) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the node upsert
	watches := NewDumbWatchManager(s.tableWatches)
	if err := s.ensureNodeTxn(tx, idx, watches, node); err != nil {
		return err
	}

	tx.Defer(func() { watches.Notify() })
	tx.Commit()
	return nil
}

// ensureNodeTxn is the inner function called to actually create a node
// registration or modify an existing one in the state store. It allows
// passing in a memdb transaction so it may be part of a larger txn.
func (s *StateStore) ensureNodeTxn(tx *memdb.Txn, idx uint64, watches *DumbWatchManager,
	node *structs.Node) error {
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

	watches.Arm("nodes")
	return nil
}

// GetNode is used to retrieve a node registration by node ID.
func (s *StateStore) GetNode(id string) (uint64, *structs.Node, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("GetNode")...)

	// Retrieve the node from the state store
	node, err := tx.First("nodes", "id", id)
	if err != nil {
		return 0, nil, fmt.Errorf("node lookup failed: %s", err)
	}
	if node != nil {
		return idx, node.(*structs.Node), nil
	}
	return idx, nil, nil
}

// Nodes is used to return all of the known nodes.
func (s *StateStore) Nodes() (uint64, structs.Nodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("Nodes")...)

	// Retrieve all of the nodes
	nodes, err := tx.Get("nodes", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed nodes lookup: %s", err)
	}

	// Create and return the nodes list.
	var results structs.Nodes
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		results = append(results, node.(*structs.Node))
	}
	return idx, results, nil
}

// NodesByMeta is used to return all nodes with the given metadata key/value pairs.
func (s *StateStore) NodesByMeta(filters map[string]string) (uint64, structs.Nodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("Nodes")...)

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
func (s *StateStore) DeleteNode(idx uint64, nodeName string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the node deletion.
	if err := s.deleteNodeTxn(tx, idx, nodeName); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// deleteNodeTxn is the inner method used for removing a node from
// the store within a given transaction.
func (s *StateStore) deleteNodeTxn(tx *memdb.Txn, idx uint64, nodeName string) error {
	// Look up the node.
	node, err := tx.First("nodes", "id", nodeName)
	if err != nil {
		return fmt.Errorf("node lookup failed: %s", err)
	}
	if node == nil {
		return nil
	}

	// Use a watch manager since the inner functions can perform multiple
	// ops per table.
	watches := NewDumbWatchManager(s.tableWatches)

	// Delete all services associated with the node and update the service index.
	services, err := tx.Get("services", "node", nodeName)
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}
	var sids []string
	for service := services.Next(); service != nil; service = services.Next() {
		sids = append(sids, service.(*structs.ServiceNode).ServiceID)
	}

	// Do the delete in a separate loop so we don't trash the iterator.
	for _, sid := range sids {
		if err := s.deleteServiceTxn(tx, idx, watches, nodeName, sid); err != nil {
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
		if err := s.deleteCheckTxn(tx, idx, watches, nodeName, cid); err != nil {
			return err
		}
	}

	// Delete any coordinate associated with this node.
	coord, err := tx.First("coordinates", "id", nodeName)
	if err != nil {
		return fmt.Errorf("failed coordinate lookup: %s", err)
	}
	if coord != nil {
		if err := tx.Delete("coordinates", coord); err != nil {
			return fmt.Errorf("failed deleting coordinate: %s", err)
		}
		if err := tx.Insert("index", &IndexEntry{"coordinates", idx}); err != nil {
			return fmt.Errorf("failed updating index: %s", err)
		}
		watches.Arm("coordinates")
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
		if err := s.deleteSessionTxn(tx, idx, watches, id); err != nil {
			return fmt.Errorf("failed session delete: %s", err)
		}
	}

	watches.Arm("nodes")
	tx.Defer(func() { watches.Notify() })
	return nil
}

// EnsureService is called to upsert creation of a given NodeService.
func (s *StateStore) EnsureService(idx uint64, node string, svc *structs.NodeService) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the service registration upsert
	watches := NewDumbWatchManager(s.tableWatches)
	if err := s.ensureServiceTxn(tx, idx, watches, node, svc); err != nil {
		return err
	}

	tx.Defer(func() { watches.Notify() })
	tx.Commit()
	return nil
}

// ensureServiceTxn is used to upsert a service registration within an
// existing memdb transaction.
func (s *StateStore) ensureServiceTxn(tx *memdb.Txn, idx uint64, watches *DumbWatchManager,
	node string, svc *structs.NodeService) error {
	// Check for existing service
	existing, err := tx.First("services", "id", node, svc.ID)
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}

	// Create the service node entry and populate the indexes. Note that
	// conversion doesn't populate any of the node-specific information.
	// That's always populated when we read from the state store.
	entry := svc.ToServiceNode(node)
	if existing != nil {
		entry.CreateIndex = existing.(*structs.ServiceNode).CreateIndex
		entry.ModifyIndex = idx
	} else {
		entry.CreateIndex = idx
		entry.ModifyIndex = idx
	}

	// Get the node
	n, err := tx.First("nodes", "id", node)
	if err != nil {
		return fmt.Errorf("failed node lookup: %s", err)
	}
	if n == nil {
		return ErrMissingNode
	}

	// Insert the service and update the index
	if err := tx.Insert("services", entry); err != nil {
		return fmt.Errorf("failed inserting service: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"services", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	watches.Arm("services")
	return nil
}

// Services returns all services along with a list of associated tags.
func (s *StateStore) Services() (uint64, structs.Services, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("Services")...)

	// List all the services.
	services, err := tx.Get("services", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying services: %s", err)
	}

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
		for tag, _ := range tags {
			results[service] = append(results[service], tag)
		}
	}
	return idx, results, nil
}

// ServicesByNodeMeta returns all services, filtered by the given node metadata.
func (s *StateStore) ServicesByNodeMeta(filters map[string]string) (uint64, structs.Services, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("ServiceNodes")...)

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
		for tag, _ := range tags {
			results[service] = append(results[service], tag)
		}
	}
	return idx, results, nil
}

// ServiceNodes returns the nodes associated with a given service name.
func (s *StateStore) ServiceNodes(serviceName string) (uint64, structs.ServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("ServiceNodes")...)

	// List all the services.
	services, err := tx.Get("services", "service", serviceName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}
	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		results = append(results, service.(*structs.ServiceNode))
	}

	// Fill in the address details.
	results, err = s.parseServiceNodes(tx, results)
	if err != nil {
		return 0, nil, fmt.Errorf("failed parsing service nodes: %s", err)
	}
	return idx, results, nil
}

// ServiceTagNodes returns the nodes associated with a given service, filtering
// out services that don't contain the given tag.
func (s *StateStore) ServiceTagNodes(service, tag string) (uint64, structs.ServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("ServiceNodes")...)

	// List all the services.
	services, err := tx.Get("services", "service", service)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}

	// Gather all the services and apply the tag filter.
	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		if !serviceTagFilter(svc, tag) {
			results = append(results, svc)
		}
	}

	// Fill in the address details.
	results, err = s.parseServiceNodes(tx, results)
	if err != nil {
		return 0, nil, fmt.Errorf("failed parsing service nodes: %s", err)
	}
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

// parseServiceNodes iterates over a services query and fills in the node details,
// returning a ServiceNodes slice.
func (s *StateStore) parseServiceNodes(tx *memdb.Txn, services structs.ServiceNodes) (structs.ServiceNodes, error) {
	var results structs.ServiceNodes
	for _, sn := range services {
		// Note that we have to clone here because we don't want to
		// modify the node-related fields on the object in the database,
		// which is what we are referencing.
		s := sn.PartialClone()

		// Grab the corresponding node record.
		n, err := tx.First("nodes", "id", sn.Node)
		if err != nil {
			return nil, fmt.Errorf("failed node lookup: %s", err)
		}

		// Populate the node-related fields. The tagged addresses may be
		// used by agents to perform address translation if they are
		// configured to do that.
		node := n.(*structs.Node)
		s.ID = node.ID
		s.Address = node.Address
		s.TaggedAddresses = node.TaggedAddresses
		s.NodeMeta = node.Meta

		results = append(results, s)
	}
	return results, nil
}

// NodeService is used to retrieve a specific service associated with the given
// node.
func (s *StateStore) NodeService(nodeName string, serviceID string) (uint64, *structs.NodeService, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("NodeService")...)

	// Query the service
	service, err := tx.First("services", "id", nodeName, serviceID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying service for node %q: %s", nodeName, err)
	}

	if service != nil {
		return idx, service.(*structs.ServiceNode).ToNodeService(), nil
	} else {
		return idx, nil, nil
	}
}

// NodeServices is used to query service registrations by node ID.
func (s *StateStore) NodeServices(nodeName string) (uint64, *structs.NodeServices, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("NodeServices")...)

	// Query the node
	n, err := tx.First("nodes", "id", nodeName)
	if err != nil {
		return 0, nil, fmt.Errorf("node lookup failed: %s", err)
	}
	if n == nil {
		return 0, nil, nil
	}
	node := n.(*structs.Node)

	// Read all of the services
	services, err := tx.Get("services", "node", nodeName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed querying services for node %q: %s", nodeName, err)
	}

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
func (s *StateStore) DeleteService(idx uint64, nodeName, serviceID string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the service deletion
	watches := NewDumbWatchManager(s.tableWatches)
	if err := s.deleteServiceTxn(tx, idx, watches, nodeName, serviceID); err != nil {
		return err
	}

	tx.Defer(func() { watches.Notify() })
	tx.Commit()
	return nil
}

// deleteServiceTxn is the inner method called to remove a service
// registration within an existing transaction.
func (s *StateStore) deleteServiceTxn(tx *memdb.Txn, idx uint64, watches *DumbWatchManager, nodeName, serviceID string) error {
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
		if err := s.deleteCheckTxn(tx, idx, watches, nodeName, cid); err != nil {
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

	watches.Arm("services")
	return nil
}

// EnsureCheck is used to store a check registration in the db.
func (s *StateStore) EnsureCheck(idx uint64, hc *structs.HealthCheck) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the check registration
	watches := NewDumbWatchManager(s.tableWatches)
	if err := s.ensureCheckTxn(tx, idx, watches, hc); err != nil {
		return err
	}

	tx.Defer(func() { watches.Notify() })
	tx.Commit()
	return nil
}

// ensureCheckTransaction is used as the inner method to handle inserting
// a health check into the state store. It ensures safety against inserting
// checks with no matching node or service.
func (s *StateStore) ensureCheckTxn(tx *memdb.Txn, idx uint64, watches *DumbWatchManager,
	hc *structs.HealthCheck) error {
	// Check if we have an existing health check
	existing, err := tx.First("checks", "id", hc.Node, string(hc.CheckID))
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

	// Delete any sessions for this check if the health is critical.
	if hc.Status == structs.HealthCritical {
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
		watches := NewDumbWatchManager(s.tableWatches)
		for _, id := range ids {
			if err := s.deleteSessionTxn(tx, idx, watches, id); err != nil {
				return fmt.Errorf("failed deleting session: %s", err)
			}
		}
		tx.Defer(func() { watches.Notify() })
	}

	// Persist the check registration in the db.
	if err := tx.Insert("checks", hc); err != nil {
		return fmt.Errorf("failed inserting check: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"checks", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	watches.Arm("checks")
	return nil
}

// NodeCheck is used to retrieve a specific check associated with the given
// node.
func (s *StateStore) NodeCheck(nodeName string, checkID types.CheckID) (uint64, *structs.HealthCheck, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("NodeCheck")...)

	// Return the check.
	check, err := tx.First("checks", "id", nodeName, string(checkID))
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	if check != nil {
		return idx, check.(*structs.HealthCheck), nil
	} else {
		return idx, nil, nil
	}
}

// NodeChecks is used to retrieve checks associated with the
// given node from the state store.
func (s *StateStore) NodeChecks(nodeName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("NodeChecks")...)

	// Return the checks.
	checks, err := tx.Get("checks", "node", nodeName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	return s.parseChecks(idx, checks)
}

// ServiceChecks is used to get all checks associated with a
// given service ID. The query is performed against a service
// _name_ instead of a service ID.
func (s *StateStore) ServiceChecks(serviceName string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("ServiceChecks")...)

	// Return the checks.
	checks, err := tx.Get("checks", "service", serviceName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	return s.parseChecks(idx, checks)
}

// ServiceChecksByNodeMeta is used to get all checks associated with a
// given service ID, filtered by the given node metadata values. The query
// is performed against a service _name_ instead of a service ID.
func (s *StateStore) ServiceChecksByNodeMeta(serviceName string, filters map[string]string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("ServiceChecksByNodeMeta")...)

	// Return the checks.
	checks, err := tx.Get("checks", "service", serviceName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	return s.parseChecksByNodeMeta(idx, checks, tx, filters)
}

// ChecksInState is used to query the state store for all checks
// which are in the provided state.
func (s *StateStore) ChecksInState(state string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("ChecksInState")...)

	// Query all checks if HealthAny is passed
	if state == structs.HealthAny {
		checks, err := tx.Get("checks", "status")
		if err != nil {
			return 0, nil, fmt.Errorf("failed check lookup: %s", err)
		}
		return s.parseChecks(idx, checks)
	}

	// Any other state we need to query for explicitly
	checks, err := tx.Get("checks", "status", state)
	if err != nil {
		return 0, nil, fmt.Errorf("failed check lookup: %s", err)
	}
	return s.parseChecks(idx, checks)
}

// ChecksInStateByNodeMeta is used to query the state store for all checks
// which are in the provided state, filtered by the given node metadata values.
func (s *StateStore) ChecksInStateByNodeMeta(state string, filters map[string]string) (uint64, structs.HealthChecks, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("ChecksInStateByNodeMeta")...)

	// Query all checks if HealthAny is passed
	var checks memdb.ResultIterator
	var err error
	if state == structs.HealthAny {
		checks, err = tx.Get("checks", "status")
		if err != nil {
			return 0, nil, fmt.Errorf("failed check lookup: %s", err)
		}
	} else {
		// Any other state we need to query for explicitly
		checks, err = tx.Get("checks", "status", state)
		if err != nil {
			return 0, nil, fmt.Errorf("failed check lookup: %s", err)
		}
	}

	return s.parseChecksByNodeMeta(idx, checks, tx, filters)
}

// parseChecks is a helper function used to deduplicate some
// repetitive code for returning health checks.
func (s *StateStore) parseChecks(idx uint64, iter memdb.ResultIterator) (uint64, structs.HealthChecks, error) {
	// Gather the health checks and return them properly type casted.
	var results structs.HealthChecks
	for check := iter.Next(); check != nil; check = iter.Next() {
		results = append(results, check.(*structs.HealthCheck))
	}
	return idx, results, nil
}

// parseChecksByNodeMeta is a helper function used to deduplicate some
// repetitive code for returning health checks filtered by node metadata fields.
func (s *StateStore) parseChecksByNodeMeta(idx uint64, iter memdb.ResultIterator, tx *memdb.Txn,
	filters map[string]string) (uint64, structs.HealthChecks, error) {
	var results structs.HealthChecks
	for check := iter.Next(); check != nil; check = iter.Next() {
		healthCheck := check.(*structs.HealthCheck)
		node, err := tx.First("nodes", "id", healthCheck.Node)
		if err != nil {
			return 0, nil, fmt.Errorf("failed node lookup: %s", err)
		}
		if node == nil {
			return 0, nil, ErrMissingNode
		}
		if structs.SatisfiesMetaFilters(node.(*structs.Node).Meta, filters) {
			results = append(results, healthCheck)
		}
	}
	return idx, results, nil
}

// DeleteCheck is used to delete a health check registration.
func (s *StateStore) DeleteCheck(idx uint64, node string, checkID types.CheckID) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call the check deletion
	watches := NewDumbWatchManager(s.tableWatches)
	if err := s.deleteCheckTxn(tx, idx, watches, node, checkID); err != nil {
		return err
	}

	tx.Defer(func() { watches.Notify() })
	tx.Commit()
	return nil
}

// deleteCheckTxn is the inner method used to call a health
// check deletion within an existing transaction.
func (s *StateStore) deleteCheckTxn(tx *memdb.Txn, idx uint64, watches *DumbWatchManager, node string, checkID types.CheckID) error {
	// Try to retrieve the existing health check.
	hc, err := tx.First("checks", "id", node, string(checkID))
	if err != nil {
		return fmt.Errorf("check lookup failed: %s", err)
	}
	if hc == nil {
		return nil
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
		if err := s.deleteSessionTxn(tx, idx, watches, id); err != nil {
			return fmt.Errorf("failed deleting session: %s", err)
		}
	}

	watches.Arm("checks")
	return nil
}

// CheckServiceNodes is used to query all nodes and checks for a given service.
func (s *StateStore) CheckServiceNodes(serviceName string) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("CheckServiceNodes")...)

	// Query the state store for the service.
	services, err := tx.Get("services", "service", serviceName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}

	// Return the results.
	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		results = append(results, service.(*structs.ServiceNode))
	}
	return s.parseCheckServiceNodes(tx, idx, results, err)
}

// CheckServiceTagNodes is used to query all nodes and checks for a given
// service, filtering out services that don't contain the given tag.
func (s *StateStore) CheckServiceTagNodes(serviceName, tag string) (uint64, structs.CheckServiceNodes, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("CheckServiceNodes")...)

	// Query the state store for the service.
	services, err := tx.Get("services", "service", serviceName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed service lookup: %s", err)
	}

	// Return the results, filtering by tag.
	var results structs.ServiceNodes
	for service := services.Next(); service != nil; service = services.Next() {
		svc := service.(*structs.ServiceNode)
		if !serviceTagFilter(svc, tag) {
			results = append(results, svc)
		}
	}
	return s.parseCheckServiceNodes(tx, idx, results, err)
}

// parseCheckServiceNodes is used to parse through a given set of services,
// and query for an associated node and a set of checks. This is the inner
// method used to return a rich set of results from a more simple query.
func (s *StateStore) parseCheckServiceNodes(
	tx *memdb.Txn, idx uint64, services structs.ServiceNodes,
	err error) (uint64, structs.CheckServiceNodes, error) {
	if err != nil {
		return 0, nil, err
	}

	// Special-case the zero return value to nil, since this ends up in
	// external APIs.
	if len(services) == 0 {
		return idx, nil, nil
	}

	results := make(structs.CheckServiceNodes, 0, len(services))
	for _, sn := range services {
		// Retrieve the node.
		n, err := tx.First("nodes", "id", sn.Node)
		if err != nil {
			return 0, nil, fmt.Errorf("failed node lookup: %s", err)
		}
		if n == nil {
			return 0, nil, ErrMissingNode
		}
		node := n.(*structs.Node)

		// We need to return the checks specific to the given service
		// as well as the node itself. Unfortunately, memdb won't let
		// us use the index to do the latter query so we have to pull
		// them all and filter.
		var checks structs.HealthChecks
		iter, err := tx.Get("checks", "node", sn.Node)
		if err != nil {
			return 0, nil, err
		}
		for check := iter.Next(); check != nil; check = iter.Next() {
			hc := check.(*structs.HealthCheck)
			if hc.ServiceID == "" || hc.ServiceID == sn.ServiceID {
				checks = append(checks, hc)
			}
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
func (s *StateStore) NodeInfo(node string) (uint64, structs.NodeDump, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("NodeInfo")...)

	// Query the node by the passed node
	nodes, err := tx.Get("nodes", "id", node)
	if err != nil {
		return 0, nil, fmt.Errorf("failed node lookup: %s", err)
	}
	return s.parseNodes(tx, idx, nodes)
}

// NodeDump is used to generate a dump of all nodes. This call is expensive
// as it has to query every node, service, and check. The response can also
// be quite large since there is currently no filtering applied.
func (s *StateStore) NodeDump() (uint64, structs.NodeDump, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("NodeDump")...)

	// Fetch all of the registered nodes
	nodes, err := tx.Get("nodes", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed node lookup: %s", err)
	}
	return s.parseNodes(tx, idx, nodes)
}

// parseNodes takes an iterator over a set of nodes and returns a struct
// containing the nodes along with all of their associated services
// and/or health checks.
func (s *StateStore) parseNodes(tx *memdb.Txn, idx uint64,
	iter memdb.ResultIterator) (uint64, structs.NodeDump, error) {

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
		for service := services.Next(); service != nil; service = services.Next() {
			ns := service.(*structs.ServiceNode).ToNodeService()
			dump.Services = append(dump.Services, ns)
		}

		// Query the node checks
		checks, err := tx.Get("checks", "node", node.Node)
		if err != nil {
			return 0, nil, fmt.Errorf("failed node lookup: %s", err)
		}
		for check := checks.Next(); check != nil; check = checks.Next() {
			hc := check.(*structs.HealthCheck)
			dump.Checks = append(dump.Checks, hc)
		}

		// Add the result to the slice
		results = append(results, dump)
	}
	return idx, results, nil
}
