package agent

import (
	"fmt"
	"time"
)

// RetryJoin is used to handle retrying a join until it succeeds or all
// retries are exhausted.
func (a *Agent) retryJoin() {
	cfg := a.config

	ec2Enabled := cfg.RetryJoinEC2.TagKey != "" && cfg.RetryJoinEC2.TagValue != ""
	gceEnabled := cfg.RetryJoinGCE.TagValue != ""
	azureEnabled := cfg.RetryJoinAzure.TagName != "" && cfg.RetryJoinAzure.TagValue != ""

	if len(cfg.RetryJoin) == 0 && !ec2Enabled && !gceEnabled && !azureEnabled {
		return
	}

	a.logger.Printf("[INFO] agent: Joining cluster...")
	attempt := 0
	for {
		var servers []string
		var err error
		switch {
		case ec2Enabled:
			servers, err = cfg.discoverEc2Hosts(a.logger)
			if err != nil {
				a.logger.Printf("[ERR] agent: Unable to query EC2 instances: %s", err)
			}
			a.logger.Printf("[INFO] agent: Discovered %d servers from EC2", len(servers))
		case gceEnabled:
			servers, err = cfg.discoverGCEHosts(a.logger)
			if err != nil {
				a.logger.Printf("[ERR] agent: Unable to query GCE instances: %s", err)
			}
			a.logger.Printf("[INFO] agent: Discovered %d servers from GCE", len(servers))
		case azureEnabled:
			servers, err = cfg.discoverAzureHosts(a.logger)
			if err != nil {
				a.logger.Printf("[ERR] agent: Unable to query Azure instances: %s", err)
			}
			a.logger.Printf("[INFO] agent: Discovered %d servers from Azure", len(servers))
		}

		servers = append(servers, cfg.RetryJoin...)
		if len(servers) == 0 {
			err = fmt.Errorf("No servers to join")
		} else {
			n, err := a.JoinLAN(servers)
			if err == nil {
				a.logger.Printf("[INFO] agent: Join completed. Synced with %d initial agents", n)
				return
			}
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
