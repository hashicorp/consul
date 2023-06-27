// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/lib/routine"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestReplicationRestart(t *testing.T) {
	mgr := routine.NewManager(testutil.Logger(t))

	config := ReplicatorConfig{
		Name: "mock",
		Delegate: &FunctionReplicator{
			ReplicateFn: func(ctx context.Context, lastRemoteIndex uint64, logger hclog.Logger) (uint64, bool, error) {
				return 1, false, nil
			},
			Name: "foo",
		},

		Rate:  1,
		Burst: 1,
	}

	repl, err := NewReplicator(&config)
	require.NoError(t, err)

	mgr.Start(context.Background(), "mock", repl.Run)
	mgr.Stop("mock")
	mgr.Start(context.Background(), "mock", repl.Run)
	// Previously this would have segfaulted
	mgr.Stop("mock")
}

type indexReplicatorTestDelegate struct {
	mock.Mock
}

func (d *indexReplicatorTestDelegate) SingularNoun() string {
	return "test"
}

func (d *indexReplicatorTestDelegate) PluralNoun() string {
	return "tests"
}

func (d *indexReplicatorTestDelegate) MetricName() string {
	return "test"
}

func (d *indexReplicatorTestDelegate) FetchRemote(lastRemoteIndex uint64) (int, interface{}, uint64, error) {
	ret := d.Called(lastRemoteIndex)
	return ret.Int(0), ret.Get(1), ret.Get(2).(uint64), ret.Error(3)
}

func (d *indexReplicatorTestDelegate) FetchLocal() (int, interface{}, error) {
	ret := d.Called()
	return ret.Int(0), ret.Get(1), ret.Error(2)
}

func (d *indexReplicatorTestDelegate) DiffRemoteAndLocalState(local interface{}, remote interface{}, lastRemoteIndex uint64) (*IndexReplicatorDiff, error) {
	ret := d.Called(local, remote, lastRemoteIndex)
	return ret.Get(0).(*IndexReplicatorDiff), ret.Error(1)
}

func (d *indexReplicatorTestDelegate) PerformDeletions(ctx context.Context, deletions interface{}) (exit bool, err error) {
	// ignore the context for the call
	ret := d.Called(deletions)
	return ret.Bool(0), ret.Error(1)
}

func (d *indexReplicatorTestDelegate) PerformUpdates(ctx context.Context, updates interface{}) (exit bool, err error) {
	// ignore the context for the call
	ret := d.Called(updates)
	return ret.Bool(0), ret.Error(1)
}

