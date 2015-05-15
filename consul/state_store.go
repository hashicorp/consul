package consul

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-radix"
	"github.com/armon/gomdb"
	"github.com/hashicorp/consul/consul/structs"
)

const (
	dbNodes                  = "nodes"
	dbServices               = "services"
	dbChecks                 = "checks"
	dbKVS                    = "kvs"
	dbTombstone              = "tombstones"
	dbSessions               = "sessions"
	dbSessionChecks          = "sessionChecks"
	dbACLs                   = "acls"
	dbMaxMapSize32bit uint64 = 128 * 1024 * 1024       // 128MB maximum size
	dbMaxMapSize64bit uint64 = 32 * 1024 * 1024 * 1024 // 32GB maximum size
	dbMaxReaders      uint   = 4096                    // 4K, default is 126
)

// kvMode is used internally to control which type of set
// operation we are performing
type kvMode int

const (
	kvSet kvMode = iota
	kvCAS
	kvLock
	kvUnlock
)

// The StateStore is responsible for maintaining all the Consul
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads. The current
// implementation uses the Lightning Memory-Mapped Database (MDB).
// This gives us Multi-Version Concurrency Control for "free"
type StateStore struct {
	logger            *log.Logger
	path              string
	env               *mdb.Env
	nodeTable         *MDBTable
	serviceTable      *MDBTable
	checkTable        *MDBTable
	kvsTable          *MDBTable
	tombstoneTable    *MDBTable
	sessionTable      *MDBTable
	sessionCheckTable *MDBTable
	aclTable          *MDBTable
	tables            MDBTables
	watch             map[*MDBTable]*NotifyGroup
	queryTables       map[string]MDBTables

	// kvWatch is a more optimized way of watching for KV changes.
	// Instead of just using a NotifyGroup for the entire table,
	// a watcher is instantiated on a given prefix. When a change happens,
	// only the relevant watchers are woken up. This reduces the cost of
	// watching for KV changes.
	kvWatch     *radix.Tree
	kvWatchLock sync.Mutex

	// lockDelay is used to mark certain locks as unacquirable.
	// When a lock is forcefully released (failing health
	// check, destroyed session, etc), it is subject to the LockDelay
	// impossed by the session. This prevents another session from
	// acquiring the lock for some period of time as a protection against
	// split-brains. This is inspired by the lock-delay in Chubby.
	// Because this relies on wall-time, we cannot assume all peers
	// perceive time as flowing uniformly. This means KVSLock MUST ignore
	// lockDelay, since the lockDelay may have expired on the leader,
	// but not on the follower. Rejecting the lock could result in
	// inconsistencies in the FSMs due to the rate time progresses. Instead,
	// only the opinion of the leader is respected, and the Raft log
	// is never questioned.
	lockDelay     map[string]time.Time
	lockDelayLock sync.RWMutex

	// GC is when we create tombstones to track their time-to-live.
	// The GC is consumed upstream to manage clearing of tombstones.
	gc *TombstoneGC
}

// StateSnapshot is used to provide a point-in-time snapshot
// It works by starting a readonly transaction against all tables.
type StateSnapshot struct {
	store     *StateStore
	tx        *MDBTxn
	lastIndex uint64
}

// sessionCheck is used to create a many-to-one table such
// that each check registered by a session can be mapped back
// to the session row.
type sessionCheck struct {
	Node    string
	CheckID string
	Session string
}

// Close is used to abort the transaction and allow for cleanup
func (s *StateSnapshot) Close() error {
	s.tx.Abort()
	return nil
}

// NewStateStore is used to create a new state store
func NewStateStore(gc *TombstoneGC, logOutput io.Writer) (*StateStore, error) {
	// Create a new temp dir
	path, err := ioutil.TempDir("", "consul")
	if err != nil {
		return nil, err
	}
	return NewStateStorePath(gc, path, logOutput)
}

// NewStateStorePath is used to create a new state store at a given path
// The path is cleared on closing.
func NewStateStorePath(gc *TombstoneGC, path string, logOutput io.Writer) (*StateStore, error) {
	// Open the env
	env, err := mdb.NewEnv()
	if err != nil {
		return nil, err
	}

	s := &StateStore{
		logger:    log.New(logOutput, "", log.LstdFlags),
		path:      path,
		env:       env,
		watch:     make(map[*MDBTable]*NotifyGroup),
		kvWatch:   radix.New(),
		lockDelay: make(map[string]time.Time),
		gc:        gc,
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

	// Set the maximum db size based on 32/64bit. Since we are
	// doing an mmap underneath, we need to limit our use of virtual
	// address space on 32bit, but don't have to care on 64bit.
	dbSize := dbMaxMapSize32bit
	if runtime.GOARCH == "amd64" {
		dbSize = dbMaxMapSize64bit
	}

	// Increase the maximum map size
	if err := s.env.SetMapSize(dbSize); err != nil {
		return err
	}

	// Increase the maximum number of concurrent readers
	// TODO: Block transactions if we could exceed dbMaxReaders
	if err := s.env.SetMaxReaders(dbMaxReaders); err != nil {
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
				Unique:          true,
				Fields:          []string{"Node"},
				CaseInsensitive: true,
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
				AllowBlank:      true,
				Fields:          []string{"ServiceName"},
				CaseInsensitive: true,
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

	s.kvsTable = &MDBTable{
		Name: dbKVS,
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Key"},
			},
			"id_prefix": &MDBIndex{
				Virtual:   true,
				RealIndex: "id",
				Fields:    []string{"Key"},
				IdxFunc:   DefaultIndexPrefixFunc,
			},
			"session": &MDBIndex{
				AllowBlank: true,
				Fields:     []string{"Session"},
			},
		},
		Decoder: func(buf []byte) interface{} {
			out := new(structs.DirEntry)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}

	s.tombstoneTable = &MDBTable{
		Name: dbTombstone,
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Key"},
			},
			"id_prefix": &MDBIndex{
				Virtual:   true,
				RealIndex: "id",
				Fields:    []string{"Key"},
				IdxFunc:   DefaultIndexPrefixFunc,
			},
		},
		Decoder: func(buf []byte) interface{} {
			out := new(structs.DirEntry)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}

	s.sessionTable = &MDBTable{
		Name: dbSessions,
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"ID"},
			},
			"node": &MDBIndex{
				AllowBlank: true,
				Fields:     []string{"Node"},
			},
		},
		Decoder: func(buf []byte) interface{} {
			out := new(structs.Session)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}

	s.sessionCheckTable = &MDBTable{
		Name: dbSessionChecks,
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Node", "CheckID", "Session"},
			},
		},
		Decoder: func(buf []byte) interface{} {
			out := new(sessionCheck)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}

	s.aclTable = &MDBTable{
		Name: dbACLs,
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"ID"},
			},
		},
		Decoder: func(buf []byte) interface{} {
			out := new(structs.ACL)
			if err := structs.Decode(buf, out); err != nil {
				panic(err)
			}
			return out
		},
	}

	// Store the set of tables
	s.tables = []*MDBTable{s.nodeTable, s.serviceTable, s.checkTable,
		s.kvsTable, s.tombstoneTable, s.sessionTable, s.sessionCheckTable,
		s.aclTable}
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
		"NodeInfo":          MDBTables{s.nodeTable, s.serviceTable, s.checkTable},
		"NodeDump":          MDBTables{s.nodeTable, s.serviceTable, s.checkTable},
		"SessionGet":        MDBTables{s.sessionTable},
		"SessionList":       MDBTables{s.sessionTable},
		"NodeSessions":      MDBTables{s.sessionTable},
		"ACLGet":            MDBTables{s.aclTable},
		"ACLList":           MDBTables{s.aclTable},
	}
	return nil
}

