// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"hash/fnv"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/consul/agent/structs"
)

const nodeWriteLockStripes = 256

// nodeWriteLocks serializes ACL-vetted catalog writes on the leader.
//
// Catalog.Register, Catalog.Deregister and Txn.Apply authorize updates against
// the catalog entries that are already stored, such as the service an existing
// check belongs to, before applying the write to Raft. Holding the lock from
// those lookups until the Raft apply completes prevents a concurrent write from
// changing the stored entries between authorization and apply.
//
// Locking is per node, so writes to unrelated nodes run concurrently and Raft
// can batch them into one log write. Two hazards keep per-node stripes
// sufficient:
//
//   - Writes that target a node by ID can resolve to a stored node whose name
//     differs from the request's name (ensureNodeTxn resolves by ID first and
//     cascade-deletes the existing node on a rename). Such writes therefore
//     also take the stripe for the node ID, so any two writes naming the same
//     ID serialize even when they name different nodes.
//   - A request with a fresh ID that reuses an existing node's name is
//     covered by the stripe for that name.
type nodeWriteLocks struct {
	stripes [nodeWriteLockStripes]sync.Mutex
}

// nodeWriteLockIndex returns the stripe for a node. Node names are matched
// case-insensitively by the state store, so the name is lowercased the same way.
func nodeWriteLockIndex(partition, peerName, node string) int {
	h := fnv.New32a()
	h.Write([]byte(partition))
	h.Write([]byte{0})
	h.Write([]byte(peerName))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToLower(node)))
	return int(h.Sum32() % nodeWriteLockStripes)
}

// nodeWriteLockIndexForID returns the stripe for a node ID. It hashes the ID
// under a distinct prefix so ID stripes do not systematically collide with
// the name stripes above; a collision only ever costs concurrency, never
// correctness.
func nodeWriteLockIndexForID(partition, peerName, id string) int {
	h := fnv.New32a()
	h.Write([]byte("node-id"))
	h.Write([]byte{0})
	h.Write([]byte(partition))
	h.Write([]byte{0})
	h.Write([]byte(peerName))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToLower(id)))
	return int(h.Sum32() % nodeWriteLockStripes)
}

// lock acquires the given stripes in ascending order, so callers locking
// several nodes cannot deadlock, and returns a function that releases them.
// Requests without catalog operations pass no indexes and take no lock, so
// KV-only transactions are not serialized.
func (l *nodeWriteLocks) lock(indexes []int) func() {
	if len(indexes) == 0 {
		return func() {}
	}
	sort.Ints(indexes)
	unique := indexes[:0]
	prev := -1
	for _, idx := range indexes {
		if idx != prev {
			unique = append(unique, idx)
			prev = idx
		}
	}
	for _, idx := range unique {
		l.stripes[idx].Lock()
	}
	return func() {
		for i := len(unique) - 1; i >= 0; i-- {
			l.stripes[unique[i]].Unlock()
		}
	}
}

// registerNodeWriteLockIndexes returns the stripes covering every node a
// register request writes to. A request carrying a node ID also takes the
// stripe for that ID, so a concurrent write for the same ID under a different
// node name cannot slip past the name stripes and rename the node out from
// under the authorization lookups.
func registerNodeWriteLockIndexes(args *structs.RegisterRequest, partition string) []int {
	indexes := []int{nodeWriteLockIndex(partition, args.PeerName, args.Node)}
	if args.ID != "" {
		indexes = append(indexes, nodeWriteLockIndexForID(partition, args.PeerName, string(args.ID)))
	}
	for _, check := range args.Checks {
		indexes = append(indexes, nodeWriteLockIndex(partition, check.PeerName, check.Node))
	}
	return indexes
}

// txnNodeWriteLockIndexes returns the stripes covering every node targeted by
// the catalog operations in a transaction. Node operations carrying a node ID
// also take the stripe for that ID, for the same rename-by-ID reason as
// registerNodeWriteLockIndexes.
func txnNodeWriteLockIndexes(ops structs.TxnOps) []int {
	var indexes []int
	for _, op := range ops {
		switch {
		case op.Node != nil:
			node := &op.Node.Node
			indexes = append(indexes, nodeWriteLockIndex(node.PartitionOrDefault(), node.PeerName, node.Node))
			if node.ID != "" {
				indexes = append(indexes, nodeWriteLockIndexForID(node.PartitionOrDefault(), node.PeerName, string(node.ID)))
			}
		case op.Service != nil:
			svc := &op.Service.Service
			indexes = append(indexes, nodeWriteLockIndex(svc.PartitionOrDefault(), svc.PeerName, op.Service.Node))
		case op.Check != nil:
			check := &op.Check.Check
			indexes = append(indexes, nodeWriteLockIndex(check.PartitionOrDefault(), check.PeerName, check.Node))
		}
	}
	return indexes
}
