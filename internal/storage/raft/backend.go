// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package raft

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-hclog"

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
// With Raft, writes and strongly consistent reads must be done on the leader.
// Backend implements a gRPC server, which followers will use to transparently
// forward operations to the leader. To do so, they will obtain a connection
// using Handle.DialLeader. Connections are cached for re-use, so when there's
// a new leader, you must call LeaderChanged to refresh the connection. Leaders
// must accept connections and hand them off by calling Backend.HandleConnection.
// Backend's gRPC client and server *DO NOT* handle TLS themselves, as they are
// intended to communicate over Consul's multiplexed server port (which handles
// TLS).
//
// For more information, see here:
// https://github.com/hashicorp/consul/tree/main/docs/resources#raft-storage-backend
//
// You must call Run before using the backend.
func NewBackend(h Handle, l hclog.Logger) (*Backend, error) {
	s, err := inmem.NewStore()
	if err != nil {
		return nil, err
	}
	b := &Backend{handle: h, store: s}
	b.forwardingServer = newForwardingServer(b)
	b.forwardingClient = newForwardingClient(h, l)
	return b, nil
}

// Handle provides glue for interacting with the Raft subsystem via existing
// machinery on consul.Server.
type Handle interface {
	// Apply the given log message.
	Apply(msg []byte) (any, error)

	// IsLeader determines if this server is the Raft leader (so can handle writes).
	IsLeader() bool

	// EnsureStrongConsistency checks the server is able to handle consistent reads by
	// verifying its leadership and checking the FSM has applied all queued writes.
	EnsureStrongConsistency(ctx context.Context) error

	// DialLeader dials a gRPC connection to the leader for forwarding.
	DialLeader() (*grpc.ClientConn, error)
}

// Backend is a Raft-backed storage backend implementation.
type Backend struct {
	handle Handle
	store  *inmem.Store

	forwardingServer *forwardingServer
	forwardingClient *forwardingClient
}

// Run until the given context is canceled. This method blocks, so should be
// called in a goroutine.
func (b *Backend) Run(ctx context.Context) {
	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		b.store.Run(groupCtx)
		return nil
	})

	group.Go(func() error {
		return b.forwardingServer.run(groupCtx)
	})

	group.Wait()
}