// Watch is used to subscribe a channel to a set of MDBTables
func (s *StateStore) Watch(tables MDBTables, notify chan struct{}) {
	for _, t := range tables {
		s.watch[t].Wait(notify)
	}
}

// StopWatch is used to unsubscribe a channel to a set of MDBTables
func (s *StateStore) StopWatch(tables MDBTables, notify chan struct{}) {
	for _, t := range tables {
		s.watch[t].Clear(notify)
	}
}

// WatchKV is used to subscribe a channel to changes in KV data
func (s *StateStore) WatchKV(prefix string, notify chan struct{}) {
	s.kvWatchLock.Lock()
	defer s.kvWatchLock.Unlock()

	// Check for an existing notify group
	if raw, ok := s.kvWatch.Get(prefix); ok {
		grp := raw.(*NotifyGroup)
		grp.Wait(notify)
		return
	}

	// Create new notify group
	grp := &NotifyGroup{}
	grp.Wait(notify)
	s.kvWatch.Insert(prefix, grp)
}

// StopWatchKV is used to unsubscribe a channel from changes in KV data
func (s *StateStore) StopWatchKV(prefix string, notify chan struct{}) {
	s.kvWatchLock.Lock()
	defer s.kvWatchLock.Unlock()

	// Check for an existing notify group
	if raw, ok := s.kvWatch.Get(prefix); ok {
		grp := raw.(*NotifyGroup)
		grp.Clear(notify)
	}
}

// notifyKV is used to notify any KV listeners of a change
// on a prefix
func (s *StateStore) notifyKV(path string, prefix bool) {
	s.kvWatchLock.Lock()
	defer s.kvWatchLock.Unlock()

	var toDelete []string
	fn := func(s string, v interface{}) bool {
		group := v.(*NotifyGroup)
		group.Notify()
		if s != "" {
			toDelete = append(toDelete, s)
		}
		return false
	}

	// Invoke any watcher on the path downward to the key.
	s.kvWatch.WalkPath(path, fn)

	// If the entire prefix may be affected (e.g. delete tree),
	// invoke the entire prefix
	if prefix {
		s.kvWatch.WalkPrefix(path, fn)
	}

	// Delete the old watch groups
	for i := len(toDelete) - 1; i >= 0; i-- {
		s.kvWatch.Delete(toDelete[i])
	}
}

// QueryTables returns the Tables that are queried for a given query
func (s *StateStore) QueryTables(q string) MDBTables {
	return s.queryTables[q]
}

// EnsureRegistration is used to make sure a node, service, and check registration
// is performed within a single transaction to avoid race conditions on state updates.
func (s *StateStore) EnsureRegistration(index uint64, req *structs.RegisterRequest) error {
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	// Ensure the node
	node := structs.Node{req.Node, req.Address}
	if err := s.ensureNodeTxn(index, node, tx); err != nil {
		return err
	}

	// Ensure the service if provided
	if req.Service != nil {
		if err := s.ensureServiceTxn(index, req.Node, req.Service, tx); err != nil {
			return err
		}
	}

	// Ensure the check(s), if provided
	if req.Check != nil {
		if err := s.ensureCheckTxn(index, req.Check, tx); err != nil {
			return err
		}
	}
	for _, check := range req.Checks {
		if err := s.ensureCheckTxn(index, check, tx); err != nil {
			return err
		}
	}

	// Commit as one unit
	return tx.Commit()
}

// EnsureNode is used to ensure a given node exists, with the provided address
func (s *StateStore) EnsureNode(index uint64, node structs.Node) error {
	tx, err := s.nodeTable.StartTxn(false, nil)
	if err != nil {
		return err
	}
	defer tx.Abort()
	if err := s.ensureNodeTxn(index, node, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ensureNodeTxn is used to ensure a given node exists, with the provided address
// within a given txn
func (s *StateStore) ensureNodeTxn(index uint64, node structs.Node, tx *MDBTxn) error {
	if err := s.nodeTable.InsertTxn(tx, node); err != nil {
		return err
	}
	if err := s.nodeTable.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	tx.Defer(func() { s.watch[s.nodeTable].Notify() })
	return nil
}

// GetNode returns all the address of the known and if it was found
func (s *StateStore) GetNode(name string) (uint64, bool, string) {
	idx, res, err := s.nodeTable.Get("id", name)
	if err != nil {
		s.logger.Printf("[ERR] consul.state: Error during node lookup: %v", err)
		return 0, false, ""
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
		s.logger.Printf("[ERR] consul.state: Error getting nodes: %v", err)
	}
	results := make([]structs.Node, len(res))
	for i, r := range res {
		results[i] = *r.(*structs.Node)
	}
	return idx, results
}

// EnsureService is used to ensure a given node exposes a service
func (s *StateStore) EnsureService(index uint64, node string, ns *structs.NodeService) error {
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()
	if err := s.ensureServiceTxn(index, node, ns, tx); err != nil {
		return nil
	}
	return tx.Commit()
}

// ensureServiceTxn is used to ensure a given node exposes a service in a transaction
func (s *StateStore) ensureServiceTxn(index uint64, node string, ns *structs.NodeService, tx *MDBTxn) error {
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
		Node:           node,
		ServiceID:      ns.ID,
		ServiceName:    ns.Service,
		ServiceTags:    ns.Tags,
		ServiceAddress: ns.Address,
		ServicePort:    ns.Port,
	}

	// Ensure the service entry is set
	if err := s.serviceTable.InsertTxn(tx, &entry); err != nil {
		return err
	}
	if err := s.serviceTable.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	tx.Defer(func() { s.watch[s.serviceTable].Notify() })
	return nil
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
		s.logger.Printf("[ERR] consul.state: Failed to get node: %v", err)
	}
	if len(res) == 0 {
		return index, nil
	}

	// Set the address
	node := res[0].(*structs.Node)
	ns.Node = *node

	// Get the services
	res, err = s.serviceTable.GetTxn(tx, "id", name)
	if err != nil {
		s.logger.Printf("[ERR] consul.state: Failed to get node '%s' services: %v", name, err)
	}

	// Add each service
	for _, r := range res {
		service := r.(*structs.ServiceNode)
		srv := &structs.NodeService{
			ID:      service.ServiceID,
			Service: service.ServiceName,
			Tags:    service.ServiceTags,
			Address: service.ServiceAddress,
			Port:    service.ServicePort,
		}
		ns.Services[srv.ID] = srv
	}
	return index, ns
}

