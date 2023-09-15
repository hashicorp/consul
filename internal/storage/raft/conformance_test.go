// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft_test

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/internal/storage/conformance"
	"github.com/hashicorp/consul/internal/storage/raft"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestBackend_Conformance(t *testing.T) {
	t.Run("Leader", func(t *testing.T) {
		conformance.Test(t, conformance.TestOptions{
			NewBackend: func(t *testing.T) storage.Backend {
				leader, _ := newRaftCluster(t)
				return leader
			},
			SupportsStronglyConsistentList: true,
		})
	})

	t.Run("Follower", func(t *testing.T) {
		conformance.Test(t, conformance.TestOptions{
			NewBackend: func(t *testing.T) storage.Backend {
				_, follower := newRaftCluster(t)
				return follower
			},
			SupportsStronglyConsistentList: true,
		})
	})
}

func newRaftCluster(t *testing.T) (*raft.Backend, *raft.Backend) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	lc, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	lh := &leaderHandle{replCh: make(chan log, 10)}
	leader, err := raft.NewBackend(lh, testutil.Logger(t))
	require.NoError(t, err)
	lh.backend = leader
	go leader.Run(ctx)

	go func() {
		for {
			conn, err := lis.Accept()
			if errors.Is(err, net.ErrClosed) {
				return
			}
			require.NoError(t, err)
			go leader.HandleConnection(conn)
		}
	}()

	follower, err := raft.NewBackend(&followerHandle{leaderConn: lc}, testutil.Logger(t))
	require.NoError(t, err)
	go follower.Run(ctx)
	follower.LeaderChanged()

	go lh.replicate(t, follower)

	return leader, follower
}

type followerHandle struct {
	leaderConn *grpc.ClientConn
}

func (followerHandle) Apply([]byte) (any, error) {
	return nil, errors.New("not leader")
}

func (followerHandle) IsLeader() bool {
	return false
}

func (followerHandle) EnsureStrongConsistency(context.Context) error {
	return errors.New("not leader")
}

func (f *followerHandle) DialLeader() (*grpc.ClientConn, error) {
	return f.leaderConn, nil
}

type leaderHandle struct {
	index  uint64
	replCh chan log

	backend *raft.Backend
}

type log struct {
	idx uint64
	msg []byte
}

func (l *leaderHandle) Apply(msg []byte) (any, error) {
	idx := atomic.AddUint64(&l.index, 1)

	// Apply the operation to the leader synchronously and capture its response
	// to return to the caller.
	rsp := l.backend.Apply(msg, idx)

	// Replicate the operation to the follower asynchronously.
	l.replCh <- log{idx, msg}

	if err, ok := rsp.(error); ok {
		return nil, err
	}
	return rsp, nil
}

func (leaderHandle) IsLeader() bool {
	return true
}

func (leaderHandle) EnsureStrongConsistency(context.Context) error {
	return nil
}

func (leaderHandle) DialLeader() (*grpc.ClientConn, error) {
	return nil, errors.New("leader should not dial itself")
}

func (h *leaderHandle) replicate(t *testing.T, follower *raft.Backend) {
	doneCh := make(chan struct{})
	t.Cleanup(func() { close(doneCh) })

	timer := time.NewTimer(replicationLag())
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			select {
			case l := <-h.replCh:
				_ = follower.Apply(l.msg, l.idx)
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
