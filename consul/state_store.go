package consul

import (
	"fmt"
	"github.com/armon/gomdb"
	"github.com/hashicorp/consul/consul/structs"
	"io"
	"io/ioutil"
	"log"
	"os"
)

const (
	dbNodes      = "nodes"
	dbServices   = "services"
	dbChecks     = "checks"
	dbMaxMapSize = 1024 * 1024 * 1024 // 1GB maximum size
)

// The StateStore is responsible for maintaining all the Consul
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads. The current
// implementation uses the Lightning Memory-Mapped Database (MDB).
// This gives us Multi-Version Concurrency Control for "free"
type StateStore struct {
	logger       *log.Logger
	path         string
	env          *mdb.Env
	nodeTable    *MDBTable
	serviceTable *MDBTable
	checkTable   *MDBTable
	tables       MDBTables
	watch        map[*MDBTable]*NotifyGroup
	queryTables  map[string]MDBTables
}

// StateSnapshot is used to provide a point-in-time snapshot
// It works by starting a readonly transaction against all tables.
type StateSnapshot struct {
	store     *StateStore
	tx        *MDBTxn
	lastIndex uint64
}

// Close is used to abort the transaction and allow for cleanup
func (s *StateSnapshot) Close() error {
	s.tx.Abort()
	return nil
}

// NewStateStore is used to create a new state store
func NewStateStore(logOutput io.Writer) (*StateStore, error) {
	// Create a new temp dir
	path, err := ioutil.TempDir("", "consul")
	if err != nil {
		return nil, err
	}

	// Open the env
	env, err := mdb.NewEnv()
	if err != nil {
		return nil, err
	}

	s := &StateStore{
		logger: log.New(logOutput, "", log.LstdFlags),
		path:   path,
		env:    env,
		watch:  make(map[*MDBTable]*NotifyGroup),
	}

	// Ensure we can initialize
	if err := s.initialize(); err != nil {
		env.Close()
		os.RemoveAll(path)
		return nil, err
	}
	return s, nil
}

// Close is used to safely shutdown the state store
func (s *StateStore) Close() error {
	s.env.Close()
	os.RemoveAll(s.path)
	return nil
}

// initialize is used to setup the store for use
func (s *StateStore) initialize() error {
	// Setup the Env first
	if err := s.env.SetMaxDBs(mdb.DBI(32)); err != nil {
		return err
	}

	// Increase the maximum map size
	if err := s.env.SetMapSize(dbMaxMapSize); err != nil {
		return err
	}

	// Optimize our flags for speed over safety, since the Raft log + snapshots
	// are durable. We treat this as an ephemeral in-memory DB, since we nuke
	// the data anyways.
	var flags uint = mdb.NOMETASYNC | mdb.NOSYNC | mdb.NOTLS
	if err := s.env.Open(s.path, flags, 0755); err != nil {
		return err
	}

	// Tables use a generic struct encoder
	encoder := func(obj interface{}) []byte {
		buf, err := structs.Encode(255, obj)
		if err != nil {
			panic(err)
		}
		return buf[1:]
	}

	// Setup our tables
	s.nodeTable = &MDBTable{
		Name: dbNodes,
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Node"},
			},
		},
		Decoder: func(buf []byte) interface{} {
			out := new(structs.Node)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}

	s.serviceTable = &MDBTable{
		Name: dbServices,
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Node", "ServiceID"},
			},
			"service": &MDBIndex{
				AllowBlank: true,
				Fields:     []string{"ServiceName", "ServiceTag"},
			},
		},
		Decoder: func(buf []byte) interface{} {
			out := new(structs.ServiceNode)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}

	s.checkTable = &MDBTable{
		Name: dbChecks,
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Node", "CheckID"},
			},
			"status": &MDBIndex{
				Fields: []string{"Status"},
			},
			"service": &MDBIndex{
				AllowBlank: true,
				Fields:     []string{"ServiceName"},
			},
			"node": &MDBIndex{
				AllowBlank: true,
				Fields:     []string{"Node", "ServiceID"},
			},
		},
		Decoder: func(buf []byte) interface{} {
			out := new(structs.HealthCheck)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}

	// Store the set of tables
	s.tables = []*MDBTable{s.nodeTable, s.serviceTable, s.checkTable}
	for _, table := range s.tables {
		table.Env = s.env
		table.Encoder = encoder
		if err := table.Init(); err != nil {
			return err
		}

		// Setup a notification group per table
		s.watch[table] = &NotifyGroup{}
	}

	// Setup the query tables
	s.queryTables = map[string]MDBTables{
		"Nodes":             MDBTables{s.nodeTable},
		"Services":          MDBTables{s.serviceTable},
		"ServiceNodes":      MDBTables{s.nodeTable, s.serviceTable},
		"NodeServices":      MDBTables{s.nodeTable, s.serviceTable},
		"ChecksInState":     MDBTables{s.checkTable},
		"NodeChecks":        MDBTables{s.checkTable},
		"ServiceChecks":     MDBTables{s.checkTable},
		"CheckServiceNodes": MDBTables{s.nodeTable, s.serviceTable, s.checkTable},
	}
	return nil
}

