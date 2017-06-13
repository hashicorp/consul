package agent

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/discover/aws"
	"github.com/hashicorp/consul/discover/azure"
	"github.com/hashicorp/consul/discover/gce"
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
			c := cfg.RetryJoinEC2
			awscfg := &aws.Config{
				Region:          c.Region,
				TagKey:          c.TagKey,
				TagValue:        c.TagValue,
				AccessKeyID:     c.AccessKeyID,
				SecretAccessKey: c.SecretAccessKey,
			}
			servers, err = aws.Discover(awscfg, a.logger)
			if err != nil {
				a.logger.Printf("[ERR] agent: Unable to query EC2 instances: %s", err)
			}
			a.logger.Printf("[INFO] agent: Discovered %d servers from EC2", len(servers))

		case gceEnabled:
			c := cfg.RetryJoinGCE
			gcecfg := &gce.Config{
				ProjectName:     c.ProjectName,
				ZonePattern:     c.ZonePattern,
				TagValue:        c.TagValue,
				CredentialsFile: c.CredentialsFile,
			}
			servers, err = gce.Discover(gcecfg, a.logger)
			if err != nil {
				a.logger.Printf("[ERR] agent: Unable to query GCE instances: %s", err)
			}
			a.logger.Printf("[INFO] agent: Discovered %d servers from GCE", len(servers))
		case azureEnabled:
			c := cfg.RetryJoinAzure
			azurecfg := &azure.Config{
				TagName:         c.TagName,
				TagValue:        c.TagValue,
				SubscriptionID:  c.SubscriptionID,
				TenantID:        c.TenantID,
				ClientID:        c.ClientID,
				SecretAccessKey: c.SecretAccessKey,
			}
			servers, err = azure.Discover(azurecfg, a.logger)
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
