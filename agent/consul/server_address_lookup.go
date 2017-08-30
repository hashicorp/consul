package consul

import (
	"fmt"
	"sync"

	"github.com/hashicorp/raft"
)

// serverIdToAddress is a map from id to address for servers in the LAN pool.
// used for fast lookup to satisfy the ServerAddressProvider interface
type ServerAddressLookup struct {
	serverIdToAddress sync.Map
}

func NewServerAddressLookup() *ServerAddressLookup {
	return &ServerAddressLookup{}
}

func (sa *ServerAddressLookup) AddServer(id string, address string) {
	sa.serverIdToAddress.Store(id, address)
}

func (sa *ServerAddressLookup) RemoveServer(id string) {
	sa.serverIdToAddress.Delete(id)
}

func (sa *ServerAddressLookup) ServerAddr(id raft.ServerID) (raft.ServerAddress, error) {
	val, ok := sa.serverIdToAddress.Load(string(id))
	if !ok {
		return "", fmt.Errorf("Could not find address for server id %v", id)
	}
	return raft.ServerAddress(val.(string)), nil
}