// Watch is used to subscribe a channel to a set of MDBTables
func (s *StateStore) Watch(tables MDBTables, notify chan struct{}) {
	for _, t := range tables {
		s.watch[t].Wait(notify)
	}
}

// QueryTables returns the Tables that are queried for a given query
func (s *StateStore) QueryTables(q string) MDBTables {
	return s.queryTables[q]
}

// EnsureNode is used to ensure a given node exists, with the provided address
func (s *StateStore) EnsureNode(index uint64, node structs.Node) error {
	// Start a new txn
	tx, err := s.nodeTable.StartTxn(false, nil)
	if err != nil {
		return err
	}
	defer tx.Abort()

	if err := s.nodeTable.InsertTxn(tx, node); err != nil {
		return err
	}
	if err := s.nodeTable.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	defer s.watch[s.nodeTable].Notify()
	return tx.Commit()
}

// GetNode returns all the address of the known and if it was found
func (s *StateStore) GetNode(name string) (uint64, bool, string) {
	idx, res, err := s.nodeTable.Get("id", name)
	if err != nil {
		panic(fmt.Errorf("Failed to get node: %v", err))
	}
	if len(res) == 0 {
		return idx, false, ""
	}
	return idx, true, res[0].(*structs.Node).Address
}

// GetNodes returns all the known nodes, the slice alternates between
// the node name and address
func (s *StateStore) Nodes() (uint64, structs.Nodes) {
	idx, res, err := s.nodeTable.Get("id")
	if err != nil {
		panic(fmt.Errorf("Failed to get nodes: %v", err))
	}
	results := make([]structs.Node, len(res))
	for i, r := range res {
		results[i] = *r.(*structs.Node)
	}
	return idx, results
}

// EnsureService is used to ensure a given node exposes a service
func (s *StateStore) EnsureService(index uint64, node string, ns *structs.NodeService) error {
	tables := MDBTables{s.nodeTable, s.serviceTable}
	tx, err := tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	// Ensure the node exists
	res, err := s.nodeTable.GetTxn(tx, "id", node)
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return fmt.Errorf("Missing node registration")
	}

	// Create the entry
	entry := structs.ServiceNode{
		Node:        node,
		ServiceID:   ns.ID,
		ServiceName: ns.Service,
		ServiceTag:  ns.Tag,
		ServicePort: ns.Port,
	}

	// Ensure the service entry is set
	if err := s.serviceTable.InsertTxn(tx, &entry); err != nil {
		return err
	}
	if err := s.serviceTable.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	defer s.watch[s.serviceTable].Notify()
	return tx.Commit()
}

// NodeServices is used to return all the services of a given node
func (s *StateStore) NodeServices(name string) (uint64, *structs.NodeServices) {
	tables := s.queryTables["NodeServices"]
	tx, err := tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()
	return s.parseNodeServices(tables, tx, name)
}

