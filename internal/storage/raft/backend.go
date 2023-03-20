package raft

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/proto-public/pbresource"

	pbstorage "github.com/hashicorp/consul/proto/private/pbstorage"
)

// NewBackend returns a storage backend that uses Raft for durable persistence
// and serves reads from an in-memory database. It's suitable for production use.
//
// It's not an entirely clean abstraction because rather than owning the Raft
// subsystem directly, it has to integrate with the existing FSM and related
// machinery from before generic resources.
//
// The given Handle will be used to apply logs and interrogate leadership state.
// In certain restricted circumstances, Handle may be nil, such as during tests
// that only exercise snapshot restoration, or when initializing a throwaway FSM
// during peers.json recovery - but calling any of the data access methods (read
// or write) will result in a panic.
//
// Calling WriteCAS, DeleteCAS, or ReadConsistent on a follower will result in
// a storage.ErrInconsistent error. It's expected that for these operations, the
// whole thing will be forwarded to the leader at the RPC level.
//
// You must call Run before using the backend.
func NewBackend(h Handle) (*Backend, error) {
	s, err := inmem.NewStore()
	if err != nil {
		return nil, err
	}
	return &Backend{h, s}, nil
}

// Handle provides glue for interacting with the Raft subsystem via existing
// machinery on consul.Server.
type Handle interface {
	// Apply the given log message.
	Apply(msg []byte) (any, error)

	// IsLeader determines if this server is the Raft leader (so can handle writes).
	IsLeader() bool

	// EnsureConsistency checks the server is able to handle consistent reads by
	// verifying its leadership and checking the FSM has applied all queued writes.
	EnsureConsistency(ctx context.Context) error
}

// Backend is a Raft-backed storage backend implementation.
type Backend struct {
	handle Handle
	store  *inmem.Store
}

// Run until the given context is canceled. This method blocks, so should be
// called in a goroutine.
func (b *Backend) Run(ctx context.Context) { b.store.Run(ctx) }

// Read implements the storage.Backend interface.
func (b *Backend) Read(ctx context.Context, consistency storage.ReadConsistency, id *pbresource.ID) (*pbresource.Resource, error) {
	if err := b.ensureConsistency(ctx, consistency); err != nil {
		return nil, err
	}
	return b.store.Read(id)
}

// WriteCAS implements the storage.Backend interface. It can only be called on
// a Raft leader.
func (b *Backend) WriteCAS(_ context.Context, res *pbresource.Resource) (*pbresource.Resource, error) {
	if !b.handle.IsLeader() {
		return nil, storage.ErrInconsistent
	}

	rsp, err := b.roundTrip(&pbstorage.Request{
		Op:      pbstorage.Request_OP_WRITE,
		Subject: &pbstorage.Request_Resource{Resource: res},
	})
	if err != nil {
		return nil, err
	}
	return rsp.GetResource(), nil
}

// DeleteCAS implements the storage.Backend interface. It can only be called on
// a Raft leader.
func (b *Backend) DeleteCAS(_ context.Context, id *pbresource.ID, version string) error {
	if !b.handle.IsLeader() {
		return storage.ErrInconsistent
	}

	_, err := b.roundTrip(&pbstorage.Request{
		Op:      pbstorage.Request_OP_DELETE,
		Subject: &pbstorage.Request_Id{Id: id},
		Version: version,
	})
	return err
}

// List implements the storage.Backend interface.
func (b *Backend) List(ctx context.Context, consistency storage.ReadConsistency, resType storage.UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) ([]*pbresource.Resource, error) {
	if err := b.ensureConsistency(ctx, consistency); err != nil {
		return nil, err
	}
	return b.store.List(resType, tenancy, namePrefix)
}

// WatchList implements the storage.Backend interface.
func (b *Backend) WatchList(_ context.Context, resType storage.UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) (storage.Watch, error) {
	return b.store.WatchList(resType, tenancy, namePrefix)
}

