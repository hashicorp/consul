package state

import (
	crand "crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
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

func snapshotIndexes(snap *Snapshot) ([]*IndexEntry, error) {
	iter, err := snap.Indexes()
	if err != nil {
		return nil, err
	}
	var indexes []*IndexEntry
	for index := iter.Next(); index != nil; index = iter.Next() {
		indexes = append(indexes, index.(*IndexEntry))
	}
	return indexes, nil
}

func restoreIndexes(indexes []*IndexEntry, r *Restore) error {
	for _, index := range indexes {
		if err := r.IndexRestore(index); err != nil {
			return err
		}
	}
	return nil
}

func testStateStore(t *testing.T) *Store {
	s := NewStateStore(nil)
	if s == nil {
		t.Fatalf("missing state store")
	}
	return s
}

func testRegisterNode(t *testing.T, s *Store, idx uint64, nodeID string) {
	testRegisterNodeOpts(t, s, idx, nodeID)
}

// testRegisterNodeWithChange registers a node and ensures it gets different from previous registration
func testRegisterNodeWithChange(t *testing.T, s *Store, idx uint64, nodeID string) {
	testRegisterNodeOpts(t, s, idx, nodeID, regNodeWithMeta(map[string]string{
		"version": fmt.Sprint(idx),
	}))
}

func testRegisterNodeWithMeta(t *testing.T, s *Store, idx uint64, nodeID string, meta map[string]string) {
	testRegisterNodeOpts(t, s, idx, nodeID, regNodeWithMeta(meta))
}

type regNodeOption func(*structs.Node) error

func regNodeWithMeta(meta map[string]string) func(*structs.Node) error {
	return func(node *structs.Node) error {
		node.Meta = meta
		return nil
	}
}

func testRegisterNodeOpts(t *testing.T, s *Store, idx uint64, nodeID string, opts ...regNodeOption) {
	node := &structs.Node{Node: nodeID}
	for _, opt := range opts {
		if err := opt(node); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	if err := s.EnsureNode(idx, node); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	n, err := tx.First(tableNodes, indexID, Query{
		Value:          nodeID,
		EnterpriseMeta: *node.GetEnterpriseMeta(),
	})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := n.(*structs.Node); !ok || result.Node != nodeID {
		t.Fatalf("bad node: %#v", result)
	}
}

// testRegisterServiceWithChange registers a service and allow ensuring the consul index is updated
// even if service already exists if using `modifyAccordingIndex`.
// This is done by setting the transaction ID in "version" meta so service will be updated if it already exists
func testRegisterServiceWithChange(t *testing.T, s *Store, idx uint64, nodeID, serviceID string, modifyAccordingIndex bool) {
	meta := make(map[string]string)
	if modifyAccordingIndex {
		meta["version"] = fmt.Sprint(idx)
	}
	svc := &structs.NodeService{
		ID:      serviceID,
		Service: serviceID,
		Address: "1.1.1.1",
		Port:    1111,
		Meta:    meta,
	}
	if err := s.EnsureService(idx, nodeID, svc); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	service, err := tx.First(tableServices, indexID, NodeServiceQuery{Node: nodeID, Service: serviceID})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := service.(*structs.ServiceNode); !ok ||
		result.Node != nodeID ||
		result.ServiceID != serviceID {
		t.Fatalf("bad service: %#v", result)
	}
}

// testRegisterService register a service with given transaction idx
// If the service already exists, transaction number might not be increased
// Use `testRegisterServiceWithChange()` if you want perform a registration that
// ensures the transaction is updated by setting idx in Meta of Service
func testRegisterService(t *testing.T, s *Store, idx uint64, nodeID, serviceID string) {
	testRegisterServiceWithChange(t, s, idx, nodeID, serviceID, false)
}

func testRegisterIngressService(t *testing.T, s *Store, idx uint64, nodeID, serviceID string) {
	svc := &structs.NodeService{
		ID:      serviceID,
		Service: serviceID,
		Kind:    structs.ServiceKindIngressGateway,
		Address: "1.1.1.1",
		Port:    1111,
	}
	if err := s.EnsureService(idx, nodeID, svc); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	service, err := tx.First(tableServices, indexID, NodeServiceQuery{Node: nodeID, Service: serviceID})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := service.(*structs.ServiceNode); !ok ||
		result.Node != nodeID ||
		result.ServiceID != serviceID {
		t.Fatalf("bad service: %#v", result)
	}
}
func testRegisterCheck(t *testing.T, s *Store, idx uint64,
	nodeID string, serviceID string, checkID types.CheckID, state string) {
	testRegisterCheckWithPartition(t, s, idx,
		nodeID, serviceID, checkID, state, "")
}

func testRegisterCheckWithPartition(t *testing.T, s *Store, idx uint64,
	nodeID string, serviceID string, checkID types.CheckID, state string, partition string) {
	chk := &structs.HealthCheck{
		Node:           nodeID,
		CheckID:        checkID,
		ServiceID:      serviceID,
		Status:         state,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(partition),
	}
	if err := s.EnsureCheck(idx, chk); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	c, err := tx.First(tableChecks, indexID, NodeCheckQuery{Node: nodeID, CheckID: string(checkID), EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(partition)})
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

func testRegisterSidecarProxy(t *testing.T, s *Store, idx uint64, nodeID string, targetServiceID string) {
	svc := &structs.NodeService{
		ID:      targetServiceID + "-sidecar-proxy",
		Service: targetServiceID + "-sidecar-proxy",
		Port:    20000,
		Kind:    structs.ServiceKindConnectProxy,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: targetServiceID,
			DestinationServiceID:   targetServiceID,
		},
	}
	require.NoError(t, s.EnsureService(idx, nodeID, svc))
}

func testRegisterConnectNativeService(t *testing.T, s *Store, idx uint64, nodeID string, serviceID string) {
	svc := &structs.NodeService{
		ID:      serviceID,
		Service: serviceID,
		Port:    1111,
		Connect: structs.ServiceConnect{
			Native: true,
		},
	}
	require.NoError(t, s.EnsureService(idx, nodeID, svc))
}

func testSetKey(t *testing.T, s *Store, idx uint64, key, value string, entMeta *acl.EnterpriseMeta) {
	entry := &structs.DirEntry{
		Key:   key,
		Value: []byte(value),
	}
	if entMeta != nil {
		entry.EnterpriseMeta = *entMeta
	}

	if err := s.KVSSet(idx, entry); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()

	e, err := tx.First(tableKVs, indexID, entry)
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

	idx, entries, err := s.KVSList(nil, "", nil)
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

func TestStateStore_Abandon(t *testing.T) {
	s := testStateStore(t)
	abandonCh := s.AbandonCh()
	s.Abandon()
	select {
	case <-abandonCh:
	default:
		t.Fatalf("bad")
	}
}

func TestStateStore_maxIndex(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "foo")
	testRegisterNode(t, s, 1, "bar")
	testRegisterService(t, s, 2, "foo", "consul")

	if max := s.maxIndex(tableNodes, tableServices); max != 2 {
		t.Fatalf("bad max: %d", max)
	}
}

func TestStateStore_indexUpdateMaxTxn(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "foo")
	testRegisterNode(t, s, 1, "bar")

	tx := s.db.WriteTxnRestore()
	if err := indexUpdateMaxTxn(tx, 3, tableNodes); err != nil {
		t.Fatalf("err: %s", err)
	}
	require.NoError(t, tx.Commit())

	if max := s.maxIndex(tableNodes); max != 3 {
		t.Fatalf("bad max: %d", max)
	}
}