// Read implements the storage.Backend interface.
func (b *Backend) Read(ctx context.Context, consistency storage.ReadConsistency, id *pbresource.ID) (*pbresource.Resource, error) {
	// Easy case. Both leaders and followers can read from the local store.
	if consistency == storage.EventualConsistency {
		return b.store.Read(id)
	}

	if consistency != storage.StrongConsistency {
		return nil, fmt.Errorf("%w: unknown consistency: %s", storage.ErrInconsistent, consistency)
	}

	// We are the leader. Handle the request ourself.
	if b.handle.IsLeader() {
		return b.leaderRead(ctx, id)
	}

	// Forward the request to the leader.
	rsp, err := b.forwardingClient.read(ctx, &pbstorage.ReadRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return rsp.GetResource(), nil
}

func (b *Backend) leaderRead(ctx context.Context, id *pbresource.ID) (*pbresource.Resource, error) {
	if err := b.ensureStrongConsistency(ctx); err != nil {
		return nil, err
	}
	return b.store.Read(id)
}

// WriteCAS implements the storage.Backend interface.
func (b *Backend) WriteCAS(ctx context.Context, res *pbresource.Resource) (*pbresource.Resource, error) {
	req := &pbstorage.WriteRequest{Resource: res}

	if b.handle.IsLeader() {
		rsp, err := b.raftApply(&pbstorage.Log{
			Type: pbstorage.LogType_LOG_TYPE_WRITE,
			Request: &pbstorage.Log_Write{
				Write: req,
			},
		})
		if err != nil {
			return nil, err
		}
		return rsp.GetWrite().GetResource(), nil
	}

	rsp, err := b.forwardingClient.write(ctx, req)
	if err != nil {
		return nil, err
	}
	return rsp.GetResource(), nil
}

// DeleteCAS implements the storage.Backend interface.
func (b *Backend) DeleteCAS(ctx context.Context, id *pbresource.ID, version string) error {
	req := &pbstorage.DeleteRequest{
		Id:      id,
		Version: version,
	}

	if b.handle.IsLeader() {
		_, err := b.raftApply(&pbstorage.Log{
			Type: pbstorage.LogType_LOG_TYPE_DELETE,
			Request: &pbstorage.Log_Delete{
				Delete: req,
			},
		})
		return err
	}

	return b.forwardingClient.delete(ctx, req)
}

// List implements the storage.Backend interface.
func (b *Backend) List(ctx context.Context, consistency storage.ReadConsistency, resType storage.UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) ([]*pbresource.Resource, error) {
	// Easy case. Both leaders and followers can read from the local store.
	if consistency == storage.EventualConsistency {
		return b.store.List(resType, tenancy, namePrefix)
	}

	if consistency != storage.StrongConsistency {
		return nil, fmt.Errorf("%w: unknown consistency: %s", storage.ErrInconsistent, consistency)
	}

	// We are the leader. Handle the request ourself.
	if b.handle.IsLeader() {
		return b.leaderList(ctx, resType, tenancy, namePrefix)
	}

	// Forward the request to the leader.
	rsp, err := b.forwardingClient.list(ctx, &pbstorage.ListRequest{
		Type: &pbresource.Type{
			Group: resType.Group,
			Kind:  resType.Kind,
		},
		Tenancy:    tenancy,
		NamePrefix: namePrefix,
	})
	if err != nil {
		return nil, err
	}
	return rsp.GetResources(), nil
}

func (b *Backend) leaderList(ctx context.Context, resType storage.UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) ([]*pbresource.Resource, error) {
	if err := b.ensureStrongConsistency(ctx); err != nil {
		return nil, err
	}
	return b.store.List(resType, tenancy, namePrefix)
}

// WatchList implements the storage.Backend interface.
func (b *Backend) WatchList(_ context.Context, resType storage.UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) (storage.Watch, error) {
	return b.store.WatchList(resType, tenancy, namePrefix)
}

// ListByOwner implements the storage.Backend interface.
func (b *Backend) ListByOwner(_ context.Context, id *pbresource.ID) ([]*pbresource.Resource, error) {
	return b.store.ListByOwner(id)
}

// Apply is called by the FSM with the bytes of a Raft log entry, with Consul's
// envelope (i.e. type prefix and msgpack wrapper) stripped off.
func (b *Backend) Apply(buf []byte, idx uint64) any {
	var req pbstorage.Log
	if err := req.UnmarshalBinary(buf); err != nil {
		return fmt.Errorf("failed to decode request: %w", err)
	}

	switch req.Type {
	case pbstorage.LogType_LOG_TYPE_WRITE:
		res := req.GetWrite().GetResource()
		oldVsn := res.Version
		res.Version = strconv.Itoa(int(idx))

		if err := b.store.WriteCAS(res, oldVsn); err != nil {
			return err
		}

		return &pbstorage.LogResponse{
			Response: &pbstorage.LogResponse_Write{
				Write: &pbstorage.WriteResponse{Resource: res},
			},
		}
	case pbstorage.LogType_LOG_TYPE_DELETE:
		req := req.GetDelete()
		if err := b.store.DeleteCAS(req.Id, req.Version); err != nil {
			return err
		}
		return &pbstorage.LogResponse{
			Response: &pbstorage.LogResponse_Delete{},
		}
	}

	return fmt.Errorf("unexpected request type: %s", req.Type)
}

// LeaderChanged should be called whenever the current Raft leader changes, to
// drop and re-create the gRPC connection used for forwarding.
func (b *Backend) LeaderChanged() { b.forwardingClient.leaderChanged() }

// HandleConnection should be called whenever a forwarding connection is opened.
func (b *Backend) HandleConnection(conn net.Conn) { b.forwardingServer.listener.Handle(conn) }

// raftApply round trips the given request through the Raft log and FSM.
func (b *Backend) raftApply(req *pbstorage.Log) (*pbstorage.LogResponse, error) {
	msg, err := req.MarshalBinary()
	if err != nil {
		return nil, err
	}

	rsp, err := b.handle.Apply(msg)
	if err != nil {
		return nil, err
	}

	switch t := rsp.(type) {
	case *pbstorage.LogResponse:
		return t, nil
	default:
		return nil, fmt.Errorf("unexpected response from Raft apply: %T", rsp)
	}
}

func (b *Backend) ensureStrongConsistency(ctx context.Context) error {
	if err := b.handle.EnsureStrongConsistency(ctx); err != nil {
		return fmt.Errorf("%w: %v", storage.ErrInconsistent, err)
	}
	return nil
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
