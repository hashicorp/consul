package state

import (
	crand "crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-memdb"
)

func testUUID() string {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}

func testStateStore(t *testing.T) *StateStore {
	s, err := NewStateStore(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if s == nil {
		t.Fatalf("missing state store")
	}
	return s
}

func testRegisterNode(t *testing.T, s *StateStore, idx uint64, nodeID string) {
	testRegisterNodeWithMeta(t, s, idx, nodeID, nil)
}

func testRegisterNodeWithMeta(t *testing.T, s *StateStore, idx uint64, nodeID string, meta map[string]string) {
	node := &structs.Node{Node: nodeID, Meta: meta}
	if err := s.EnsureNode(idx, node); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	n, err := tx.First("nodes", "id", nodeID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := n.(*structs.Node); !ok || result.Node != nodeID {
		t.Fatalf("bad node: %#v", result)
	}
}

func testRegisterService(t *testing.T, s *StateStore, idx uint64, nodeID, serviceID string) {
	svc := &structs.NodeService{
		ID:      serviceID,
		Service: serviceID,
		Address: "1.1.1.1",
		Port:    1111,
	}
	if err := s.EnsureService(idx, nodeID, svc); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	service, err := tx.First("services", "id", nodeID, serviceID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := service.(*structs.ServiceNode); !ok ||
		result.Node != nodeID ||
		result.ServiceID != serviceID {
		t.Fatalf("bad service: %#v", result)
	}
}

func testRegisterCheck(t *testing.T, s *StateStore, idx uint64,
	nodeID string, serviceID string, checkID types.CheckID, state string) {
	chk := &structs.HealthCheck{
		Node:      nodeID,
		CheckID:   checkID,
		ServiceID: serviceID,
		Status:    state,
	}
	if err := s.EnsureCheck(idx, chk); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	c, err := tx.First("checks", "id", nodeID, string(checkID))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := c.(*structs.HealthCheck); !ok ||
		result.Node != nodeID ||
		result.ServiceID != serviceID ||
		result.CheckID != checkID {
		t.Fatalf("bad check: %#v", result)
	}
}

func testSetKey(t *testing.T, s *StateStore, idx uint64, key, value string) {
	entry := &structs.DirEntry{Key: key, Value: []byte(value)}
	if err := s.KVSSet(idx, entry); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	e, err := tx.First("kvs", "id", key)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := e.(*structs.DirEntry); !ok || result.Key != key {
		t.Fatalf("bad kvs entry: %#v", result)
	}
}

// watchFired is a helper for unit tests that returns if the given watch set
// fired (it doesn't care which watch actually fired). This uses a fixed
// timeout since we already expect the event happened before calling this and
// just need to distinguish a fire from a timeout. We do need a little time to
// allow the watch to set up any goroutines, though.
func watchFired(ws memdb.WatchSet) bool {
	timedOut := ws.Watch(time.After(50 * time.Millisecond))
	return !timedOut
}

func TestStateStore_Restore_Abort(t *testing.T) {
	s := testStateStore(t)

	// The detailed restore functions are tested below, this just checks
	// that abort works.
	restore := s.Restore()
	entry := &structs.DirEntry{
		Key:   "foo",
		Value: []byte("bar"),
		RaftIndex: structs.RaftIndex{
			ModifyIndex: 5,
		},
	}
	if err := restore.KVS(entry); err != nil {
		t.Fatalf("err: %s", err)
	}
	restore.Abort()

	idx, entries, err := s.KVSList("")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}
	if len(entries) != 0 {
		t.Fatalf("bad: %#v", entries)
	}
}

func TestStateStore_maxIndex(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "foo")
	testRegisterNode(t, s, 1, "bar")
	testRegisterService(t, s, 2, "foo", "consul")

	if max := s.maxIndex("nodes", "services"); max != 2 {
		t.Fatalf("bad max: %d", max)
	}
}

func TestStateStore_indexUpdateMaxTxn(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "foo")
	testRegisterNode(t, s, 1, "bar")

	tx := s.db.Txn(true)
	if err := indexUpdateMaxTxn(tx, 3, "nodes"); err != nil {
		t.Fatalf("err: %s", err)
	}
	tx.Commit()

	if max := s.maxIndex("nodes"); max != 3 {
		t.Fatalf("bad max: %d", max)
	}
}

func TestStateStore_GetWatches(t *testing.T) {
	s := testStateStore(t)

	// This test does two things - it makes sure there's no full table
	// watch for KVS, and it makes sure that asking for a watch that
	// doesn't exist causes a panic.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("didn't get expected panic")
			}
		}()
		s.getTableWatch("kvs")
	}()

	// Similar for tombstones; those don't support watches at all.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("didn't get expected panic")
			}
		}()
		s.getTableWatch("tombstones")
	}()

	// Make sure requesting a bogus method causes a panic.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("didn't get expected panic")
			}
		}()
		s.GetQueryWatch("dogs")
	}()

	// Request valid watches.
	if w := s.GetQueryWatch("Nodes"); w == nil {
		t.Fatalf("didn't get a watch")
	}
	if w := s.GetQueryWatch("NodeDump"); w == nil {
		t.Fatalf("didn't get a watch")
	}
	if w := s.GetKVSWatch("/dogs"); w == nil {
		t.Fatalf("didn't get a watch")
	}
}