// parseNodeServices is used to get the services belonging to a
// node, using a given txn
func (s *StateStore) parseNodeServices(tables MDBTables, tx *MDBTxn, name string) (uint64, *structs.NodeServices) {
	ns := &structs.NodeServices{
		Services: make(map[string]*structs.NodeService),
	}

	// Get the maximum index
	index, err := tables.LastIndexTxn(tx)
	if err != nil {
		panic(fmt.Errorf("Failed to get last index: %v", err))
	}

	// Get the node first
	res, err := s.nodeTable.GetTxn(tx, "id", name)
	if err != nil {
		panic(fmt.Errorf("Failed to get node: %v", err))
	}
	if len(res) == 0 {
		return index, ns
	}

	// Set the address
	node := res[0].(*structs.Node)
	ns.Node = *node

	// Get the services
	res, err = s.serviceTable.GetTxn(tx, "id", name)
	if err != nil {
		panic(fmt.Errorf("Failed to get node: %v", err))
	}

	// Add each service
	for _, r := range res {
		service := r.(*structs.ServiceNode)
		srv := &structs.NodeService{
			ID:      service.ServiceID,
			Service: service.ServiceName,
			Tag:     service.ServiceTag,
			Port:    service.ServicePort,
		}
		ns.Services[srv.ID] = srv
	}
	return index, ns
}

// DeleteNodeService is used to delete a node service
func (s *StateStore) DeleteNodeService(index uint64, node, id string) error {
	tables := MDBTables{s.serviceTable, s.checkTable}
	tx, err := tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	if n, err := s.serviceTable.DeleteTxn(tx, "id", node, id); err != nil {
		return err
	} else if n > 0 {
		if err := s.serviceTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		defer s.watch[s.serviceTable].Notify()
	}
	if n, err := s.checkTable.DeleteTxn(tx, "node", node, id); err != nil {
		return err
	} else if n > 0 {
		if err := s.checkTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		defer s.watch[s.checkTable].Notify()
	}
	return tx.Commit()
}

// DeleteNode is used to delete a node and all it's services
func (s *StateStore) DeleteNode(index uint64, node string) error {
	tables := MDBTables{s.nodeTable, s.serviceTable, s.checkTable}
	tx, err := tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	if n, err := s.serviceTable.DeleteTxn(tx, "id", node); err != nil {
		return err
	} else if n > 0 {
		if err := s.serviceTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		defer s.watch[s.serviceTable].Notify()
	}
	if n, err := s.checkTable.DeleteTxn(tx, "id", node); err != nil {
		return err
	} else if n > 0 {
		if err := s.checkTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		defer s.watch[s.checkTable].Notify()
	}
	if n, err := s.nodeTable.DeleteTxn(tx, "id", node); err != nil {
		return err
	} else if n > 0 {
		if err := s.nodeTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		defer s.watch[s.nodeTable].Notify()
	}
	return tx.Commit()
}

// Services is used to return all the services with a list of associated tags
func (s *StateStore) Services() (uint64, map[string][]string) {
	// TODO: Optimize to not table scan.. We can do a distinct
	// type of query to avoid this
	idx, res, err := s.serviceTable.Get("id")
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	services := make(map[string][]string)
	for _, r := range res {
		srv := r.(*structs.ServiceNode)

		tags := services[srv.ServiceName]
		if !strContains(tags, srv.ServiceTag) {
			tags = append(tags, srv.ServiceTag)
			services[srv.ServiceName] = tags
		}
	}
	return idx, services
}

// ServiceNodes returns the nodes associated with a given service
func (s *StateStore) ServiceNodes(service string) (uint64, structs.ServiceNodes) {
	tables := s.queryTables["ServiceNodes"]
	tx, err := tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	idx, err := tables.LastIndexTxn(tx)
	if err != nil {
		panic(fmt.Errorf("Failed to get last index: %v", err))
	}

	res, err := s.serviceTable.GetTxn(tx, "service", service)
	return idx, parseServiceNodes(tx, s.nodeTable, res, err)
}

