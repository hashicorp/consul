package consul

import (
	"bytes"
	"fmt"
	"github.com/armon/gomdb"
	"github.com/hashicorp/consul/rpc"
	"io/ioutil"
	"os"
)

const (
	dbNodes        = "nodes"        // Maps node -> addr
	dbServices     = "services"     // Maps node||serv -> rpc.NodeService
	dbServiceIndex = "serviceIndex" // Maps serv||tag||node -> rpc.ServiceNode
)

// The StateStore is responsible for maintaining all the Consul
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads. The current
// implementation uses the Lightning Memory-Mapped Database (MDB).
// This gives us Multi-Version Concurrency Control for "free"
type StateStore struct {
	path string
	env  *mdb.Env
}

// StateSnapshot is used to provide a point-in-time snapshot
// It works by starting a readonly transaction against all tables.
type StateSnapshot struct {
	tx   *mdb.Txn
	dbis []mdb.DBI
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
	if err := s.env.SetMaxDBs(mdb.DBI(16)); err != nil {
		return err
	}

	// Optimize our flags for speed over safety, since the Raft log + snapshots
	// are durable. We treat this as an ephemeral in-memory DB, since we nuke
	// the data anyways.
	var flags uint = mdb.NOMETASYNC | mdb.NOSYNC | mdb.NOTLS
	if err := s.env.Open(s.path, flags, 0755); err != nil {
		return err
	}

	// Create all the tables
	tx, _, err := s.startTxn(false, dbNodes, dbServices, dbServiceIndex)
	if err != nil {
		tx.Abort()
		return err
	}
	return tx.Commit()
}

// startTxn is used to start a transaction and open all the associated sub-databases
func (s *StateStore) startTxn(readonly bool, open ...string) (*mdb.Txn, []mdb.DBI, error) {
	var txFlags uint = 0
	var dbFlags uint = 0
	if readonly {
		txFlags |= mdb.RDONLY
	} else {
		dbFlags |= mdb.CREATE
	}

	tx, err := s.env.BeginTxn(nil, txFlags)
	if err != nil {
		return nil, nil, err
	}

	var dbs []mdb.DBI
	for _, name := range open {
		dbi, err := tx.DBIOpen(name, dbFlags)
		if err != nil {
			tx.Abort()
			return nil, nil, err
		}
		dbs = append(dbs, dbi)
	}

	return tx, dbs, nil
}

// EnsureNode is used to ensure a given node exists, with the provided address
func (s *StateStore) EnsureNode(name string, address string) error {
	tx, dbis, err := s.startTxn(false, dbNodes)
	if err != nil {
		return err
	}
	if err := tx.Put(dbis[0], []byte(name), []byte(address), 0); err != nil {
		tx.Abort()
		return err
	}
	return tx.Commit()
}

// GetNode returns all the address of the known and if it was found
func (s *StateStore) GetNode(name string) (bool, string) {
	tx, dbis, err := s.startTxn(true, dbNodes)
	if err != nil {
		panic(fmt.Errorf("Failed to get node: %v", err))
	}
	defer tx.Abort()

	val, err := tx.Get(dbis[0], []byte(name))
	if err == mdb.NotFound {
		return false, ""
	} else if err != nil {
		panic(fmt.Errorf("Failed to get node: %v", err))
	}

	return true, string(val)
}

// GetNodes returns all the known nodes, the slice alternates between
// the node name and address
func (s *StateStore) Nodes() []string {
	tx, dbis, err := s.startTxn(true, dbNodes)
	if err != nil {
		panic(fmt.Errorf("Failed to get nodes: %v", err))
	}
	defer tx.Abort()

	cursor, err := tx.CursorOpen(dbis[0])
	if err != nil {
		panic(fmt.Errorf("Failed to get nodes: %v", err))
	}

	var nodes []string
	for {
		key, val, err := cursor.Get(nil, mdb.NEXT)
		if err == mdb.NotFound {
			break
		} else if err != nil {
			panic(fmt.Errorf("Failed to get nodes: %v", err))
		}
		nodes = append(nodes, string(key), string(val))
	}
	return nodes
}