func TestIndexReplicator(t *testing.T) {
	t.Parallel()

	t.Run("Remote Fetch Error", func(t *testing.T) {
		delegate := &indexReplicatorTestDelegate{}

		replicator := IndexReplicator{
			Delegate: delegate,
			Logger:   testutil.Logger(t),
		}

		delegate.On("FetchRemote", uint64(0)).Return(0, nil, uint64(0), fmt.Errorf("induced error"))

		idx, done, err := replicator.Replicate(context.Background(), 0, nil)

		require.Equal(t, uint64(0), idx)
		require.False(t, done)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to retrieve tests: induced error")
		delegate.AssertExpectations(t)
	})

	t.Run("Local Fetch Error", func(t *testing.T) {
		delegate := &indexReplicatorTestDelegate{}

		replicator := IndexReplicator{
			Delegate: delegate,
			Logger:   testutil.Logger(t),
		}

		delegate.On("FetchRemote", uint64(3)).Return(1, nil, uint64(1), nil)
		delegate.On("FetchLocal").Return(0, nil, fmt.Errorf("induced error"))

		idx, done, err := replicator.Replicate(context.Background(), 3, nil)

		require.Equal(t, uint64(0), idx)
		require.False(t, done)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to retrieve local tests: induced error")
		delegate.AssertExpectations(t)
	})

	t.Run("Diff Error", func(t *testing.T) {
		delegate := &indexReplicatorTestDelegate{}

		replicator := IndexReplicator{
			Delegate: delegate,
			Logger:   testutil.Logger(t),
		}

		delegate.On("FetchRemote", uint64(3)).Return(1, nil, uint64(1), nil)
		delegate.On("FetchLocal").Return(1, nil, nil)
		// this also is verifying that when the remote index goes backwards then we reset the index to 0
		delegate.On("DiffRemoteAndLocalState", nil, nil, uint64(0)).Return(&IndexReplicatorDiff{}, fmt.Errorf("induced error"))

		idx, done, err := replicator.Replicate(context.Background(), 3, nil)

		require.Equal(t, uint64(0), idx)
		require.False(t, done)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to diff test local and remote states: induced error")
		delegate.AssertExpectations(t)
	})

	t.Run("No Change", func(t *testing.T) {
		delegate := &indexReplicatorTestDelegate{}

		replicator := IndexReplicator{
			Delegate: delegate,
			Logger:   testutil.Logger(t),
		}

		delegate.On("FetchRemote", uint64(3)).Return(1, nil, uint64(4), nil)
		delegate.On("FetchLocal").Return(1, nil, nil)
		delegate.On("DiffRemoteAndLocalState", nil, nil, uint64(3)).Return(&IndexReplicatorDiff{}, nil)

		idx, done, err := replicator.Replicate(context.Background(), 3, nil)

		require.Equal(t, uint64(4), idx)
		require.False(t, done)
		require.NoError(t, err)
		delegate.AssertExpectations(t)
	})

	t.Run("Deletion Error", func(t *testing.T) {
		delegate := &indexReplicatorTestDelegate{}

		replicator := IndexReplicator{
			Delegate: delegate,
			Logger:   testutil.Logger(t),
		}

		delegate.On("FetchRemote", uint64(3)).Return(1, nil, uint64(4), nil)
		delegate.On("FetchLocal").Return(1, nil, nil)
		delegate.On("DiffRemoteAndLocalState", nil, nil, uint64(3)).Return(&IndexReplicatorDiff{NumDeletions: 1}, nil)
		delegate.On("PerformDeletions", nil).Return(false, fmt.Errorf("induced error"))

		idx, done, err := replicator.Replicate(context.Background(), 3, nil)

		require.Equal(t, uint64(0), idx)
		require.False(t, done)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to apply local test deletions: induced error")
		delegate.AssertExpectations(t)
	})

	t.Run("Deletion Exit", func(t *testing.T) {
		delegate := &indexReplicatorTestDelegate{}

		replicator := IndexReplicator{
			Delegate: delegate,
			Logger:   testutil.Logger(t),
		}

		delegate.On("FetchRemote", uint64(3)).Return(1, nil, uint64(4), nil)
		delegate.On("FetchLocal").Return(1, nil, nil)
		delegate.On("DiffRemoteAndLocalState", nil, nil, uint64(3)).Return(&IndexReplicatorDiff{NumDeletions: 1}, nil)
		delegate.On("PerformDeletions", nil).Return(true, nil)

		idx, done, err := replicator.Replicate(context.Background(), 3, nil)

		require.Equal(t, uint64(0), idx)
		require.True(t, done)
		require.NoError(t, err)
		delegate.AssertExpectations(t)
	})

	t.Run("Update Error", func(t *testing.T) {
		delegate := &indexReplicatorTestDelegate{}

		replicator := IndexReplicator{
			Delegate: delegate,
			Logger:   testutil.Logger(t),
		}

		delegate.On("FetchRemote", uint64(3)).Return(1, nil, uint64(4), nil)
		delegate.On("FetchLocal").Return(1, nil, nil)
		delegate.On("DiffRemoteAndLocalState", nil, nil, uint64(3)).Return(&IndexReplicatorDiff{NumUpdates: 1}, nil)
		delegate.On("PerformUpdates", nil).Return(false, fmt.Errorf("induced error"))

		idx, done, err := replicator.Replicate(context.Background(), 3, nil)

		require.Equal(t, uint64(0), idx)
		require.False(t, done)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to apply local test updates: induced error")
		delegate.AssertExpectations(t)
	})

	t.Run("Update Exit", func(t *testing.T) {
		delegate := &indexReplicatorTestDelegate{}

		replicator := IndexReplicator{
			Delegate: delegate,
			Logger:   testutil.Logger(t),
		}

		delegate.On("FetchRemote", uint64(3)).Return(1, nil, uint64(4), nil)
		delegate.On("FetchLocal").Return(1, nil, nil)
		delegate.On("DiffRemoteAndLocalState", nil, nil, uint64(3)).Return(&IndexReplicatorDiff{NumUpdates: 1}, nil)
		delegate.On("PerformUpdates", nil).Return(true, nil)

		idx, done, err := replicator.Replicate(context.Background(), 3, nil)

		require.Equal(t, uint64(0), idx)
		require.True(t, done)
		require.NoError(t, err)
		delegate.AssertExpectations(t)
	})

	t.Run("All Good", func(t *testing.T) {
		delegate := &indexReplicatorTestDelegate{}

		replicator := IndexReplicator{
			Delegate: delegate,
			Logger:   testutil.Logger(t),
		}

		delegate.On("FetchRemote", uint64(3)).Return(3, "bcd", uint64(4), nil)
		delegate.On("FetchLocal").Return(1, "a", nil)
		delegate.On("DiffRemoteAndLocalState", "a", "bcd", uint64(3)).Return(&IndexReplicatorDiff{NumDeletions: 1, Deletions: "a", NumUpdates: 3, Updates: "bcd"}, nil)
		delegate.On("PerformDeletions", "a").Return(false, nil)
		delegate.On("PerformUpdates", "bcd").Return(false, nil)

		idx, done, err := replicator.Replicate(context.Background(), 3, nil)

		require.Equal(t, uint64(4), idx)
		require.False(t, done)
		require.NoError(t, err)
		delegate.AssertExpectations(t)
	})
}