// ServiceTagNodes returns the nodes associated with a given service matching a tag
func (s *StateStore) ServiceTagNodes(service, tag string) (uint64, structs.ServiceNodes) {
	tables := s.queryTables["ServiceNodes"]
	tx, err := tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	idx, err := tables.LastIndexTxn(tx)
	if err != nil {
		panic(fmt.Errorf("Failed to get last index: %v", err))
	}

	res, err := s.serviceTable.GetTxn(tx, "service", service, tag)
	return idx, parseServiceNodes(tx, s.nodeTable, res, err)
}

// parseServiceNodes parses results ServiceNodes and ServiceTagNodes
func parseServiceNodes(tx *MDBTxn, table *MDBTable, res []interface{}, err error) structs.ServiceNodes {
	if err != nil {
		panic(fmt.Errorf("Failed to get node services: %v", err))
	}

	nodes := make(structs.ServiceNodes, len(res))
	for i, r := range res {
		srv := r.(*structs.ServiceNode)

		// Get the address of the node
		nodeRes, err := table.GetTxn(tx, "id", srv.Node)
		if err != nil || len(nodeRes) != 1 {
			panic(fmt.Errorf("Failed to join node: %v", err))
		}
		srv.Address = nodeRes[0].(*structs.Node).Address

		nodes[i] = *srv
	}

	return nodes
}

// EnsureCheck is used to create a check or updates it's state
func (s *StateStore) EnsureCheck(index uint64, check *structs.HealthCheck) error {
	// Ensure we have a status
	if check.Status == "" {
		check.Status = structs.HealthUnknown
	}

	// Start the txn
	tables := MDBTables{s.nodeTable, s.serviceTable, s.checkTable}
	tx, err := tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	// Ensure the node exists
	res, err := s.nodeTable.GetTxn(tx, "id", check.Node)
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return fmt.Errorf("Missing node registration")
	}

	// Ensure the service exists if specified
	if check.ServiceID != "" {
		res, err = s.serviceTable.GetTxn(tx, "id", check.Node, check.ServiceID)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return fmt.Errorf("Missing service registration")
		}
		// Ensure we set the correct service
		srv := res[0].(*structs.ServiceNode)
		check.ServiceName = srv.ServiceName
	}

	// Ensure the check is set
	if err := s.checkTable.InsertTxn(tx, check); err != nil {
		return err
	}
	if err := s.checkTable.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	defer s.watch[s.checkTable].Notify()
	return tx.Commit()
}

// DeleteNodeCheck is used to delete a node health check
func (s *StateStore) DeleteNodeCheck(index uint64, node, id string) error {
	tx, err := s.checkTable.StartTxn(false, nil)
	if err != nil {
		return err
	}
	defer tx.Abort()

	if n, err := s.checkTable.DeleteTxn(tx, "id", node, id); err != nil {
		return err
	} else if n > 0 {
		if err := s.checkTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		defer s.watch[s.checkTable].Notify()
	}
	return tx.Commit()
}

// NodeChecks is used to get all the checks for a node
func (s *StateStore) NodeChecks(node string) (uint64, structs.HealthChecks) {
	return parseHealthChecks(s.checkTable.Get("id", node))
}

// ServiceChecks is used to get all the checks for a service
func (s *StateStore) ServiceChecks(service string) (uint64, structs.HealthChecks) {
	return parseHealthChecks(s.checkTable.Get("service", service))
}

// CheckInState is used to get all the checks for a service in a given state
func (s *StateStore) ChecksInState(state string) (uint64, structs.HealthChecks) {
	return parseHealthChecks(s.checkTable.Get("status", state))
}

// parseHealthChecks is used to handle the resutls of a Get against
// the checkTable
func parseHealthChecks(idx uint64, res []interface{}, err error) (uint64, structs.HealthChecks) {
	if err != nil {
		panic(fmt.Errorf("Failed to get checks: %v", err))
	}
	results := make([]*structs.HealthCheck, len(res))
	for i, r := range res {
		results[i] = r.(*structs.HealthCheck)
	}
	return idx, results
}

