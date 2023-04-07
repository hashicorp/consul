package agent

import (
	"fmt"
	"strings"
	"time"

	discoverhcp "github.com/hashicorp/consul/agent/hcp/discover"
	discover "github.com/hashicorp/go-discover"
	discoverk8s "github.com/hashicorp/go-discover/provider/k8s"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/lib"
)

func (a *Agent) retryJoinLAN() {
	r := &retryJoiner{
		variant:     retryJoinSerfVariant,
		cluster:     "LAN",
		addrs:       a.config.RetryJoinLAN,
		maxAttempts: a.config.RetryJoinMaxAttemptsLAN,
		interval:    a.config.RetryJoinIntervalLAN,
		join: func(addrs []string) (int, error) {
			// NOTE: For partitioned servers you are only capable of using retry join
			// to join nodes in the default partition.
			return a.JoinLAN(addrs, a.AgentEnterpriseMeta())
		},
		logger: a.logger.With("cluster", "LAN"),
	}
	if err := r.retryJoin(); err != nil {
		a.retryJoinCh <- err
	}
}

func (a *Agent) retryJoinWAN() {
	if !a.config.ServerMode {
		a.logger.Warn("(WAN) couldn't join: Err: Must be a server to join WAN cluster")
		return
	}

	isPrimary := a.config.PrimaryDatacenter == a.config.Datacenter

	var joinAddrs []string
	if a.config.ConnectMeshGatewayWANFederationEnabled {
		// When wanfed is activated each datacenter 100% relies upon flood-join
		// to replicate the LAN members in a dc into the WAN pool. We
		// completely hijack whatever the user configured to correctly
		// implement the star-join.
		//
		// Elsewhere we enforce that retry-join-wan cannot be set if wanfed is
		// enabled so we don't have to emit any warnings related to that here.
		if isPrimary {
			// Wanfed requires that secondaries join TO the primary and the
			// primary doesn't explicitly join down to the secondaries, so as
			// such in the primary a retry-join operation is a no-op.
			return
		}

		// First get a handle on dialing the primary
		a.refreshPrimaryGatewayFallbackAddresses()

		// Then "retry join" a special address via the gateway which is
		// load balanced to all servers in the primary datacenter
		//
		// Since this address is merely a placeholder we use an address from the
		// TEST-NET-1 block as described in https://tools.ietf.org/html/rfc5735#section-3
		const placeholderIPAddress = "192.0.2.2"
		joinAddrs = []string{
			fmt.Sprintf("*.%s/%s", a.config.PrimaryDatacenter, placeholderIPAddress),
		}
	} else {
		joinAddrs = a.config.RetryJoinWAN
	}

	r := &retryJoiner{
		variant:     retryJoinSerfVariant,
		cluster:     "WAN",
		addrs:       joinAddrs,
		maxAttempts: a.config.RetryJoinMaxAttemptsWAN,
		interval:    a.config.RetryJoinIntervalWAN,
		join:        a.JoinWAN,
		logger:      a.logger.With("cluster", "WAN"),
	}
	if err := r.retryJoin(); err != nil {
		a.retryJoinCh <- err
	}
}

func (a *Agent) refreshPrimaryGatewayFallbackAddresses() {
	r := &retryJoiner{
		variant:     retryJoinMeshGatewayVariant,
		cluster:     "primary",
		addrs:       a.config.PrimaryGateways,
		maxAttempts: 0,
		interval:    a.config.PrimaryGatewaysInterval,
		join: func(addrs []string) (int, error) {
			if err := a.RefreshPrimaryGatewayFallbackAddresses(addrs); err != nil {
				return 0, err
			}
			return len(addrs), nil
		},
		logger: a.logger,
		stopCh: a.PrimaryMeshGatewayAddressesReadyCh(),
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
	providers["hcp"] = &discoverhcp.Provider{}

	return discover.New(
		discover.WithUserAgent(lib.UserAgent()),
		discover.WithProviders(providers),
	)
}

func retryJoinAddrs(disco *discover.Discover, variant, cluster string, retryJoin []string, logger hclog.Logger) []string {
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
					if variant == retryJoinMeshGatewayVariant {
						logger.Info("Discovered mesh gateways",
							"cluster", cluster,
							"mesh_gateways", strings.Join(servers, " "),
						)
					} else {
						logger.Info("Discovered servers",
							"cluster", cluster,
							"servers", strings.Join(servers, " "),
						)
					}
				}
			}

		default:
			addrs = append(addrs, addr)
		}
	}

	return addrs
}

const (
	retryJoinSerfVariant        = "serf"
	retryJoinMeshGatewayVariant = "mesh-gateway"
)

// retryJoiner is used to handle retrying a join until it succeeds or all
// retries are exhausted.
type retryJoiner struct {
	// variant is either "serf" or "mesh-gateway" and just adjusts the log messaging
	// emitted
	variant string

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

	// stopCh is an optional stop channel to exit the retry loop early
	stopCh <-chan struct{}

	// logger is the agent logger.
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

	if r.variant == retryJoinMeshGatewayVariant {
		r.logger.Info("Refreshing mesh gateways is supported for the following discovery methods",
			"discovery_methods", strings.Join(disco.Names(), " "),
		)
		r.logger.Info("Refreshing mesh gateways...")
	} else {
		r.logger.Info("Retry join is supported for the following discovery methods",
			"discovery_methods", strings.Join(disco.Names(), " "),
		)
		r.logger.Info("Joining cluster...")
	}

	attempt := 0
	for {
		addrs := retryJoinAddrs(disco, r.variant, r.cluster, r.addrs, r.logger)
		if len(addrs) > 0 {
			n := 0
			n, err = r.join(addrs)
			if err == nil {
				if r.variant == retryJoinMeshGatewayVariant {
					r.logger.Info("Refreshing mesh gateways completed")
				} else {
					r.logger.Info("Join cluster completed. Synced with initial agents", "num_agents", n)
				}
				return nil
			}
		} else if len(addrs) == 0 {
			if r.variant == retryJoinMeshGatewayVariant {
				err = fmt.Errorf("No mesh gateways found")
			} else {
				err = fmt.Errorf("No servers to join")
			}
		}

		attempt++
		if r.maxAttempts > 0 && attempt > r.maxAttempts {
			if r.variant == retryJoinMeshGatewayVariant {
				return fmt.Errorf("agent: max refresh of %s mesh gateways retry exhausted, exiting", r.cluster)
			} else {
				return fmt.Errorf("agent: max join %s retry exhausted, exiting", r.cluster)
			}
		}

		if r.variant == retryJoinMeshGatewayVariant {
			r.logger.Warn("Refreshing mesh gateways failed, will retry",
				"retry_interval", r.interval,
				"error", err,
			)
		} else {
			r.logger.Warn("Join cluster failed, will retry",
				"retry_interval", r.interval,
				"error", err,
			)
		}

		select {
		case <-time.After(r.interval):
		case <-r.stopCh:
			return nil
		}
	}
}