// OwnerReferences implements the storage.Backend interface.
func (b *Backend) OwnerReferences(_ context.Context, id *pbresource.ID) ([]*pbresource.ID, error) {
	return b.store.OwnerReferences(id)
}

// Apply is called by the FSM with the bytes of a Raft log entry, with Consul's
// envelope (i.e. type prefix and msgpack wrapper) stripped off.
func (b *Backend) Apply(buf []byte, idx uint64) any {
	var req pbstorage.Request
	if err := req.UnmarshalBinary(buf); err != nil {
		return fmt.Errorf("failed to decode request: %w", err)
	}

	switch req.Op {
	case pbstorage.Request_OP_WRITE:
		res := req.GetResource()
		oldVsn := res.Version
		res.Version = strconv.Itoa(int(idx))

		if err := b.store.WriteCAS(res, oldVsn); err != nil {
			return err
		}

		return &pbstorage.Response{Resource: res}
	case pbstorage.Request_OP_DELETE:
		if err := b.store.DeleteCAS(req.GetId(), req.Version); err != nil {
			return err
		}
		return &pbstorage.Response{}
	}
	return fmt.Errorf("unexpected op: %d", req.Op)
}

// roundTrip the given request through the Raft log and FSM.
func (b *Backend) roundTrip(req *pbstorage.Request) (*pbstorage.Response, error) {
	msg, err := req.MarshalBinary()
	if err != nil {
		return nil, err
	}

	rsp, err := b.handle.Apply(msg)
	if err != nil {
		return nil, err
	}

	switch t := rsp.(type) {
	case *pbstorage.Response:
		return t, nil
	case error:
		return nil, t
	default:
		return nil, fmt.Errorf("unexpected response from Raft apply: %T", rsp)
	}
}

func (b *Backend) ensureConsistency(ctx context.Context, consistency storage.ReadConsistency) error {
	switch consistency {
	case storage.EventualConsistency:
		return nil
	case storage.StrongConsistency:
		err := b.handle.EnsureConsistency(ctx)
		if errors.Is(err, structs.ErrNotReadyForConsistentReads) {
			return storage.ErrInconsistent
		}
		return err
	default:
		return fmt.Errorf("%w: unknown consistency mode (%s)", storage.ErrInconsistent, consistency)
	}
}

// Snapshot obtains a point-in-time snapshot of the backend's state, so that it
// can be written to disk as a backup or sent to bootstrap a follower.
func (b *Backend) Snapshot() (*Snapshot, error) {
	s, err := b.store.Snapshot()
	if err != nil {
		return nil, err
	}
	return &Snapshot{s}, nil
}

// Snapshot is a point-in-time snapshot of a backend's state.
type Snapshot struct{ s *inmem.Snapshot }

// Next returns the next resource in the snapshot, protobuf encoded. nil bytes
// will be returned when the end of the snapshot has been reached.
func (s *Snapshot) Next() ([]byte, error) {
	res := s.s.Next()
	if res == nil {
		return nil, nil
	}
	return res.MarshalBinary()
}

// Restore starts the process of restoring a snapshot (i.e. from an on-disk
// backup, or to bootstrap from a leader).
//
// Callers *must* call Abort or Commit when done, to free resources.
func (b *Backend) Restore() (*Restoration, error) {
	r, err := b.store.Restore()
	if err != nil {
		return nil, err
	}
	return &Restoration{r}, nil
}

// Restoration is a handle that can be used to restore a snapshot.
type Restoration struct{ r *inmem.Restoration }

// Apply the given protobuf-encoded resource to the backend.
func (r *Restoration) Apply(msg []byte) error {
	var res pbresource.Resource
	if err := res.UnmarshalBinary(msg); err != nil {
		return err
	}
	return r.r.Apply(&res)
}

// Commit the restoration.
func (r *Restoration) Commit() { r.r.Commit() }

// Abort the restoration. It's safe to always call this in a defer statement
// because aborting a committed restoration is a no-op.
func (r *Restoration) Abort() { r.r.Abort() }