// CheckServiceNodes returns the nodes associated with a given service, along
// with any associated check
func (s *StateStore) CheckServiceNodes(service string) (uint64, structs.CheckServiceNodes) {
	tables := s.queryTables["CheckServiceNodes"]
	tx, err := tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	idx, err := tables.LastIndexTxn(tx)
	if err != nil {
		panic(fmt.Errorf("Failed to get last index: %v", err))
	}

	res, err := s.serviceTable.GetTxn(tx, "service", service)
	return idx, s.parseCheckServiceNodes(tx, res, err)
}

// CheckServiceNodes returns the nodes associated with a given service, along
// with any associated checks
func (s *StateStore) CheckServiceTagNodes(service, tag string) (uint64, structs.CheckServiceNodes) {
	tables := s.queryTables["CheckServiceNodes"]
	tx, err := tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	idx, err := tables.LastIndexTxn(tx)
	if err != nil {
		panic(fmt.Errorf("Failed to get last index: %v", err))
	}

	res, err := s.serviceTable.GetTxn(tx, "service", service, tag)
	return idx, s.parseCheckServiceNodes(tx, res, err)
}

// parseCheckServiceNodes parses results CheckServiceNodes and CheckServiceTagNodes
func (s *StateStore) parseCheckServiceNodes(tx *MDBTxn, res []interface{}, err error) structs.CheckServiceNodes {
	if err != nil {
		panic(fmt.Errorf("Failed to get node services: %v", err))
	}

	nodes := make(structs.CheckServiceNodes, len(res))
	for i, r := range res {
		srv := r.(*structs.ServiceNode)

		// Get the node
		nodeRes, err := s.nodeTable.GetTxn(tx, "id", srv.Node)
		if err != nil || len(nodeRes) != 1 {
			panic(fmt.Errorf("Failed to join node: %v", err))
		}

		// Get any associated checks of the service
		res, err := s.checkTable.GetTxn(tx, "node", srv.Node, srv.ServiceID)
		_, checks := parseHealthChecks(0, res, err)

		// Get any checks of the node, not assciated with any service
		res, err = s.checkTable.GetTxn(tx, "node", srv.Node, "")
		_, nodeChecks := parseHealthChecks(0, res, err)
		checks = append(checks, nodeChecks...)

		// Setup the node
		nodes[i].Node = *nodeRes[0].(*structs.Node)
		nodes[i].Service = structs.NodeService{
			ID:      srv.ServiceID,
			Service: srv.ServiceName,
			Tag:     srv.ServiceTag,
			Port:    srv.ServicePort,
		}
		nodes[i].Checks = checks
	}

	return nodes
}

// Snapshot is used to create a point in time snapshot
func (s *StateStore) Snapshot() (*StateSnapshot, error) {
	// Begin a new txn on all tables
	tx, err := s.tables.StartTxn(true)
	if err != nil {
		return nil, err
	}

	// Determine the max index
	index, err := s.tables.LastIndexTxn(tx)
	if err != nil {
		tx.Abort()
		return nil, err
	}

	// Return the snapshot
	snap := &StateSnapshot{
		store:     s,
		tx:        tx,
		lastIndex: index,
	}
	return snap, nil
}

// LastIndex returns the last index that affects the snapshotted data
func (s *StateSnapshot) LastIndex() uint64 {
	return s.lastIndex
}

// Nodes returns all the known nodes, the slice alternates between
// the node name and address
func (s *StateSnapshot) Nodes() structs.Nodes {
	res, err := s.store.nodeTable.GetTxn(s.tx, "id")
	if err != nil {
		panic(fmt.Errorf("Failed to get nodes: %v", err))
	}
	results := make([]structs.Node, len(res))
	for i, r := range res {
		results[i] = *r.(*structs.Node)
	}
	return results
}

// NodeServices is used to return all the services of a given node
func (s *StateSnapshot) NodeServices(name string) *structs.NodeServices {
	_, res := s.store.parseNodeServices(s.store.tables, s.tx, name)
	return res
}

// NodeChecks is used to return all the checks of a given node
func (s *StateSnapshot) NodeChecks(node string) structs.HealthChecks {
	res, err := s.store.checkTable.GetTxn(s.tx, "id", node)
	_, checks := parseHealthChecks(s.lastIndex, res, err)
	return checks
}
