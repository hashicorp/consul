package consul

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/lib"
	"golang.org/x/time/rate"
)

const (
	// replicationMaxRetryWait is the maximum number of seconds to wait between
	// failed blocking queries when backing off.
	replicationDefaultMaxRetryWait = 120 * time.Second

	replicationDefaultRate = 1
)

type ReplicatorDelegate interface {
	Replicate(ctx context.Context, lastRemoteIndex uint64) (index uint64, exit bool, err error)
}

type ReplicatorConfig struct {
	// Name to be used in various logging
	Name string
	// Delegate to perform each round of replication
	Delegate ReplicatorDelegate
	// The number of replication rounds per second that are allowed
	Rate int
	// The number of replication rounds that can be done in a burst
	Burst int
	// Minimum number of RPC failures to ignore before backing off
	MinFailures int
	// Maximum wait time between failing RPCs
	MaxRetryWait time.Duration
	// Where to send our logs
	Logger *log.Logger
}

type Replicator struct {
	name            string
	limiter         *rate.Limiter
	waiter          *lib.RetryWaiter
	delegate        ReplicatorDelegate
	logger          *log.Logger
	lastRemoteIndex uint64
}

func NewReplicator(config *ReplicatorConfig) (*Replicator, error) {
	if config == nil {
		return nil, fmt.Errorf("Cannot create the Replicator without a config")
	}
	if config.Delegate == nil {
		return nil, fmt.Errorf("Cannot create the Replicator without a Delegate set in the config")
	}
	if config.Logger == nil {
		config.Logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	limiter := rate.NewLimiter(rate.Limit(config.Rate), config.Burst)

	maxWait := config.MaxRetryWait
	if maxWait == 0 {
		maxWait = replicationDefaultMaxRetryWait
	}

	minFailures := config.MinFailures
	if minFailures < 0 {
		minFailures = 0
	}
	waiter := lib.NewRetryWaiter(minFailures, 0*time.Second, maxWait, lib.NewJitterRandomStagger(10))
	return &Replicator{
		name:     config.Name,
		limiter:  limiter,
		waiter:   waiter,
		delegate: config.Delegate,
		logger:   config.Logger,
	}, nil
}

func (r *Replicator) Run(ctx context.Context) error {
	defer r.logger.Printf("[INFO] replication: stopped %s replication", r.name)

	for {
		// This ensures we aren't doing too many successful replication rounds - mostly useful when
		// the data within the primary datacenter is changing rapidly but we try to limit the amount
		// of resources replication into the secondary datacenter should take
		if err := r.limiter.Wait(ctx); err != nil {
			return nil
		}

		// Perform a single round of replication
		index, exit, err := r.delegate.Replicate(ctx, atomic.LoadUint64(&r.lastRemoteIndex))
		if exit {
			// the replication function told us to exit
			return nil
		}

		if err != nil {
			// reset the lastRemoteIndex when there is an RPC failure. This should cause a full sync to be done during
			// the next round of replication
			atomic.StoreUint64(&r.lastRemoteIndex, 0)
			r.logger.Printf("[WARN] replication: %s replication error (will retry if still leader): %v", r.name, err)
		} else {
			atomic.StoreUint64(&r.lastRemoteIndex, index)
			r.logger.Printf("[DEBUG] replication: %s replication completed through remote index %d", r.name, index)
		}

		select {
		case <-ctx.Done():
			return nil
		// wait some amount of time to prevent churning through many replication rounds while replication is failing
		case <-r.waiter.WaitIfErr(err):
			// do nothing
		}
	}
}

func (r *Replicator) Index() uint64 {
	return atomic.LoadUint64(&r.lastRemoteIndex)
}

type ReplicatorFunc func(ctx context.Context, lastRemoteIndex uint64) (index uint64, exit bool, err error)

type FunctionReplicator struct {
	ReplicateFn ReplicatorFunc
}

func (r *FunctionReplicator) Replicate(ctx context.Context, lastRemoteIndex uint64) (uint64, bool, error) {
	return r.ReplicateFn(ctx, lastRemoteIndex)
}

type IndexReplicatorDiff struct {
	NumUpdates   int
	Updates      interface{}
	NumDeletions int
	Deletions    interface{}
}

type IndexReplicatorDelegate interface {
	// SingularNoun is the singular form of the item being replicated.
	SingularNoun() string

	// PluralNoun is the plural form of the item being replicated.
	PluralNoun() string

	// Name to use when emitting metrics
	MetricName() string

	// FetchRemote retrieves items newer than the provided index from the
	// remote datacenter (for diffing purposes).
	FetchRemote(lastRemoteIndex uint64) (int, interface{}, uint64, error)

	// FetchLocal retrieves items from the current datacenter (for diffing
	// purposes).
	FetchLocal() (int, interface{}, error)

	DiffRemoteAndLocalState(local interface{}, remote interface{}, lastRemoteIndex uint64) (*IndexReplicatorDiff, error)

	PerformDeletions(ctx context.Context, deletions interface{}) (exit bool, err error)

	PerformUpdates(ctx context.Context, updates interface{}) (exit bool, err error)
}

type IndexReplicator struct {
	Delegate IndexReplicatorDelegate
	Logger   *log.Logger
}

func (r *IndexReplicator) Replicate(ctx context.Context, lastRemoteIndex uint64) (uint64, bool, error) {
	fetchStart := time.Now()
	lenRemote, remote, remoteIndex, err := r.Delegate.FetchRemote(lastRemoteIndex)
	metrics.MeasureSince([]string{"leader", "replication", r.Delegate.MetricName(), "fetch"}, fetchStart)

	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve %s: %v", r.Delegate.PluralNoun(), err)
	}

	r.Logger.Printf("[DEBUG] replication: finished fetching %s: %d", r.Delegate.PluralNoun(), lenRemote)

	// Need to check if we should be stopping. This will be common as the fetching process is a blocking
	// RPC which could have been hanging around for a long time and during that time leadership could
	// have been lost.
	select {
	case <-ctx.Done():
		return 0, true, nil
	default:
		// do nothing
	}

	// Measure everything after the remote query, which can block for long
	// periods of time. This metric is a good measure of how expensive the
	// replication process is.
	defer metrics.MeasureSince([]string{"leader", "replication", r.Delegate.MetricName(), "apply"}, time.Now())

	lenLocal, local, err := r.Delegate.FetchLocal()
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve local %s: %v", r.Delegate.PluralNoun(), err)
	}

	// If the remote index ever goes backwards, it's a good indication that
	// the remote side was rebuilt and we should do a full sync since we
	// can't make any assumptions about what's going on.
	//
	// Resetting lastRemoteIndex to 0 will work because we never consider local
	// raft indices. Instead we compare the raft modify index in the response object
	// with the lastRemoteIndex (only when we already have a config entry of the same kind/name)
	// to determine if an update is needed. Resetting lastRemoteIndex to 0 then has the affect
	// of making us think all the local state is out of date and any matching entries should
	// still be updated.
	//
	// The lastRemoteIndex is not used when the entry exists either only in the local state or
	// only in the remote state. In those situations we need to either delete it or create it.
	if remoteIndex < lastRemoteIndex {
		r.Logger.Printf("[WARN] replication: %[1]s replication remote index moved backwards (%d to %d), forcing a full %[1]s sync", r.Delegate.SingularNoun(), lastRemoteIndex, remoteIndex)
		lastRemoteIndex = 0
	}

	r.Logger.Printf("[DEBUG] replication: %s replication - local: %d, remote: %d", r.Delegate.SingularNoun(), lenLocal, lenRemote)

	// Calculate the changes required to bring the state into sync and then
	// apply them.
	diff, err := r.Delegate.DiffRemoteAndLocalState(local, remote, lastRemoteIndex)
	if err != nil {
		return 0, false, fmt.Errorf("failed to diff %s local and remote states: %v", r.Delegate.SingularNoun(), err)
	}

	r.Logger.Printf("[DEBUG] replication: %s replication - deletions: %d, updates: %d", r.Delegate.SingularNoun(), diff.NumDeletions, diff.NumUpdates)

	if diff.NumDeletions > 0 {
		r.Logger.Printf("[DEBUG] replication: %s replication - performing %d deletions", r.Delegate.SingularNoun(), diff.NumDeletions)

		exit, err := r.Delegate.PerformDeletions(ctx, diff.Deletions)
		if exit {
			return 0, true, nil
		}

		if err != nil {
			return 0, false, fmt.Errorf("failed to apply local %s deletions: %v", r.Delegate.SingularNoun(), err)
		}
		r.Logger.Printf("[DEBUG] replication: %s replication - finished deletions", r.Delegate.SingularNoun())
	}

	if diff.NumUpdates > 0 {
		r.Logger.Printf("[DEBUG] replication: %s replication - performing %d updates", r.Delegate.SingularNoun(), diff.NumUpdates)

		exit, err := r.Delegate.PerformUpdates(ctx, diff.Updates)
		if exit {
			return 0, true, nil
		}

		if err != nil {
			return 0, false, fmt.Errorf("failed to apply local %s updates: %v", r.Delegate.SingularNoun(), err)
		}
		r.Logger.Printf("[DEBUG] replication: %s replication - finished updates", r.Delegate.SingularNoun())
	}

	// Return the index we got back from the remote side, since we've synced
	// up with the remote state as of that index.
	return remoteIndex, false, nil
}