// EnsureService is used to ensure a given node exposes a service
func (s *StateStore) EnsureService(name, service, tag string, port int) error {
	// Start a txn
	tx, dbis, err := s.startTxn(false, dbNodes, dbServices, dbServiceIndex)
	if err != nil {
		return err
	}
	nodes := dbis[0]
	services := dbis[1]
	index := dbis[2]

	// Get the existing services
	existing := filterNodeServices(tx, services, name)

	// Get the node
	addr, err := tx.Get(nodes, []byte(name))
	if err != nil {
		tx.Abort()
		return err
	}

	// Update the service entry
	key := []byte(fmt.Sprintf("%s||%s", name, service))
	nService := rpc.NodeService{
		Tag:  tag,
		Port: port,
	}
	val, err := rpc.Encode(255, &nService)
	if err != nil {
		tx.Abort()
		return err
	}
	if err := tx.Put(services, key, val, 0); err != nil {
		tx.Abort()
		return err
	}

	// Remove previous entry if any
	if exist, ok := existing[service]; ok {
		key := []byte(fmt.Sprintf("%s||%s||%s", service, exist.Tag, name))
		if err := tx.Del(index, key, nil); err != nil {
			tx.Abort()
			return err
		}
	}

	// Update the index entry
	key = []byte(fmt.Sprintf("%s||%s||%s", service, tag, name))
	node := rpc.ServiceNode{
		Node:        name,
		Address:     string(addr),
		ServiceTag:  tag,
		ServicePort: port,
	}
	val, err = rpc.Encode(255, &node)
	if err != nil {
		tx.Abort()
		return err
	}
	if err := tx.Put(index, key, val, 0); err != nil {
		tx.Abort()
		return err
	}

	return tx.Commit()
}

// NodeServices is used to return all the services of a given node
func (s *StateStore) NodeServices(name string) rpc.NodeServices {
	tx, dbis, err := s.startTxn(true, dbServices)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	defer tx.Abort()
	return filterNodeServices(tx, dbis[0], name)
}

// filterNodeServices is used to filter the services to a specific node
func filterNodeServices(tx *mdb.Txn, services mdb.DBI, name string) rpc.NodeServices {
	keyPrefix := []byte(fmt.Sprintf("%s||", name))
	return parseNodeServices(tx, services, keyPrefix)
}

// parseNodeServices is used to parse the results of a queryNodeServices
func parseNodeServices(tx *mdb.Txn, dbi mdb.DBI, prefix []byte) rpc.NodeServices {
	// Create the cursor
	cursor, err := tx.CursorOpen(dbi)
	if err != nil {
		panic(fmt.Errorf("Failed to get nodes: %v", err))
	}

	services := rpc.NodeServices(make(map[string]rpc.NodeService))
	var service string
	var entry rpc.NodeService
	var key, val []byte
	first := true

	for {
		if first {
			first = false
			key, val, err = cursor.Get(prefix, mdb.SET_RANGE)
		} else {
			key, val, err = cursor.Get(nil, mdb.NEXT)
		}
		if err == mdb.NotFound {
			break
		} else if err != nil {
			panic(fmt.Errorf("Failed to get node services: %v", err))
		}

		// Bail if this does not match our filter
		if !bytes.HasPrefix(key, prefix) {
			break
		}

		// Split to get service name
		parts := bytes.SplitN(key, []byte("||"), 2)
		service = string(parts[1])

		// Setup the entry
		if val[0] != 255 {
			panic(fmt.Errorf("Bad service value: %v", val))
		}
		if err := rpc.Decode(val[1:], &entry); err != nil {
			panic(fmt.Errorf("Failed to get node services: %v", err))
		}

		// Add to the map
		services[service] = entry
	}
	return services
}

// DeleteNodeService is used to delete a node service
func (s *StateStore) DeleteNodeService(node, service string) error {
	tx, dbis, err := s.startTxn(false, dbServices, dbServiceIndex)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	services := dbis[0]
	index := dbis[1]

	// Get the existing services
	existing := filterNodeServices(tx, services, node)
	exist, ok := existing[service]

	// Bail if no existing entry
	if !ok {
		tx.Abort()
		return nil
	}

	// Delete the node service entry
	key := []byte(fmt.Sprintf("%s||%s", node, service))
	if err = tx.Del(services, key, nil); err != nil {
		tx.Abort()
		return err
	}

	// Delete the sevice index entry
	key = []byte(fmt.Sprintf("%s||%s||%s", service, exist.Tag, node))
	if err := tx.Del(index, key, nil); err != nil {
		tx.Abort()
		return err
	}

	return tx.Commit()
}