// DeleteNodeService is used to delete a node service
func (s *StateStore) DeleteNodeService(index uint64, node, id string) error {
	tx, err := s.tables.StartTxn(false)
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
		tx.Defer(func() { s.watch[s.serviceTable].Notify() })
	}

	// Invalidate any sessions using these checks
	checks, err := s.checkTable.GetTxn(tx, "node", node, id)
	if err != nil {
		return err
	}
	for _, c := range checks {
		check := c.(*structs.HealthCheck)
		if err := s.invalidateCheck(index, tx, node, check.CheckID); err != nil {
			return err
		}
	}

	if n, err := s.checkTable.DeleteTxn(tx, "node", node, id); err != nil {
		return err
	} else if n > 0 {
		if err := s.checkTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		tx.Defer(func() { s.watch[s.checkTable].Notify() })
	}
	return tx.Commit()
}

// DeleteNode is used to delete a node and all it's services
func (s *StateStore) DeleteNode(index uint64, node string) error {
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	// Invalidate any sessions held by the node
	if err := s.invalidateNode(index, tx, node); err != nil {
		return err
	}

	if n, err := s.serviceTable.DeleteTxn(tx, "id", node); err != nil {
		return err
	} else if n > 0 {
		if err := s.serviceTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		tx.Defer(func() { s.watch[s.serviceTable].Notify() })
	}
	if n, err := s.checkTable.DeleteTxn(tx, "id", node); err != nil {
		return err
	} else if n > 0 {
		if err := s.checkTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		tx.Defer(func() { s.watch[s.checkTable].Notify() })
	}
	if n, err := s.nodeTable.DeleteTxn(tx, "id", node); err != nil {
		return err
	} else if n > 0 {
		if err := s.nodeTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		tx.Defer(func() { s.watch[s.nodeTable].Notify() })
	}
	return tx.Commit()
}

