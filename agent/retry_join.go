package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/lib"
	discover "github.com/hashicorp/go-discover"
	discoverk8s "github.com/hashicorp/go-discover/provider/k8s"
	"github.com/hashicorp/go-hclog"
)

func (a *Agent) retryJoinLAN() {
	r := &retryJoiner{
		cluster:     "LAN",
		addrs:       a.config.RetryJoinLAN,
		maxAttempts: a.config.RetryJoinMaxAttemptsLAN,
		interval:    a.config.RetryJoinIntervalLAN,
		join:        a.JoinLAN,
		logger:      a.logger.With("cluster", "LAN"),
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
		logger:      a.logger.With("cluster", "WAN"),
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

func retryJoinAddrs(disco *discover.Discover, cluster string, retryJoin []string, logger hclog.Logger) []string {
	addrs := []string{}
	if disco == nil {
		return addrs
	}
	for _, addr := range retryJoin {
		switch {
		case strings.Contains(addr, "provider="):
			servers, err := disco.Addrs(addr, logger.StandardLogger(&hclog.StandardLoggerOptions{
				InferLevels: true,
			}))
			if err != nil {
				if logger != nil {
					logger.Error("Cannot discover address",
						"address", addr,
						"error", err,
					)
				}
			} else {
				addrs = append(addrs, servers...)
				if logger != nil {
					logger.Info("Discovered servers",
						"cluster", cluster,
						"servers", strings.Join(servers, " "),
					)
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
	logger hclog.Logger
}

func (r *retryJoiner) retryJoin() error {
	if len(r.addrs) == 0 {
		return nil
	}

	disco, err := newDiscover()
	if err != nil {
		return err
	}

	r.logger.Info("Retry join is supported for the following discovery methods",
		"discovery_methods", strings.Join(disco.Names(), " "),
	)
	r.logger.Info("Joining cluster...")
	attempt := 0
	for {
		addrs := retryJoinAddrs(disco, r.cluster, r.addrs, r.logger)
		if len(addrs) > 0 {
			n, err := r.join(addrs)
			if err == nil {
				r.logger.Info("Join cluster completed. Synced with initial agents", "num_agents", n)
				return nil
			}
		} else if len(addrs) == 0 {
			err = fmt.Errorf("No servers to join")
		}

		attempt++
		if r.maxAttempts > 0 && attempt > r.maxAttempts {
			return fmt.Errorf("agent: max join %s retry exhausted, exiting", r.cluster)
		}

		r.logger.Warn("Join cluster failed, will retry",
			"retry_interval", r.interval,
			"error", err,
		)
		time.Sleep(r.interval)
	}
}
