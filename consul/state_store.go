package consul

import (
	"fmt"
	"github.com/armon/gomdb"
	"github.com/hashicorp/consul/consul/structs"
	"io/ioutil"
	"os"
)

const (
	dbNodes      = "nodes"
	dbServices   = "services"
	dbMaxMapSize = 1024 * 1024 * 1024 // 1GB maximum size
)

var (
	nullSentinel = []byte{0, 0, 0, 0} // used to encode a null value
)

// The StateStore is responsible for maintaining all the Consul
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads. The current
// implementation uses the Lightning Memory-Mapped Database (MDB).
// This gives us Multi-Version Concurrency Control for "free"
type StateStore struct {
	path         string
	env          *mdb.Env
	nodeTable    *MDBTable
	serviceTable *MDBTable
	tables       MDBTables
}

// StateSnapshot is used to provide a point-in-time snapshot
// It works by starting a readonly transaction against all tables.
type StateSnapshot struct {
	store *StateStore
	tx    *MDBTxn
}

// Close is used to abort the transaction and allow for cleanup
func (s *StateSnapshot) Close() error {
	s.tx.Abort()
	return nil
}

// NewStateStore is used to create a new state store
func NewStateStore() (*StateStore, error) {
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
		path: path,
		env:  env,
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

	// Setup our tables
	s.nodeTable = &MDBTable{
		Env:  s.env,
		Name: dbNodes,
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Node"},
			},
		},
		Encoder: func(obj interface{}) []byte {
			buf, err := structs.Encode(255, obj)
			if err != nil {
				panic(err)
			}
			return buf[1:]
		},
		Decoder: func(buf []byte) interface{} {
			out := new(structs.Node)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}
	if err := s.nodeTable.Init(); err != nil {
		return err
	}

	s.serviceTable = &MDBTable{
		Env:  s.env,
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
		Encoder: func(obj interface{}) []byte {
			buf, err := structs.Encode(255, obj)
			if err != nil {
				panic(err)
			}
			return buf[1:]
		},
		Decoder: func(buf []byte) interface{} {
			out := new(structs.ServiceNode)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}
	if err := s.serviceTable.Init(); err != nil {
		return err
	}

	// Store the set of tables
	s.tables = []*MDBTable{s.nodeTable, s.serviceTable}
	return nil
}

// EnsureNode is used to ensure a given node exists, with the provided address
func (s *StateStore) EnsureNode(node structs.Node) error {
	return s.nodeTable.Insert(node)
}

// GetNode returns all the address of the known and if it was found
func (s *StateStore) GetNode(name string) (bool, string) {
	res, err := s.nodeTable.Get("id", name)
	if err != nil {
		panic(fmt.Errorf("Failed to get node: %v", err))
	}
	if len(res) == 0 {
		return false, ""
	}
	return true, res[0].(*structs.Node).Address
}

// GetNodes returns all the known nodes, the slice alternates between
// the node name and address
func (s *StateStore) Nodes() structs.Nodes {
	res, err := s.nodeTable.Get("id")
	if err != nil {
		panic(fmt.Errorf("Failed to get nodes: %v", err))
	}
	results := make([]structs.Node, len(res))
	for i, r := range res {
		results[i] = *r.(*structs.Node)
	}
	return results
}

// EnsureService is used to ensure a given node exposes a service
func (s *StateStore) EnsureService(name, id, service, tag string, port int) error {
	// Ensure the node exists
	res, err := s.nodeTable.Get("id", name)
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return fmt.Errorf("Missing node registration")
	}

	// Create the entry
	entry := structs.ServiceNode{
		Node:        name,
		ServiceID:   id,
		ServiceName: service,
		ServiceTag:  tag,
		ServicePort: port,
	}

	// Ensure the service entry is set
	return s.serviceTable.Insert(&entry)
}

// NodeServices is used to return all the services of a given node
func (s *StateStore) NodeServices(name string) *structs.NodeServices {
	tx, err := s.tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()
	return s.parseNodeServices(tx, name)
}

// parseNodeServices is used to get the services belonging to a
// node, using a given txn
func (s *StateStore) parseNodeServices(tx *MDBTxn, name string) *structs.NodeServices {
	ns := &structs.NodeServices{
		Services: make(map[string]*structs.NodeService),
	}

	// Get the node first
	res, err := s.nodeTable.GetTxn(tx, "id", name)
	if err != nil {
		panic(fmt.Errorf("Failed to get node: %v", err))
	}
	if len(res) == 0 {
		return ns
	}

	// Set the address
	node := res[0].(*structs.Node)
	ns.Address = node.Address

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
	return ns
}

// DeleteNodeService is used to delete a node service
func (s *StateStore) DeleteNodeService(node, id string) error {
	_, err := s.serviceTable.Delete("id", node, id)
	return err
}

// DeleteNode is used to delete a node and all it's services
func (s *StateStore) DeleteNode(node string) error {
	if _, err := s.serviceTable.Delete("id", node); err != nil {
		return err
	}
	if _, err := s.nodeTable.Delete("id", node); err != nil {
		return err
	}
	return nil
}

// Services is used to return all the services with a list of associated tags
func (s *StateStore) Services() map[string][]string {
	// TODO: Optimize to not table scan.. We can do a distinct
	// type of query to avoid this
	res, err := s.serviceTable.Get("id")
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
	return services
}

// ServiceNodes returns the nodes associated with a given service
func (s *StateStore) ServiceNodes(service string) structs.ServiceNodes {
	tx, err := s.tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	res, err := s.serviceTable.Get("service", service)
	return parseServiceNodes(tx, s.nodeTable, res, err)
}

// ServiceTagNodes returns the nodes associated with a given service matching a tag
func (s *StateStore) ServiceTagNodes(service, tag string) structs.ServiceNodes {
	tx, err := s.tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	res, err := s.serviceTable.Get("service", service, tag)
	return parseServiceNodes(tx, s.nodeTable, res, err)
}

// parseServiceNodes parses results ServiceNodes and ServiceTagNodes
func parseServiceNodes(tx *MDBTxn, table *MDBTable, res []interface{}, err error) structs.ServiceNodes {
	if err != nil {
		panic(fmt.Errorf("Failed to get node services: %v", err))
	}
	println(fmt.Sprintf("res: %#v", res))

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

// Snapshot is used to create a point in time snapshot
func (s *StateStore) Snapshot() (*StateSnapshot, error) {
	// Begin a new txn on all tables
	tx, err := s.tables.StartTxn(true)
	if err != nil {
		return nil, err
	}

	// Return the snapshot
	snap := &StateSnapshot{
		store: s,
		tx:    tx,
	}
	return snap, nil
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
	return s.store.parseNodeServices(s.tx, name)
}
