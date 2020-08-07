package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbreplication"
)

type ReplicationBackend interface {
	ReplicationInfo() pbreplication.InfoList
	ForwardRPC(method string, info structs.RPCInfo, args, reply interface{}) (bool, error)
}

type Replication struct {
	backend ReplicationBackend
}

func NewReplication(backend ReplicationBackend) (*Replication, error) {
	if backend == nil {
		return nil, fmt.Errorf("a ReplicationBackend is required")
	}

	return &Replication{
		backend: backend,
	}, nil
}

func (r *Replication) List(args *pbcommon.DCSpecificRequest, reply *pbreplication.InfoList) error {
	// This must be sent to the leader as the leader is the only server performing replication. So we
	// fix the args regardless of whether a stale query was requested.
	args.QueryOptions.RequireConsistent = true
	args.QueryOptions.AllowStale = false

	if done, err := r.backend.ForwardRPC("Replication.List", args, args, reply); done {
		return err
	}

	*reply = r.backend.ReplicationInfo()
	return nil
}
