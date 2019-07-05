package agent

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/consul/lib"
	discover "github.com/hashicorp/go-discover"
	discoverk8s "github.com/hashicorp/go-discover/provider/k8s"
)

func (a *Agent) retryJoinLAN() {
	r := &retryJoiner{
		cluster:     "LAN",
		addrs:       a.config.RetryJoinLAN,
		maxAttempts: a.config.RetryJoinMaxAttemptsLAN,
		interval:    a.config.RetryJoinIntervalLAN,
		join:        a.JoinLAN,
		logger:      a.logger,
	}
	if err := r.retryJoin(); err != nil {
		a.retryJoinCh <- err
	}
}

func (a *Agent) retryJoinWAN() {
	r := &retryJoiner{
		cluster:     "WAN",
		addrs:       a.config.RetryJoinWAN,
		maxAttempts: a.config.RetryJoinMaxAttemptsWAN,
		interval:    a.config.RetryJoinIntervalWAN,
		join:        a.JoinWAN,
		logger:      a.logger,
	}
	if err := r.retryJoin(); err != nil {
		a.retryJoinCh <- err
	}
}

func newDiscover() (*discover.Discover, error) {
	providers := make(map[string]discover.Provider)
	for k, v := range discover.Providers {
		providers[k] = v
	}
	providers["k8s"] = &discoverk8s.Provider{}

	return discover.New(
		discover.WithUserAgent(lib.UserAgent()),
		discover.WithProviders(providers),
	)
}

func retryJoinAddrs(disco *discover.Discover, cluster string, retryJoin []string, logger *log.Logger) []string {
	addrs := []string{}
	if disco == nil {
		return addrs
	}
	for _, addr := range retryJoin {
		switch {
		case strings.Contains(addr, "provider="):
			servers, err := disco.Addrs(addr, logger)
			if err != nil {
				if logger != nil {
					logger.Printf("[ERR] agent: Cannot discover %s %s: %s", cluster, addr, err)
				}
			} else {
				addrs = append(addrs, servers...)
				if logger != nil {
					logger.Printf("[INFO] agent: Discovered %s servers: %s", cluster, strings.Join(servers, " "))
				}
			}

		default:
			addrs = append(addrs, addr)
		}
	}

	return addrs
}

// retryJoiner is used to handle retrying a join until it succeeds or all
// retries are exhausted.
type retryJoiner struct {
	// cluster is the name of the serf cluster, e.g. "LAN" or "WAN".
	cluster string

	// addrs is the list of servers or go-discover configurations
	// to join with.
	addrs []string

	// maxAttempts is the number of join attempts before giving up.
	maxAttempts int

	// interval is the time between two join attempts.
	interval time.Duration

	// join adds the discovered or configured servers to the given
	// serf cluster.
	join func([]string) (int, error)

	// logger is the agent logger. Log messages should contain the
	// "agent: " prefix.
	logger *log.Logger
}

func (r *retryJoiner) retryJoin() error {
	if len(r.addrs) == 0 {
		return nil
	}

	disco, err := newDiscover()
	if err != nil {
		return err
	}

	r.logger.Printf("[INFO] agent: Retry join %s is supported for: %s", r.cluster, strings.Join(disco.Names(), " "))
	r.logger.Printf("[INFO] agent: Joining %s cluster...", r.cluster)
	attempt := 0
	for {
		addrs := retryJoinAddrs(disco, r.cluster, r.addrs, r.logger)
		if len(addrs) > 0 {
			n, err := r.join(addrs)
			if err == nil {
				r.logger.Printf("[INFO] agent: Join %s completed. Synced with %d initial agents", r.cluster, n)
				return nil
			}
		} else if len(addrs) == 0 {
			err = fmt.Errorf("No servers to join")
		}

		attempt++
		if r.maxAttempts > 0 && attempt > r.maxAttempts {
			return fmt.Errorf("agent: max join %s retry exhausted, exiting", r.cluster)
		}

		r.logger.Printf("[WARN] agent: Join %s failed: %v, retrying in %v", r.cluster, err, r.interval)
		time.Sleep(r.interval)
	}
}
