package consul

import (
	"bytes"
	"fmt"
	"github.com/armon/gomdb"
	"github.com/hashicorp/consul/consul/structs"
	"io/ioutil"
	"os"
)

const (
	dbNodes        = "nodes"            // Maps node -> addr
	dbServices     = "services"         // Maps node||servId -> structs.NodeService
	dbServiceIndex = "serviceIndex"     // Maps serv||tag||node||servId -> structs.ServiceNode
	dbMaxMapSize   = 1024 * 1024 * 1024 // 1GB maximum size
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
	defer tx.Abort()

	if err := tx.Put(dbis[0], encNull(name), encNull(address), 0); err != nil {
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
	return true, decNull(sliceCopy(val))
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
		nodes = append(nodes, decNull(sliceCopy(key)), decNull(sliceCopy(val)))
	}
	return nodes
}

// EnsureService is used to ensure a given node exposes a service
func (s *StateStore) EnsureService(name, id, service, tag string, port int) error {
	// Start a txn
	tx, dbis, err := s.startTxn(false, dbNodes, dbServices, dbServiceIndex)
	if err != nil {
		return err
	}
	defer tx.Abort()
	nodes := dbis[0]
	services := dbis[1]
	index := dbis[2]

	// Get the existing services
	existing := filterNodeServices(tx, services, name)

	// Get the node
	addr, err := tx.Get(nodes, []byte(name))
	if err != nil {
		return err
	}

	// Update the service entry
	key := []byte(fmt.Sprintf("%s||%s", name, id))
	nService := structs.NodeService{
		ID:      id,
		Service: service,
		Tag:     tag,
		Port:    port,
	}
	val, err := structs.Encode(255, &nService)
	if err != nil {
		return err
	}
	if err := tx.Put(services, key, val, 0); err != nil {
		return err
	}

	// Remove previous entry if any
	if exist, ok := existing.Services[id]; ok {
		key := []byte(fmt.Sprintf("%s||%s||%s||%s", service, exist.Tag, name, id))
		if err := tx.Del(index, key, nil); err != nil {
			return err
		}
	}

	// Update the index entry
	key = []byte(fmt.Sprintf("%s||%s||%s||%s", service, tag, name, id))
	node := structs.ServiceNode{
		Node:        name,
		Address:     string(addr),
		ServiceID:   id,
		ServiceTag:  tag,
		ServicePort: port,
	}
	val, err = structs.Encode(255, &node)
	if err != nil {
		return err
	}
	if err := tx.Put(index, key, val, 0); err != nil {
		return err
	}

	return tx.Commit()
}

// NodeServices is used to return all the services of a given node
func (s *StateStore) NodeServices(name string) *structs.NodeServices {
	tx, dbis, err := s.startTxn(true, dbNodes, dbServices)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	defer tx.Abort()
	ns := filterNodeServices(tx, dbis[1], name)

	// Get the address of the ndoe
	val, err := tx.Get(dbis[0], []byte(name))
	if err == mdb.NotFound {
		return ns
	} else if err != nil {
		panic(fmt.Errorf("Failed to get node: %v", err))
	}
	ns.Address = decNull(sliceCopy(val))

	return ns
}

// filterNodeServices is used to filter the services to a specific node
func filterNodeServices(tx *mdb.Txn, services mdb.DBI, name string) *structs.NodeServices {
	keyPrefix := []byte(fmt.Sprintf("%s||", name))
	return parseNodeServices(tx, services, keyPrefix)
}

// parseNodeServices is used to parse the results of a queryNodeServices
func parseNodeServices(tx *mdb.Txn, dbi mdb.DBI, prefix []byte) *structs.NodeServices {
	// Create the cursor
	cursor, err := tx.CursorOpen(dbi)
	if err != nil {
		panic(fmt.Errorf("Failed to get nodes: %v", err))
	}

	ns := &structs.NodeServices{
		Services: make(map[string]structs.NodeService),
	}
	var id string
	var entry structs.NodeService
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
		parts := bytes.SplitN(sliceCopy(key), []byte("||"), 2)
		id = string(parts[1])

		// Setup the entry
		if val[0] != 255 {
			panic(fmt.Errorf("Bad service value: %v", val))
		}
		if err := structs.Decode(val[1:], &entry); err != nil {
			panic(fmt.Errorf("Failed to get node services: %v", err))
		}

		// Add to the map
		ns.Services[id] = entry
	}
	return ns
}

