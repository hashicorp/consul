package consul

import (
	"context"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/storage/raft"
)

// raftHandle is the glue layer between the Raft resource storage backend and
// the exising Raft logic in Server.
type raftHandle struct{ s *Server }

func (h *raftHandle) IsLeader() bool {
	return h.s.IsLeader()
}

func (h *raftHandle) EnsureConsistency(ctx context.Context) error {
	return h.s.consistentReadWithContext(ctx)
}

func (h *raftHandle) Apply(msg []byte) (any, error) {
	return h.s.raftApplyEncoded(
		structs.ResourceOperationType,
		append([]byte{uint8(structs.ResourceOperationType)}, msg...),
	)
}

var _ raft.Handle = (*raftHandle)(nil)