// Services is used to return all the services with a list of associated tags
func (s *StateStore) Services() (uint64, map[string][]string) {
	services := make(map[string][]string)
	idx, res, err := s.serviceTable.Get("id")
	if err != nil {
		s.logger.Printf("[ERR] consul.state: Failed to get services: %v", err)
		return idx, services
	}
	for _, r := range res {
		srv := r.(*structs.ServiceNode)
		tags, ok := services[srv.ServiceName]
		if !ok {
			services[srv.ServiceName] = make([]string, 0)
		}

		for _, tag := range srv.ServiceTags {
			if !strContains(tags, tag) {
				tags = append(tags, tag)
				services[srv.ServiceName] = tags
			}
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
	return idx, s.parseServiceNodes(tx, s.nodeTable, res, err)
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

	res, err := s.serviceTable.GetTxn(tx, "service", service)
	res = serviceTagFilter(res, tag)
	return idx, s.parseServiceNodes(tx, s.nodeTable, res, err)
}

// serviceTagFilter is used to filter a list of *structs.ServiceNode which do
// not have the specified tag
func serviceTagFilter(l []interface{}, tag string) []interface{} {
	n := len(l)
	for i := 0; i < n; i++ {
		srv := l[i].(*structs.ServiceNode)
		if !strContains(ToLowerList(srv.ServiceTags), strings.ToLower(tag)) {
			l[i], l[n-1] = l[n-1], nil
			i--
			n--
		}
	}
	return l[:n]
}

// parseServiceNodes parses results ServiceNodes and ServiceTagNodes
func (s *StateStore) parseServiceNodes(tx *MDBTxn, table *MDBTable, res []interface{}, err error) structs.ServiceNodes {
	nodes := make(structs.ServiceNodes, len(res))
	if err != nil {
		s.logger.Printf("[ERR] consul.state: Failed to get service nodes: %v", err)
		return nodes
	}

	for i, r := range res {
		srv := r.(*structs.ServiceNode)

		// Get the address of the node
		nodeRes, err := table.GetTxn(tx, "id", srv.Node)
		if err != nil || len(nodeRes) != 1 {
			s.logger.Printf("[ERR] consul.state: Failed to join service node %#v with node: %v", *srv, err)
			continue
		}
		srv.Address = nodeRes[0].(*structs.Node).Address

		nodes[i] = *srv
	}

	return nodes
}

// EnsureCheck is used to create a check or updates it's state
func (s *StateStore) EnsureCheck(index uint64, check *structs.HealthCheck) error {
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()
	if err := s.ensureCheckTxn(index, check, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ensureCheckTxn is used to create a check or updates it's state in a transaction
func (s *StateStore) ensureCheckTxn(index uint64, check *structs.HealthCheck, tx *MDBTxn) error {
	// Ensure we have a status
	if check.Status == "" {
		check.Status = structs.HealthCritical
	}

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

	// Invalidate any sessions if status is critical
	if check.Status == structs.HealthCritical {
		err := s.invalidateCheck(index, tx, check.Node, check.CheckID)
		if err != nil {
			return err
		}
	}

	// Ensure the check is set
	if err := s.checkTable.InsertTxn(tx, check); err != nil {
		return err
	}
	if err := s.checkTable.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	tx.Defer(func() { s.watch[s.checkTable].Notify() })
	return nil
}

// DeleteNodeCheck is used to delete a node health check
func (s *StateStore) DeleteNodeCheck(index uint64, node, id string) error {
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		return err
	}
	defer tx.Abort()

	// Invalidate any sessions held by this check
	if err := s.invalidateCheck(index, tx, node, id); err != nil {
		return err
	}

	if n, err := s.checkTable.DeleteTxn(tx, "id", node, id); err != nil {
		return err
	} else if n > 0 {
		if err := s.checkTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		tx.Defer(func() { s.watch[s.checkTable].Notify() })
	}
	return tx.Commit()
}

// NodeChecks is used to get all the checks for a node
func (s *StateStore) NodeChecks(node string) (uint64, structs.HealthChecks) {
	return s.parseHealthChecks(s.checkTable.Get("id", node))
}

// ServiceChecks is used to get all the checks for a service
func (s *StateStore) ServiceChecks(service string) (uint64, structs.HealthChecks) {
	return s.parseHealthChecks(s.checkTable.Get("service", service))
}

// CheckInState is used to get all the checks for a service in a given state
func (s *StateStore) ChecksInState(state string) (uint64, structs.HealthChecks) {
	var idx uint64
	var res []interface{}
	var err error
	if state == structs.HealthAny {
		idx, res, err = s.checkTable.Get("id")
	} else {
		idx, res, err = s.checkTable.Get("status", state)
	}
	return s.parseHealthChecks(idx, res, err)
}

// parseHealthChecks is used to handle the resutls of a Get against
// the checkTable
func (s *StateStore) parseHealthChecks(idx uint64, res []interface{}, err error) (uint64, structs.HealthChecks) {
	results := make([]*structs.HealthCheck, len(res))
	if err != nil {
		s.logger.Printf("[ERR] consul.state: Failed to get health checks: %v", err)
		return idx, results
	}
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

	res, err := s.serviceTable.GetTxn(tx, "service", service)
	res = serviceTagFilter(res, tag)
	return idx, s.parseCheckServiceNodes(tx, res, err)
}

// parseCheckServiceNodes parses results CheckServiceNodes and CheckServiceTagNodes
func (s *StateStore) parseCheckServiceNodes(tx *MDBTxn, res []interface{}, err error) structs.CheckServiceNodes {
	nodes := make(structs.CheckServiceNodes, len(res))
	if err != nil {
		s.logger.Printf("[ERR] consul.state: Failed to get service nodes: %v", err)
		return nodes
	}

	for i, r := range res {
		srv := r.(*structs.ServiceNode)

		// Get the node
		nodeRes, err := s.nodeTable.GetTxn(tx, "id", srv.Node)
		if err != nil || len(nodeRes) != 1 {
			s.logger.Printf("[ERR] consul.state: Failed to join service node %#v with node: %v", *srv, err)
			continue
		}

		// Get any associated checks of the service
		res, err := s.checkTable.GetTxn(tx, "node", srv.Node, srv.ServiceID)
		_, checks := s.parseHealthChecks(0, res, err)

		// Get any checks of the node, not assciated with any service
		res, err = s.checkTable.GetTxn(tx, "node", srv.Node, "")
		_, nodeChecks := s.parseHealthChecks(0, res, err)
		checks = append(checks, nodeChecks...)

		// Setup the node
		nodes[i].Node = *nodeRes[0].(*structs.Node)
		nodes[i].Service = structs.NodeService{
			ID:      srv.ServiceID,
			Service: srv.ServiceName,
			Tags:    srv.ServiceTags,
			Address: srv.ServiceAddress,
			Port:    srv.ServicePort,
		}
		nodes[i].Checks = checks
	}

	return nodes
}

// NodeInfo is used to generate the full info about a node.
func (s *StateStore) NodeInfo(node string) (uint64, structs.NodeDump) {
	tables := s.queryTables["NodeInfo"]
	tx, err := tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	idx, err := tables.LastIndexTxn(tx)
	if err != nil {
		panic(fmt.Errorf("Failed to get last index: %v", err))
	}

	res, err := s.nodeTable.GetTxn(tx, "id", node)
	return idx, s.parseNodeInfo(tx, res, err)
}

// NodeDump is used to generate the NodeInfo for all nodes. This is very expensive,
// and should generally be avoided for programatic access.
func (s *StateStore) NodeDump() (uint64, structs.NodeDump) {
	tables := s.queryTables["NodeDump"]
	tx, err := tables.StartTxn(true)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	idx, err := tables.LastIndexTxn(tx)
	if err != nil {
		panic(fmt.Errorf("Failed to get last index: %v", err))
	}

	res, err := s.nodeTable.GetTxn(tx, "id")
	return idx, s.parseNodeInfo(tx, res, err)
}

// parseNodeInfo is used to scan over the results of a node
// iteration and generate a NodeDump
func (s *StateStore) parseNodeInfo(tx *MDBTxn, res []interface{}, err error) structs.NodeDump {
	dump := make(structs.NodeDump, 0, len(res))
	if err != nil {
		s.logger.Printf("[ERR] consul.state: Failed to get nodes: %v", err)
		return dump
	}

	for _, r := range res {
		// Copy the address and node
		node := r.(*structs.Node)
		info := &structs.NodeInfo{
			Node:    node.Node,
			Address: node.Address,
		}

		// Get any services of the node
		res, err = s.serviceTable.GetTxn(tx, "id", node.Node)
		if err != nil {
			s.logger.Printf("[ERR] consul.state: Failed to get node services: %v", err)
		}
		info.Services = make([]*structs.NodeService, 0, len(res))
		for _, r := range res {
			service := r.(*structs.ServiceNode)
			srv := &structs.NodeService{
				ID:      service.ServiceID,
				Service: service.ServiceName,
				Tags:    service.ServiceTags,
				Address: service.ServiceAddress,
				Port:    service.ServicePort,
			}
			info.Services = append(info.Services, srv)
		}

		// Get any checks of the node
		res, err = s.checkTable.GetTxn(tx, "node", node.Node)
		if err != nil {
			s.logger.Printf("[ERR] consul.state: Failed to get node checks: %v", err)
		}
		info.Checks = make([]*structs.HealthCheck, 0, len(res))
		for _, r := range res {
			chk := r.(*structs.HealthCheck)
			info.Checks = append(info.Checks, chk)
		}

		// Add the node info
		dump = append(dump, info)
	}
	return dump
}

// KVSSet is used to create or update a KV entry
func (s *StateStore) KVSSet(index uint64, d *structs.DirEntry) error {
	_, err := s.kvsSet(index, d, kvSet)
	return err
}

// KVSRestore is used to restore a DirEntry. It should only be used when
// doing a restore, otherwise KVSSet should be used.
func (s *StateStore) KVSRestore(d *structs.DirEntry) error {
	// Start a new txn
	tx, err := s.kvsTable.StartTxn(false, nil)
	if err != nil {
		return err
	}
	defer tx.Abort()

	if err := s.kvsTable.InsertTxn(tx, d); err != nil {
		return err
	}
	if err := s.kvsTable.SetMaxLastIndexTxn(tx, d.ModifyIndex); err != nil {
		return err
	}
	return tx.Commit()
}

// KVSGet is used to get a KV entry
func (s *StateStore) KVSGet(key string) (uint64, *structs.DirEntry, error) {
	idx, res, err := s.kvsTable.Get("id", key)
	var d *structs.DirEntry
	if len(res) > 0 {
		d = res[0].(*structs.DirEntry)
	}
	return idx, d, err
}

// KVSList is used to list all KV entries with a prefix
func (s *StateStore) KVSList(prefix string) (uint64, uint64, structs.DirEntries, error) {
	tables := MDBTables{s.kvsTable, s.tombstoneTable}
	tx, err := tables.StartTxn(true)
	if err != nil {
		return 0, 0, nil, err
	}
	defer tx.Abort()

	idx, err := tables.LastIndexTxn(tx)
	if err != nil {
		return 0, 0, nil, err
	}

	res, err := s.kvsTable.GetTxn(tx, "id_prefix", prefix)
	if err != nil {
		return 0, 0, nil, err
	}
	ents := make(structs.DirEntries, len(res))
	for idx, r := range res {
		ents[idx] = r.(*structs.DirEntry)
	}

	// Check for the higest index in the tombstone table
	var maxIndex uint64
	res, err = s.tombstoneTable.GetTxn(tx, "id_prefix", prefix)
	for _, r := range res {
		ent := r.(*structs.DirEntry)
		if ent.ModifyIndex > maxIndex {
			maxIndex = ent.ModifyIndex
		}
	}

	return maxIndex, idx, ents, err
}

// KVSListKeys is used to list keys with a prefix, and up to a given seperator
func (s *StateStore) KVSListKeys(prefix, seperator string) (uint64, []string, error) {
	tables := MDBTables{s.kvsTable, s.tombstoneTable}
	tx, err := tables.StartTxn(true)
	if err != nil {
		return 0, nil, err
	}
	defer tx.Abort()

	idx, err := s.kvsTable.LastIndexTxn(tx)
	if err != nil {
		return 0, nil, err
	}

	// Ensure a non-zero index
	if idx == 0 {
		// Must provide non-zero index to prevent blocking
		// Index 1 is impossible anyways (due to Raft internals)
		idx = 1
	}

	// Aggregate the stream
	stream := make(chan interface{}, 128)
	streamTomb := make(chan interface{}, 128)
	done := make(chan struct{})
	var keys []string
	var maxIndex uint64
	go func() {
		prefixLen := len(prefix)
		sepLen := len(seperator)
		last := ""
		for raw := range stream {
			ent := raw.(*structs.DirEntry)
			after := ent.Key[prefixLen:]

			// Update the hightest index we've seen
			if ent.ModifyIndex > maxIndex {
				maxIndex = ent.ModifyIndex
			}

			// If there is no seperator, always accumulate
			if sepLen == 0 {
				keys = append(keys, ent.Key)
				continue
			}

			// Check for the seperator
			if idx := strings.Index(after, seperator); idx >= 0 {
				toSep := ent.Key[:prefixLen+idx+sepLen]
				if last != toSep {
					keys = append(keys, toSep)
					last = toSep
				}
			} else {
				keys = append(keys, ent.Key)
			}
		}

		// Handle the tombstones for any index updates
		for raw := range streamTomb {
			ent := raw.(*structs.DirEntry)
			if ent.ModifyIndex > maxIndex {
				maxIndex = ent.ModifyIndex
			}
		}
		close(done)
	}()

	// Start the stream, and wait for completion
	if err = s.kvsTable.StreamTxn(stream, tx, "id_prefix", prefix); err != nil {
		return 0, nil, err
	}
	if err := s.tombstoneTable.StreamTxn(streamTomb, tx, "id_prefix", prefix); err != nil {
		return 0, nil, err
	}
	<-done

	// Use the maxIndex if we have any keys
	if maxIndex != 0 {
		idx = maxIndex
	}
	return idx, keys, nil
}

// KVSDelete is used to delete a KVS entry
func (s *StateStore) KVSDelete(index uint64, key string) error {
	return s.kvsDeleteWithIndex(index, "id", key)
}

// KVSDeleteCheckAndSet is used to perform an atomic delete check-and-set
func (s *StateStore) KVSDeleteCheckAndSet(index uint64, key string, casIndex uint64) (bool, error) {
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		return false, err
	}
	defer tx.Abort()

	// Get the existing node
	res, err := s.kvsTable.GetTxn(tx, "id", key)
	if err != nil {
		return false, err
	}

	// Get the existing node if any
	var exist *structs.DirEntry
	if len(res) > 0 {
		exist = res[0].(*structs.DirEntry)
	}

	// Use the casIndex as the constraint. A modify time of 0 means
	// we are doign a delete-if-not-exists (odd...), while any other
	// value means we expect that modify time.
	if casIndex == 0 {
		return exist == nil, nil
	} else if casIndex > 0 && (exist == nil || exist.ModifyIndex != casIndex) {
		return false, nil
	}

	// Do the actual delete
	if err := s.kvsDeleteWithIndexTxn(index, tx, "id", key); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// KVSDeleteTree is used to delete all keys with a given prefix
func (s *StateStore) KVSDeleteTree(index uint64, prefix string) error {
	if prefix == "" {
		return s.kvsDeleteWithIndex(index, "id")
	}
	return s.kvsDeleteWithIndex(index, "id_prefix", prefix)
}

// kvsDeleteWithIndex does a delete with either the id or id_prefix
func (s *StateStore) kvsDeleteWithIndex(index uint64, tableIndex string, parts ...string) error {
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		return err
	}
	defer tx.Abort()
	if err := s.kvsDeleteWithIndexTxn(index, tx, tableIndex, parts...); err != nil {
		return err
	}
	return tx.Commit()
}

// kvsDeleteWithIndexTxn does a delete within an existing transaction
func (s *StateStore) kvsDeleteWithIndexTxn(index uint64, tx *MDBTxn, tableIndex string, parts ...string) error {
	num := 0
	for {
		// Get some number of entries to delete
		pairs, err := s.kvsTable.GetTxnLimit(tx, 128, tableIndex, parts...)
		if err != nil {
			return err
		}

		// Create the tombstones and delete
		for _, raw := range pairs {
			ent := raw.(*structs.DirEntry)
			ent.ModifyIndex = index // Update the index
			ent.Value = nil         // Reduce storage required
			ent.Session = ""
			if err := s.tombstoneTable.InsertTxn(tx, ent); err != nil {
				return err
			}
			if num, err := s.kvsTable.DeleteTxn(tx, "id", ent.Key); err != nil {
				return err
			} else if num != 1 {
				return fmt.Errorf("Failed to delete key '%s'", ent.Key)
			}
		}

		// Increment the total number
		num += len(pairs)
		if len(pairs) == 0 {
			break
		}
	}

	if num > 0 {
		if err := s.kvsTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		tx.Defer(func() {
			// Trigger the most fine grained notifications if possible
			switch {
			case len(parts) == 0:
				s.notifyKV("", true)
			case tableIndex == "id":
				s.notifyKV(parts[0], false)
			case tableIndex == "id_prefix":
				s.notifyKV(parts[0], true)
			default:
				s.notifyKV("", true)
			}
			if s.gc != nil {
				// If GC is configured, then we hint that this index
				// required expiration.
				s.gc.Hint(index)
			}
		})
	}
	return nil
}

// KVSCheckAndSet is used to perform an atomic check-and-set
func (s *StateStore) KVSCheckAndSet(index uint64, d *structs.DirEntry) (bool, error) {
	return s.kvsSet(index, d, kvCAS)
}

// KVSLock works like KVSSet but only writes if the lock can be acquired
func (s *StateStore) KVSLock(index uint64, d *structs.DirEntry) (bool, error) {
	return s.kvsSet(index, d, kvLock)
}

// KVSUnlock works like KVSSet but only writes if the lock can be unlocked
func (s *StateStore) KVSUnlock(index uint64, d *structs.DirEntry) (bool, error) {
	return s.kvsSet(index, d, kvUnlock)
}

// KVSLockDelay returns the expiration time of a key lock delay. A key may
// have a lock delay if it was unlocked due to a session invalidation instead
// of a graceful unlock. This must be checked on the leader node, and not in
// KVSLock due to the variability of clocks.
func (s *StateStore) KVSLockDelay(key string) time.Time {
	s.lockDelayLock.RLock()
	expires := s.lockDelay[key]
	s.lockDelayLock.RUnlock()
	return expires
}

// kvsSet is the internal setter
func (s *StateStore) kvsSet(
	index uint64,
	d *structs.DirEntry,
	mode kvMode) (bool, error) {
	// Start a new txn
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		return false, err
	}
	defer tx.Abort()

	// Get the existing node
	res, err := s.kvsTable.GetTxn(tx, "id", d.Key)
	if err != nil {
		return false, err
	}

	// Get the existing node if any
	var exist *structs.DirEntry
	if len(res) > 0 {
		exist = res[0].(*structs.DirEntry)
	}

	// Use the ModifyIndex as the constraint. A modify of time of 0
	// means we are doing a set-if-not-exists, while any other value
	// means we expect that modify time.
	if mode == kvCAS {
		if d.ModifyIndex == 0 && exist != nil {
			return false, nil
		} else if d.ModifyIndex > 0 && (exist == nil || exist.ModifyIndex != d.ModifyIndex) {
			return false, nil
		}
	}

	// If attempting to lock, check this is possible
	if mode == kvLock {
		// Verify we have a session
		if d.Session == "" {
			return false, fmt.Errorf("Missing session")
		}

		// Bail if it is already locked
		if exist != nil && exist.Session != "" {
			return false, nil
		}

		// Verify the session exists
		res, err := s.sessionTable.GetTxn(tx, "id", d.Session)
		if err != nil {
			return false, err
		}
		if len(res) == 0 {
			return false, fmt.Errorf("Invalid session")
		}

		// Update the lock index
		if exist != nil {
			exist.LockIndex++
			exist.Session = d.Session
		} else {
			d.LockIndex = 1
		}
	}

	// If attempting to unlock, verify the key exists and is held
	if mode == kvUnlock {
		if exist == nil || exist.Session != d.Session {
			return false, nil
		}
		// Clear the session to unlock
		exist.Session = ""
	}

	// Set the create and modify times
	if exist == nil {
		d.CreateIndex = index
	} else {
		d.CreateIndex = exist.CreateIndex
		d.LockIndex = exist.LockIndex
		d.Session = exist.Session

	}
	d.ModifyIndex = index

	if err := s.kvsTable.InsertTxn(tx, d); err != nil {
		return false, err
	}
	if err := s.kvsTable.SetLastIndexTxn(tx, index); err != nil {
		return false, err
	}
	tx.Defer(func() { s.notifyKV(d.Key, false) })
	return true, tx.Commit()
}

// ReapTombstones is used to delete all the tombstones with a ModifyTime
// less than or equal to the given index. This is used to prevent unbounded
// storage growth of the tombstones.
func (s *StateStore) ReapTombstones(index uint64) error {
	tx, err := s.tombstoneTable.StartTxn(false, nil)
	if err != nil {
		return fmt.Errorf("failed to start txn: %v", err)
	}
	defer tx.Abort()

	// Scan the tombstone table for all the entries that are
	// eligble for GC. This could be improved by indexing on
	// ModifyTime and doing a less-than-equals scan, however
	// we don't currently support numeric indexes internally.
	// Luckily, this is a low frequency operation.
	var toDelete []string
	streamCh := make(chan interface{}, 128)
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		for raw := range streamCh {
			ent := raw.(*structs.DirEntry)
			if ent.ModifyIndex <= index {
				toDelete = append(toDelete, ent.Key)
			}
		}
	}()
	if err := s.tombstoneTable.StreamTxn(streamCh, tx, "id"); err != nil {
		s.logger.Printf("[ERR] consul.state: failed to scan tombstones: %v", err)
		return fmt.Errorf("failed to scan tombstones: %v", err)
	}
	<-doneCh

	// Delete each tombstone
	if len(toDelete) > 0 {
		s.logger.Printf("[DEBUG] consul.state: reaping %d tombstones up to %d", len(toDelete), index)
	}
	for _, key := range toDelete {
		num, err := s.tombstoneTable.DeleteTxn(tx, "id", key)
		if err != nil {
			s.logger.Printf("[ERR] consul.state: failed to delete tombstone: %v", err)
			return fmt.Errorf("failed to delete tombstone: %v", err)
		}
		if num != 1 {
			return fmt.Errorf("failed to delete tombstone '%s'", key)
		}
	}
	return tx.Commit()
}

