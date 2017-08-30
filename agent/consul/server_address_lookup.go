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
	IdToServer      map[raft.ServerID]*metadata.Server
}

func NewServerLookup() *ServerLookup {
	return &ServerLookup{addressToServer: make(map[raft.ServerAddress]*metadata.Server), IdToServer: make(map[raft.ServerID]*metadata.Server)}
}

func (sa *ServerLookup) AddServer(server *metadata.Server) {
	sa.lock.Lock()
	defer sa.lock.Unlock()
	sa.addressToServer[raft.ServerAddress(server.Addr.String())] = server
	sa.IdToServer[raft.ServerID(server.ID)] = server
}

func (sa *ServerLookup) RemoveServer(server *metadata.Server) {
	sa.lock.Lock()
	defer sa.lock.Unlock()
	delete(sa.addressToServer, raft.ServerAddress(server.Addr.String()))
	delete(sa.IdToServer, raft.ServerID(server.ID))
}

// Implements the ServerAddressProvider interface
func (sa *ServerLookup) ServerAddr(id raft.ServerID) (raft.ServerAddress, error) {
	sa.lock.RLock()
	defer sa.lock.RUnlock()
	svr, ok := sa.IdToServer[id]
	if !ok {
		return "", fmt.Errorf("Could not find address for server id %v", id)
	}
	return raft.ServerAddress(svr.Addr.String()), nil
}

// GetServer looks up the server by address, returns a boolean if not found
func (sa *ServerLookup) GetServer(addr raft.ServerAddress) (*metadata.Server, bool) {
	sa.lock.RLock()
	defer sa.lock.RUnlock()
	svr, ok := sa.addressToServer[addr]
	if !ok {
		return nil, false
	}
	return svr, true
}