// DeleteNodeService is used to delete a node service
func (s *StateStore) DeleteNodeService(node, id string) error {
	tx, dbis, err := s.startTxn(false, dbServices, dbServiceIndex)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	defer tx.Abort()
	services := dbis[0]
	index := dbis[1]

	// Get the existing services
	existing := filterNodeServices(tx, services, node)
	exist, ok := existing.Services[id]

	// Bail if no existing entry
	if !ok {
		return nil
	}

	// Delete the node service entry
	key := []byte(fmt.Sprintf("%s||%s", node, id))
	if err = tx.Del(services, key, nil); err != nil {
		return err
	}

	// Delete the sevice index entry
	key = []byte(fmt.Sprintf("%s||%s||%s||%s", exist.Service, exist.Tag, node, id))
	if err := tx.Del(index, key, nil); err != nil {
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
	defer tx.Abort()
	nodes := dbis[0]
	services := dbis[1]
	index := dbis[2]

	// Delete the node
	err = tx.Del(nodes, []byte(node), nil)
	if err == mdb.NotFound {
		err = nil
	} else if err != nil {
		return err
	}

	// Get the existing services
	existing := filterNodeServices(tx, services, node)

	// Nuke all the services
	for id, entry := range existing.Services {
		// Delete the node service entry
		key := []byte(fmt.Sprintf("%s||%s", node, id))
		if err = tx.Del(services, key, nil); err != nil {
			return err
		}

		// Delete the sevice index entry
		key = []byte(fmt.Sprintf("%s||%s||%s||%s", entry.Service, entry.Tag, node, id))
		if err := tx.Del(index, key, nil); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Services is used to return all the services with a list of associated tags
func (s *StateStore) Services() map[string][]string {
	tx, dbis, err := s.startTxn(true, dbServiceIndex)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	defer tx.Abort()
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
		parts := bytes.SplitN(sliceCopy(key), []byte("||"), 3)
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
func (s *StateStore) ServiceNodes(service string) structs.ServiceNodes {
	tx, dbis, err := s.startTxn(true, dbServiceIndex)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	defer tx.Abort()
	prefix := []byte(fmt.Sprintf("%s||", service))
	return parseServiceNodes(tx, dbis[0], prefix)
}

// ServiceTagNodes returns the nodes associated with a given service matching a tag
func (s *StateStore) ServiceTagNodes(service, tag string) structs.ServiceNodes {
	tx, dbis, err := s.startTxn(true, dbServiceIndex)
	if err != nil {
		panic(fmt.Errorf("Failed to get node servicess: %v", err))
	}
	defer tx.Abort()
	prefix := []byte(fmt.Sprintf("%s||%s||", service, tag))
	return parseServiceNodes(tx, dbis[0], prefix)
}

// parseServiceNodes parses results ServiceNodes and ServiceTagNodes
func parseServiceNodes(tx *mdb.Txn, index mdb.DBI, prefix []byte) structs.ServiceNodes {
	cursor, err := tx.CursorOpen(index)
	if err != nil {
		panic(fmt.Errorf("Failed to get node services: %v", err))
	}

	var nodes structs.ServiceNodes
	var node structs.ServiceNode
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

		// Setup the node
		if val[0] != 255 {
			panic(fmt.Errorf("Bad service value: %v", val))
		}
		if err := structs.Decode(val[1:], &node); err != nil {
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
		nodes = append(nodes, decNull(sliceCopy(key)), decNull(sliceCopy(val)))
	}
	return nodes
}

// NodeServices is used to return all the services of a given node
func (s *StateSnapshot) NodeServices(name string) *structs.NodeServices {
	// Get the node services
	ns := filterNodeServices(s.tx, s.dbis[1], name)

	// Get the address of the node
	val, err := s.tx.Get(s.dbis[0], []byte(name))
	if err == mdb.NotFound {
		return ns
	} else if err != nil {
		panic(fmt.Errorf("Failed to get node: %v", err))
	}
	ns.Address = decNull(sliceCopy(val))
	return ns
}

// copies a slice to prevent access to lmdb private data
func sliceCopy(in []byte) []byte {
	c := make([]byte, len(in))
	copy(c, in)
	return c
}

// encodes a potentially empty string using a sentinel
func encNull(s string) []byte {
	if s == "" {
		return nullSentinel
	}
	return []byte(s)
}

// decodes the potential sentinel to an empty string
func decNull(s []byte) string {
	if bytes.Compare(s, nullSentinel) == 0 {
		return ""
	}
	return string(s)
}
