package consul

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	"golang.org/x/time/rate"
)

const (
	// replicationMaxRetryWait is the maximum number of seconds to wait between
	// failed blocking queries when backing off.
	replicationDefaultMaxRetryWait = 120 * time.Second

	replicationDefaultRate = 1
)

type ReplicatorDelegate interface {
	Replicate(ctx context.Context, lastRemoteIndex uint64, logger hclog.Logger) (index uint64, exit bool, err error)
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
	Logger hclog.Logger
	// Function to use for determining if an error should be suppressed
	SuppressErrorLog func(err error) bool
}

type Replicator struct {
	limiter          *rate.Limiter
	waiter           *lib.RetryWaiter
	delegate         ReplicatorDelegate
	logger           hclog.Logger
	lastRemoteIndex  uint64
	suppressErrorLog func(err error) bool
}

func NewReplicator(config *ReplicatorConfig) (*Replicator, error) {
	if config == nil {
		return nil, fmt.Errorf("Cannot create the Replicator without a config")
	}
	if config.Delegate == nil {
		return nil, fmt.Errorf("Cannot create the Replicator without a Delegate set in the config")
	}
	if config.Logger == nil {
		logger := hclog.New(&hclog.LoggerOptions{})
		config.Logger = logger
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
		limiter:          limiter,
		waiter:           waiter,
		delegate:         config.Delegate,
		logger:           config.Logger.Named(logging.Replication).Named(config.Name),
		suppressErrorLog: config.SuppressErrorLog,
	}, nil
}

func (r *Replicator) Run(ctx context.Context) error {
	defer r.logger.Info("stopped replication")

	for {
		// This ensures we aren't doing too many successful replication rounds - mostly useful when
		// the data within the primary datacenter is changing rapidly but we try to limit the amount
		// of resources replication into the secondary datacenter should take
		if err := r.limiter.Wait(ctx); err != nil {
			return nil
		}

		// Perform a single round of replication
		index, exit, err := r.delegate.Replicate(ctx, atomic.LoadUint64(&r.lastRemoteIndex), r.logger)
		if exit {
			// the replication function told us to exit
			return nil
		}

		if err != nil {
			// reset the lastRemoteIndex when there is an RPC failure. This should cause a full sync to be done during
			// the next round of replication
			atomic.StoreUint64(&r.lastRemoteIndex, 0)

			if r.suppressErrorLog != nil && !r.suppressErrorLog(err) {
				r.logger.Warn("replication error (will retry if still leader)", "error", err)
			}
		} else {
			atomic.StoreUint64(&r.lastRemoteIndex, index)
			r.logger.Debug("replication completed through remote index", "index", index)
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

type ReplicatorFunc func(ctx context.Context, lastRemoteIndex uint64, logger hclog.Logger) (index uint64, exit bool, err error)

type FunctionReplicator struct {
	ReplicateFn ReplicatorFunc
}

func (r *FunctionReplicator) Replicate(ctx context.Context, lastRemoteIndex uint64, logger hclog.Logger) (uint64, bool, error) {
	return r.ReplicateFn(ctx, lastRemoteIndex, logger)
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
	Logger   hclog.Logger
}

func (r *IndexReplicator) Replicate(ctx context.Context, lastRemoteIndex uint64, _ hclog.Logger) (uint64, bool, error) {
	fetchStart := time.Now()
	lenRemote, remote, remoteIndex, err := r.Delegate.FetchRemote(lastRemoteIndex)
	metrics.MeasureSince([]string{"leader", "replication", r.Delegate.MetricName(), "fetch"}, fetchStart)

	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve %s: %v", r.Delegate.PluralNoun(), err)
	}

	r.Logger.Debug("finished fetching remote objects",
		"amount", lenRemote,
	)

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
		r.Logger.Warn("replication remote index moved backwards, forcing a full sync",
			"from", lastRemoteIndex,
			"to", remoteIndex,
		)
		lastRemoteIndex = 0
	}

	r.Logger.Debug("diffing replication state",
		"local_amount", lenLocal,
		"remote_amount", lenRemote,
	)

	// Calculate the changes required to bring the state into sync and then
	// apply them.
	diff, err := r.Delegate.DiffRemoteAndLocalState(local, remote, lastRemoteIndex)
	if err != nil {
		return 0, false, fmt.Errorf("failed to diff %s local and remote states: %v", r.Delegate.SingularNoun(), err)
	}

	r.Logger.Debug("diffed replication state",
		"deletions", diff.NumDeletions,
		"updates", diff.NumUpdates,
	)

	if diff.NumDeletions > 0 {
		r.Logger.Debug("performing deletions",
			"deletions", diff.NumDeletions,
		)

		exit, err := r.Delegate.PerformDeletions(ctx, diff.Deletions)
		if exit {
			return 0, true, nil
		}

		if err != nil {
			return 0, false, fmt.Errorf("failed to apply local %s deletions: %v", r.Delegate.SingularNoun(), err)
		}
		r.Logger.Debug("finished deletions")
	}

	if diff.NumUpdates > 0 {
		r.Logger.Debug("performing updates",
			"updates", diff.NumUpdates,
		)

		exit, err := r.Delegate.PerformUpdates(ctx, diff.Updates)
		if exit {
			return 0, true, nil
		}

		if err != nil {
			return 0, false, fmt.Errorf("failed to apply local %s updates: %v", r.Delegate.SingularNoun(), err)
		}
		r.Logger.Debug("finished updates")
	}

	// Return the index we got back from the remote side, since we've synced
	// up with the remote state as of that index.
	return remoteIndex, false, nil
}
