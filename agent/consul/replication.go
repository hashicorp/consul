package consul

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// replicationMaxRetryBackoff is the maximum number of seconds to wait between
	// failed blocking queries when backing off.
	replicationMaxRetryBackoff = 256
)

type ReplicatorConfig struct {
	Name            string
	ReplicateFn     ReplicatorFunc
	Rate            int
	Burst           int
	MaxRetryBackoff int
	Logger          *log.Logger
}

type ReplicatorFunc func(ctx context.Context, lastRemoteIndex uint64) (index uint64, exit bool, err error)

type Replicator struct {
	name            string
	lock            sync.RWMutex
	running         bool
	cancel          context.CancelFunc
	ctx             context.Context
	limiter         *rate.Limiter
	maxRetryBackoff int
	replicate       ReplicatorFunc
	logger          *log.Logger
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
	ctx, cancel := context.WithCancel(context.Background())
	limiter := rate.NewLimiter(rate.Limit(config.Rate), config.Burst)
	return &Replicator{
		name:            config.Name,
		running:         false,
		cancel:          cancel,
		ctx:             ctx,
		limiter:         limiter,
		maxRetryBackoff: config.MaxRetryBackoff,
		replicate:       config.ReplicateFn,
		logger:          config.Logger,
	}, nil
}

func (r *Replicator) Start() {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.running {
		return
	}

	go r.run()

	r.running = true
	r.logger.Printf("[INFO] replication: started %s replication", r.name)
}

func (r *Replicator) run() {
	var failedAttempts uint
	var lastRemoteIndex uint64

	defer r.logger.Printf("[INFO] replication: stopped %s replication", r.name)

	for {
		if err := r.limiter.Wait(r.ctx); err != nil {
			return
		}

		// Perform a single round of replication
		index, exit, err := r.replicate(r.ctx, lastRemoteIndex)
		if exit {
			return
		}

		if err != nil {
			lastRemoteIndex = 0
			r.logger.Printf("[WARN] replication: %s replication error (will retry if still leader): %v", r.name, err)
			if (1 << failedAttempts) < r.maxRetryBackoff {
				failedAttempts++
			}

			select {
			case <-r.ctx.Done():
				return
			case <-time.After((1 << failedAttempts) * time.Second):
				// do nothing
			}
		} else {
			lastRemoteIndex = index
			r.logger.Printf("[DEBUG] replication: %s replication completed through remote index %d", r.name, index)
			failedAttempts = 0
		}
	}
}

func (r *Replicator) Stop() {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.running {
		return
	}

	r.logger.Printf("[DEBUG] replication: stopping %s replication", r.name)
	r.cancel()
	r.cancel = nil
	r.running = false
}