// TombstoneRestore is used to restore a tombstone.
// It should only be used when doing a restore.
func (s *StateStore) TombstoneRestore(d *structs.DirEntry) error {
	// Start a new txn
	tx, err := s.tombstoneTable.StartTxn(false, nil)
	if err != nil {
		return err
	}
	defer tx.Abort()

	if err := s.tombstoneTable.InsertTxn(tx, d); err != nil {
		return err
	}
	return tx.Commit()
}

// SessionCreate is used to create a new session. The
// ID will be populated on a successful return
func (s *StateStore) SessionCreate(index uint64, session *structs.Session) error {
	// Verify a Session ID is generated
	if session.ID == "" {
		return fmt.Errorf("Missing Session ID")
	}

	switch session.Behavior {
	case "":
		// Default behavior is Release for backwards compatibility
		session.Behavior = structs.SessionKeysRelease
	case structs.SessionKeysRelease:
	case structs.SessionKeysDelete:
	default:
		return fmt.Errorf("Invalid Session Behavior setting '%s'", session.Behavior)
	}

	// Assign the create index
	session.CreateIndex = index

	// Start the transaction
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	// Verify that the node exists
	res, err := s.nodeTable.GetTxn(tx, "id", session.Node)
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return fmt.Errorf("Missing node registration")
	}

	// Verify that the checks exist and are not critical
	for _, checkId := range session.Checks {
		res, err := s.checkTable.GetTxn(tx, "id", session.Node, checkId)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return fmt.Errorf("Missing check '%s' registration", checkId)
		}
		chk := res[0].(*structs.HealthCheck)
		if chk.Status == structs.HealthCritical {
			return fmt.Errorf("Check '%s' is in %s state", checkId, chk.Status)
		}
	}

	// Insert the session
	if err := s.sessionTable.InsertTxn(tx, session); err != nil {
		return err
	}

	// Insert the check mappings
	sCheck := sessionCheck{Node: session.Node, Session: session.ID}
	for _, checkID := range session.Checks {
		sCheck.CheckID = checkID
		if err := s.sessionCheckTable.InsertTxn(tx, &sCheck); err != nil {
			return err
		}
	}

	// Trigger the update notifications
	if err := s.sessionTable.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	tx.Defer(func() { s.watch[s.sessionTable].Notify() })
	return tx.Commit()
}