// DeleteNode is used to delete a node and all it's services
func (s *StateStore) DeleteNode(node string) error {
	tx, dbis, err := s.startTxn(false, dbNodes, dbServices, dbServiceIndex)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	nodes := dbis[0]
	services := dbis[1]
	index := dbis[2]

	// Delete the node
	err = tx.Del(nodes, []byte(node), nil)
	if err == mdb.NotFound {
		err = nil
	} else if err != nil {
		tx.Abort()
		return err
	}

	// Get the existing services
	existing := filterNodeServices(tx, services, node)

	// Nuke all the services
	for service, entry := range existing {
		// Delete the node service entry
		key := []byte(fmt.Sprintf("%s||%s", node, service))
		if err = tx.Del(services, key, nil); err != nil {
			tx.Abort()
			return err
		}

		// Delete the sevice index entry
		key = []byte(fmt.Sprintf("%s||%s||%s", service, entry.Tag, node))
		if err := tx.Del(index, key, nil); err != nil {
			tx.Abort()
			return err
		}
	}

	return tx.Commit()
}

// Services is used to return all the services with a list of associated tags
func (s *StateStore) Services() map[string][]string {
	tx, dbis, err := s.startTxn(false, dbServiceIndex)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	index := dbis[0]

	cursor, err := tx.CursorOpen(index)
	if err != nil {
		panic(fmt.Errorf("Failed to get services: %v", err))
	}

	services := make(map[string][]string)
	for {
		key, _, err := cursor.Get(nil, mdb.NEXT)
		if err == mdb.NotFound {
			break
		} else if err != nil {
			panic(fmt.Errorf("Failed to get services: %v", err))
		}
		parts := bytes.SplitN(key, []byte("||"), 3)
		service := string(parts[0])
		tag := string(parts[1])

		tags := services[service]
		if !strContains(tags, tag) {
			tags = append(tags, tag)
			services[service] = tags
		}
	}
	return services
}

// ServiceNodes returns the nodes associated with a given service
func (s *StateStore) ServiceNodes(service string) rpc.ServiceNodes {
	tx, dbis, err := s.startTxn(false, dbServiceIndex)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	defer tx.Abort()
	prefix := []byte(fmt.Sprintf("%s||", service))
	return parseServiceNodes(tx, dbis[0], prefix)
}

// ServiceTagNodes returns the nodes associated with a given service matching a tag
func (s *StateStore) ServiceTagNodes(service, tag string) rpc.ServiceNodes {
	tx, dbis, err := s.startTxn(false, dbServiceIndex)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	defer tx.Abort()
	prefix := []byte(fmt.Sprintf("%s||%s||", service, tag))
	return parseServiceNodes(tx, dbis[0], prefix)
}

// parseServiceNodes parses results ServiceNodes and ServiceTagNodes
func parseServiceNodes(tx *mdb.Txn, index mdb.DBI, prefix []byte) rpc.ServiceNodes {
	cursor, err := tx.CursorOpen(index)
	if err != nil {
		panic(fmt.Errorf("Failed to get node services: %v", err))
	}

	var nodes rpc.ServiceNodes
	var node rpc.ServiceNode
	for {
		key, val, err := cursor.Get(nil, mdb.NEXT)
		if err == mdb.NotFound {
			break
		} else if err != nil {
			panic(fmt.Errorf("Failed to get node services: %v", err))
		}

		// Bail if this does not match our filter
		if !bytes.HasPrefix(key, prefix) {
			break
		}

		// Setup the node
		if val[0] != 255 {
			panic(fmt.Errorf("Bad service value: %v", val))
		}
		if err := rpc.Decode(val[1:], &node); err != nil {
			panic(fmt.Errorf("Failed to get node services: %v", err))
		}

		nodes = append(nodes, node)
	}
	return nodes
}

// Snapshot is used to create a point in time snapshot
func (s *StateStore) Snapshot() (*StateSnapshot, error) {
	// Begin a new txn
	tx, dbis, err := s.startTxn(true, dbNodes, dbServices, dbServiceIndex)
	if err != nil {
		tx.Abort()
		return nil, err
	}

	// Return the snapshot
	snap := &StateSnapshot{
		tx:   tx,
		dbis: dbis,
	}
	return snap, nil
}

// Nodes returns all the known nodes, the slice alternates between
// the node name and address
func (s *StateSnapshot) Nodes() []string {
	cursor, err := s.tx.CursorOpen(s.dbis[0])
	if err != nil {
		panic(fmt.Errorf("Failed to get nodes: %v", err))
	}

	var nodes []string
	for {
		key, val, err := cursor.Get(nil, mdb.NEXT)
		if err == mdb.NotFound {
			break
		} else if err != nil {
			panic(fmt.Errorf("Failed to get nodes: %v", err))
		}
		nodes = append(nodes, string(key), string(val))
	}
	return nodes
}

// NodeServices is used to return all the services of a given node
func (s *StateSnapshot) NodeServices(name string) rpc.NodeServices {
	return filterNodeServices(s.tx, s.dbis[1], name)
}
