package agent

import (
	"fmt"
	"strings"
	"time"

	discover "github.com/hashicorp/go-discover"
)

// RetryJoin is used to handle retrying a join until it succeeds or all
// retries are exhausted.
func (a *Agent) retryJoin() {
	cfg := a.config
	if len(cfg.RetryJoin) == 0 {
		return
	}

	disco := discover.Discover{}
	a.logger.Printf("[INFO] agent: Retry join is supported for: %s", strings.Join(disco.Names(), " "))
	a.logger.Printf("[INFO] agent: Joining cluster...")
	attempt := 0
	for {
		var addrs []string
		var err error

		for _, addr := range cfg.RetryJoin {
			switch {
			case strings.Contains(addr, "provider="):
				servers, err := disco.Addrs(addr, a.logger)
				if err != nil {
					a.logger.Printf("[ERR] agent: %s", err)
				} else {
					addrs = append(addrs, servers...)
					a.logger.Printf("[INFO] agent: Discovered servers: %s", strings.Join(servers, " "))
				}

			default:
				addrs = append(addrs, addr)
			}
		}

		if len(addrs) > 0 {
			n, err := a.JoinLAN(addrs)
			if err == nil {
				a.logger.Printf("[INFO] agent: Join completed. Synced with %d initial agents", n)
				return
			}
		}

		if len(addrs) == 0 {
			err = fmt.Errorf("No servers to join")
		}

		attempt++
		if cfg.RetryMaxAttempts > 0 && attempt > cfg.RetryMaxAttempts {
			a.retryJoinCh <- fmt.Errorf("agent: max join retry exhausted, exiting")
			return
		}

		a.logger.Printf("[WARN] agent: Join failed: %v, retrying in %v", err, cfg.RetryInterval)
		time.Sleep(cfg.RetryInterval)
	}
}

// RetryJoinWan is used to handle retrying a join -wan until it succeeds or all
// retries are exhausted.
func (a *Agent) retryJoinWan() {
	cfg := a.config

	if len(cfg.RetryJoinWan) == 0 {
		return
	}

	a.logger.Printf("[INFO] agent: Joining WAN cluster...")

	attempt := 0
	for {
		n, err := a.JoinWAN(cfg.RetryJoinWan)
		if err == nil {
			a.logger.Printf("[INFO] agent: Join -wan completed. Synced with %d initial agents", n)
			return
		}

		attempt++
		if cfg.RetryMaxAttemptsWan > 0 && attempt > cfg.RetryMaxAttemptsWan {
			a.retryJoinCh <- fmt.Errorf("agent: max join -wan retry exhausted, exiting")
			return
		}

		a.logger.Printf("[WARN] agent: Join -wan failed: %v, retrying in %v", err, cfg.RetryIntervalWan)
		time.Sleep(cfg.RetryIntervalWan)
	}
}