// SessionRestore is used to restore a session. It should only be used when
// doing a restore, otherwise SessionCreate should be used.
func (s *StateStore) SessionRestore(session *structs.Session) error {
	// Start the transaction
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	// Insert the session
	if err := s.sessionTable.InsertTxn(tx, session); err != nil {
		return err
	}

	// Insert the check mappings
	sCheck := sessionCheck{Node: session.Node, Session: session.ID}
	for _, checkID := range session.Checks {
		sCheck.CheckID = checkID
		if err := s.sessionCheckTable.InsertTxn(tx, &sCheck); err != nil {
			return err
		}
	}

	// Trigger the update notifications
	index := session.CreateIndex
	if err := s.sessionTable.SetMaxLastIndexTxn(tx, index); err != nil {
		return err
	}
	tx.Defer(func() { s.watch[s.sessionTable].Notify() })
	return tx.Commit()
}

// SessionGet is used to get a session entry
func (s *StateStore) SessionGet(id string) (uint64, *structs.Session, error) {
	idx, res, err := s.sessionTable.Get("id", id)
	var d *structs.Session
	if len(res) > 0 {
		d = res[0].(*structs.Session)
	}
	return idx, d, err
}

// SessionList is used to list all the open sessions
func (s *StateStore) SessionList() (uint64, []*structs.Session, error) {
	idx, res, err := s.sessionTable.Get("id")
	out := make([]*structs.Session, len(res))
	for i, raw := range res {
		out[i] = raw.(*structs.Session)
	}
	return idx, out, err
}

