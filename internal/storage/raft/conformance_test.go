package raft_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/internal/storage/conformance"
	"github.com/hashicorp/consul/internal/storage/raft"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestBackend_Conformance(t *testing.T) {
	// We run the conformance suite against a backend that simulates reads being
	// handled by an eventually consistent follower. Writes will be replicated to
	// the follower with a small amount of lag (you can use the -short flag to
	// disable this).
	conformance.Test(t, conformance.TestOptions{
		NewBackend: func(t *testing.T) storage.Backend {
			return newRaftCluster(t)
		},
	})
}

func newRaftCluster(t *testing.T) *raftCluster {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	leaderHandle := &leaderHandle{replCh: make(chan []byte, 10)}
	leader, err := raft.NewBackend(leaderHandle)
	require.NoError(t, err)
	go leader.Run(ctx)
	leaderHandle.backend = leader

	follower, err := raft.NewBackend(followerHandle{})
	require.NoError(t, err)
	go follower.Run(ctx)

	go leaderHandle.replicate(t, follower)

	return &raftCluster{leader, follower}
}

type raftCluster struct {
	leader, follower *raft.Backend
}

func (c *raftCluster) Read(ctx context.Context, id *pbresource.ID) (*pbresource.Resource, error) {
	return c.follower.Read(ctx, id)
}

func (c *raftCluster) ReadConsistent(ctx context.Context, id *pbresource.ID) (*pbresource.Resource, error) {
	return c.leader.Read(ctx, id)
}

func (c *raftCluster) WriteCAS(ctx context.Context, res *pbresource.Resource, version string) (*pbresource.Resource, error) {
	return c.leader.WriteCAS(ctx, res, version)
}

func (c *raftCluster) DeleteCAS(ctx context.Context, id *pbresource.ID, version string) error {
	return c.leader.DeleteCAS(ctx, id, version)
}

func (c *raftCluster) List(ctx context.Context, resType storage.UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) ([]*pbresource.Resource, error) {
	return c.follower.List(ctx, resType, tenancy, namePrefix)
}

func (c *raftCluster) WatchList(ctx context.Context, resType storage.UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) (storage.Watch, error) {
	return c.follower.WatchList(ctx, resType, tenancy, namePrefix)
}

func (c *raftCluster) OwnerReferences(ctx context.Context, id *pbresource.ID) ([]*pbresource.ID, error) {
	return c.follower.OwnerReferences(ctx, id)
}

type followerHandle struct{}

func (followerHandle) IsLeader() bool { return false }

func (followerHandle) EnsureConsistency(context.Context) error {
	return structs.ErrNotReadyForConsistentReads
}

func (followerHandle) Apply([]byte) (any, error) {
	return nil, errors.New("follower should not try to handle writes")
}

type leaderHandle struct {
	backend *raft.Backend
	replCh  chan []byte
}

func (leaderHandle) IsLeader() bool                          { return true }
func (leaderHandle) EnsureConsistency(context.Context) error { return nil }

func (h leaderHandle) Apply(msg []byte) (any, error) {
	// Apply the operation to the leader synchronously and capture its response
	// to return to the caller.
	rsp := h.backend.Apply(msg)

	// Replicate the operation to the follower asynchronously.
	h.replCh <- msg

	return rsp, nil
}

func (h leaderHandle) replicate(t *testing.T, follower *raft.Backend) {
	doneCh := make(chan struct{})
	t.Cleanup(func() { close(doneCh) })

	timer := time.NewTimer(replicationLag())
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			select {
			case l := <-h.replCh:
				_ = follower.Apply(l)
			default:
			}
			timer.Reset(replicationLag())
		case <-doneCh:
			return
		}
	}
}

func replicationLag() time.Duration {
	if testing.Short() {
		return 0
	}
	return time.Duration(rand.Intn(50)) * time.Millisecond
}
