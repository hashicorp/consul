package consul

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/lib"
	"golang.org/x/time/rate"
)

const (
	// replicationMaxRetryWait is the maximum number of seconds to wait between
	// failed blocking queries when backing off.
	replicationDefaultMaxRetryWait = 120 * time.Second

	replicationDefaultRate = 1
)

type ReplicatorConfig struct {
	// Name to be used in various logging
	Name string
	// Function to perform the actual replication
	ReplicateFn ReplicatorFunc
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

type ReplicatorFunc func(ctx context.Context, lastRemoteIndex uint64) (index uint64, exit bool, err error)

type Replicator struct {
	name            string
	limiter         *rate.Limiter
	waiter          *lib.RetryWaiter
	replicateFn     ReplicatorFunc
	logger          *log.Logger
	lastRemoteIndex uint64
}

func NewReplicator(config *ReplicatorConfig) (*Replicator, error) {
	if config == nil {
		return nil, fmt.Errorf("Cannot create the Replicator without a config")
	}
	if config.ReplicateFn == nil {
		return nil, fmt.Errorf("Cannot create the Replicator without a ReplicateFn set in the config")
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
		name:        config.Name,
		limiter:     limiter,
		waiter:      waiter,
		replicateFn: config.ReplicateFn,
		logger:      config.Logger,
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
		index, exit, err := r.replicateFn(ctx, atomic.LoadUint64(&r.lastRemoteIndex))
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