// NodeSessions is used to list all the open sessions for a node
func (s *StateStore) NodeSessions(node string) (uint64, []*structs.Session, error) {
	idx, res, err := s.sessionTable.Get("node", node)
	out := make([]*structs.Session, len(res))
	for i, raw := range res {
		out[i] = raw.(*structs.Session)
	}
	return idx, out, err
}

// SessionDestroy is used to destroy a session.
func (s *StateStore) SessionDestroy(index uint64, id string) error {
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	s.logger.Printf("[DEBUG] consul.state: Invalidating session %s due to session destroy",
		id)
	if err := s.invalidateSession(index, tx, id); err != nil {
		return err
	}
	return tx.Commit()
}

// invalideNode is used to invalide all sessions belonging to a node
// All tables should be locked in the tx.
func (s *StateStore) invalidateNode(index uint64, tx *MDBTxn, node string) error {
	sessions, err := s.sessionTable.GetTxn(tx, "node", node)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		session := sess.(*structs.Session).ID
		s.logger.Printf("[DEBUG] consul.state: Invalidating session %s due to node '%s' invalidation",
			session, node)
		if err := s.invalidateSession(index, tx, session); err != nil {
			return err
		}
	}
	return nil
}

// invalidateCheck is used to invalide all sessions belonging to a check
// All tables should be locked in the tx.
func (s *StateStore) invalidateCheck(index uint64, tx *MDBTxn, node, check string) error {
	sessionChecks, err := s.sessionCheckTable.GetTxn(tx, "id", node, check)
	if err != nil {
		return err
	}
	for _, sc := range sessionChecks {
		session := sc.(*sessionCheck).Session
		s.logger.Printf("[DEBUG] consul.state: Invalidating session %s due to check '%s' invalidation",
			session, check)
		if err := s.invalidateSession(index, tx, session); err != nil {
			return err
		}
	}
	return nil
}

// invalidateSession is used to invalide a session within a given txn
// All tables should be locked in the tx.
func (s *StateStore) invalidateSession(index uint64, tx *MDBTxn, id string) error {
	// Get the session
	res, err := s.sessionTable.GetTxn(tx, "id", id)
	if err != nil {
		return err
	}

	// Quit if this session does not exist
	if len(res) == 0 {
		return nil
	}
	session := res[0].(*structs.Session)

	// Enforce the MaxLockDelay
	delay := session.LockDelay
	if delay > structs.MaxLockDelay {
		delay = structs.MaxLockDelay
	}

	// Invalidate any held locks
	if session.Behavior == structs.SessionKeysDelete {
		if err := s.deleteLocks(index, tx, delay, id); err != nil {
			return err
		}
	} else if err := s.invalidateLocks(index, tx, delay, id); err != nil {
		return err
	}

	// Nuke the session
	if _, err := s.sessionTable.DeleteTxn(tx, "id", id); err != nil {
		return err
	}

	// Delete the check mappings
	for _, checkID := range session.Checks {
		if _, err := s.sessionCheckTable.DeleteTxn(tx, "id",
			session.Node, checkID, id); err != nil {
			return err
		}
	}

	// Trigger the update notifications
	if err := s.sessionTable.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	tx.Defer(func() { s.watch[s.sessionTable].Notify() })
	return nil
}

