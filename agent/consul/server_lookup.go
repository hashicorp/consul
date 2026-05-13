// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"sync"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/raft"
)

// ServerLookup encapsulates looking up servers by id and address
type ServerLookup struct {
	lock            sync.RWMutex
	addressToServer map[raft.ServerAddress]*metadata.Server
	idToServer      map[raft.ServerID]*metadata.Server
}

func NewServerLookup() *ServerLookup {
	return &ServerLookup{
		addressToServer: make(map[raft.ServerAddress]*metadata.Server),
		idToServer:      make(map[raft.ServerID]*metadata.Server),
	}
}

func (sl *ServerLookup) AddServer(server *metadata.Server) {
	sl.lock.Lock()
	defer sl.lock.Unlock()
	sl.addressToServer[raft.ServerAddress(server.Addr.String())] = server
	sl.idToServer[raft.ServerID(server.ID)] = server
}

// RemoveServer removes the entries for the given server from both the
// address- and ID-keyed indexes, but only if the currently-tracked entry
// at each key still refers to the same generation of the peer.
//
// The keys (IP:port and Raft NodeID) are not unique across time: the same
// IP:port can host different generations of a peer (rename, wipe and
// rejoin, address reuse), and the same NodeID can move to a different
// IP:port (relocation). When two generations share a key, an unguarded
// delete for the older generation can blank out the entry that AddServer
// installed for the live generation, and any subsequent RPC that hits
// getLeader() then returns structs.ErrLeaderNotTracked until a later
// member event happens to resync the lookup — see
// TestServerLookup_HostnameRenameRace.
//
// "Same generation" is identified by the full (ID, Name, Addr) tuple,
// which is the minimal set of fields that distinguishes the three known
// collision modes:
//
//   - rename:        same ID and Addr,   different Name.
//   - relocation:    same ID and Name,   different Addr.
//   - address reuse: different ID,       different Name.
func (sl *ServerLookup) RemoveServer(server *metadata.Server) {
	sl.lock.Lock()
	defer sl.lock.Unlock()
	addr := raft.ServerAddress(server.Addr.String())
	id := raft.ServerID(server.ID)
	if cur, ok := sl.addressToServer[addr]; ok && sameGeneration(cur, server) {
		delete(sl.addressToServer, addr)
	}
	if cur, ok := sl.idToServer[id]; ok && sameGeneration(cur, server) {
		delete(sl.idToServer, id)
	}
}

// sameGeneration reports whether two metadata.Server values refer to the
// same generation of a Consul peer. Equality is defined as identical
// (ID, Name, Addr) — see RemoveServer for why.
func sameGeneration(a, b *metadata.Server) bool {
	return a.ID == b.ID && a.Name == b.Name && a.Addr.String() == b.Addr.String()
}

// Implements the ServerAddressProvider interface
func (sl *ServerLookup) ServerAddr(id raft.ServerID) (raft.ServerAddress, error) {
	sl.lock.RLock()
	defer sl.lock.RUnlock()
	svr, ok := sl.idToServer[id]
	if !ok {
		return "", fmt.Errorf("Could not find address for server id %v", id)
	}
	return raft.ServerAddress(svr.Addr.String()), nil
}

// Server looks up the server by address, returns a boolean if not found
func (sl *ServerLookup) Server(addr raft.ServerAddress) *metadata.Server {
	sl.lock.RLock()
	defer sl.lock.RUnlock()
	return sl.addressToServer[addr]
}

func (sl *ServerLookup) Servers() []*metadata.Server {
	sl.lock.RLock()
	defer sl.lock.RUnlock()
	var ret []*metadata.Server
	for _, svr := range sl.addressToServer {
		ret = append(ret, svr)
	}
	return ret
}

func (sl *ServerLookup) CheckServers(fn func(srv *metadata.Server) bool) {
	sl.lock.RLock()
	defer sl.lock.RUnlock()

	for _, srv := range sl.addressToServer {
		if !fn(srv) {
			return
		}
	}
}