// invalidateLocks is used to invalidate all the locks held by a session
// within a given txn. All tables should be locked in the tx.
func (s *StateStore) invalidateLocks(index uint64, tx *MDBTxn,
	lockDelay time.Duration, id string) error {
	pairs, err := s.kvsTable.GetTxn(tx, "session", id)
	if err != nil {
		return err
	}

	var expires time.Time
	if lockDelay > 0 {
		s.lockDelayLock.Lock()
		defer s.lockDelayLock.Unlock()
		expires = time.Now().Add(lockDelay)
	}

	for _, pair := range pairs {
		kv := pair.(*structs.DirEntry)
		kv.Session = ""        // Clear the lock
		kv.ModifyIndex = index // Update the modified time
		if err := s.kvsTable.InsertTxn(tx, kv); err != nil {
			return err
		}
		// If there is a lock delay, prevent acquisition
		// for at least lockDelay period
		if lockDelay > 0 {
			s.lockDelay[kv.Key] = expires
			time.AfterFunc(lockDelay, func() {
				s.lockDelayLock.Lock()
				delete(s.lockDelay, kv.Key)
				s.lockDelayLock.Unlock()
			})
		}
		tx.Defer(func() { s.notifyKV(kv.Key, false) })
	}
	if len(pairs) > 0 {
		if err := s.kvsTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
	}
	return nil
}

// deleteLocks is used to delete all the locks held by a session
// within a given txn. All tables should be locked in the tx.
func (s *StateStore) deleteLocks(index uint64, tx *MDBTxn,
	lockDelay time.Duration, id string) error {
	pairs, err := s.kvsTable.GetTxn(tx, "session", id)
	if err != nil {
		return err
	}

	var expires time.Time
	if lockDelay > 0 {
		s.lockDelayLock.Lock()
		defer s.lockDelayLock.Unlock()
		expires = time.Now().Add(lockDelay)
	}

	for _, pair := range pairs {
		kv := pair.(*structs.DirEntry)
		if err := s.kvsDeleteWithIndexTxn(index, tx, "id", kv.Key); err != nil {
			return err
		}

		// If there is a lock delay, prevent acquisition
		// for at least lockDelay period
		if lockDelay > 0 {
			s.lockDelay[kv.Key] = expires
			time.AfterFunc(lockDelay, func() {
				s.lockDelayLock.Lock()
				delete(s.lockDelay, kv.Key)
				s.lockDelayLock.Unlock()
			})
		}
	}
	return nil
}

// ACLSet is used to create or update an ACL entry
func (s *StateStore) ACLSet(index uint64, acl *structs.ACL) error {
	// Check for an ID
	if acl.ID == "" {
		return fmt.Errorf("Missing ACL ID")
	}

	// Start a new txn
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		return err
	}
	defer tx.Abort()

	// Look for the existing node
	res, err := s.aclTable.GetTxn(tx, "id", acl.ID)
	if err != nil {
		return err
	}

	switch len(res) {
	case 0:
		acl.CreateIndex = index
		acl.ModifyIndex = index
	case 1:
		exist := res[0].(*structs.ACL)
		acl.CreateIndex = exist.CreateIndex
		acl.ModifyIndex = index
	default:
		panic(fmt.Errorf("Duplicate ACL definition. Internal error"))
	}

	// Insert the ACL
	if err := s.aclTable.InsertTxn(tx, acl); err != nil {
		return err
	}

	// Trigger the update notifications
	if err := s.aclTable.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	tx.Defer(func() { s.watch[s.aclTable].Notify() })
	return tx.Commit()
}

// ACLRestore is used to restore an ACL. It should only be used when
// doing a restore, otherwise ACLSet should be used.
func (s *StateStore) ACLRestore(acl *structs.ACL) error {
	// Start a new txn
	tx, err := s.aclTable.StartTxn(false, nil)
	if err != nil {
		return err
	}
	defer tx.Abort()

	if err := s.aclTable.InsertTxn(tx, acl); err != nil {
		return err
	}
	if err := s.aclTable.SetMaxLastIndexTxn(tx, acl.ModifyIndex); err != nil {
		return err
	}
	return tx.Commit()
}

// ACLGet is used to get an ACL by ID
func (s *StateStore) ACLGet(id string) (uint64, *structs.ACL, error) {
	idx, res, err := s.aclTable.Get("id", id)
	var d *structs.ACL
	if len(res) > 0 {
		d = res[0].(*structs.ACL)
	}
	return idx, d, err
}

// ACLList is used to list all the acls
func (s *StateStore) ACLList() (uint64, []*structs.ACL, error) {
	idx, res, err := s.aclTable.Get("id")
	out := make([]*structs.ACL, len(res))
	for i, raw := range res {
		out[i] = raw.(*structs.ACL)
	}
	return idx, out, err
}

// ACLDelete is used to remove an ACL
func (s *StateStore) ACLDelete(index uint64, id string) error {
	tx, err := s.tables.StartTxn(false)
	if err != nil {
		panic(fmt.Errorf("Failed to start txn: %v", err))
	}
	defer tx.Abort()

	if n, err := s.aclTable.DeleteTxn(tx, "id", id); err != nil {
		return err
	} else if n > 0 {
		if err := s.aclTable.SetLastIndexTxn(tx, index); err != nil {
			return err
		}
		tx.Defer(func() { s.watch[s.aclTable].Notify() })
	}
	return tx.Commit()
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
		s.store.logger.Printf("[ERR] consul.state: Failed to get nodes: %v", err)
		return nil
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
	_, checks := s.store.parseHealthChecks(s.lastIndex, res, err)
	return checks
}

// KVSDump is used to list all KV entries. It takes a channel and streams
// back *struct.DirEntry objects. This will block and should be invoked
// in a goroutine.
func (s *StateSnapshot) KVSDump(stream chan<- interface{}) error {
	return s.store.kvsTable.StreamTxn(stream, s.tx, "id")
}

// TombstoneDump is used to dump all tombstone entries. It takes a channel and streams
// back *struct.DirEntry objects. This will block and should be invoked
// in a goroutine.
func (s *StateSnapshot) TombstoneDump(stream chan<- interface{}) error {
	return s.store.tombstoneTable.StreamTxn(stream, s.tx, "id")
}

// SessionList is used to list all the open sessions
func (s *StateSnapshot) SessionList() ([]*structs.Session, error) {
	res, err := s.store.sessionTable.GetTxn(s.tx, "id")
	out := make([]*structs.Session, len(res))
	for i, raw := range res {
		out[i] = raw.(*structs.Session)
	}
	return out, err
}

// ACLList is used to list all of the ACLs
func (s *StateSnapshot) ACLList() ([]*structs.ACL, error) {
	res, err := s.store.aclTable.GetTxn(s.tx, "id")
	out := make([]*structs.ACL, len(res))
	for i, raw := range res {
		out[i] = raw.(*structs.ACL)
	}
	return out, err
}
