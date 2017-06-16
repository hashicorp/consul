package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/discover"
)

// RetryJoin is used to handle retrying a join until it succeeds or all
// retries are exhausted.
func (a *Agent) retryJoin() {
	cfg := a.config
	awscfg := cfg.RetryJoinEC2
	azurecfg := cfg.RetryJoinAzure
	gcecfg := cfg.RetryJoinGCE

	a.logger.Printf("[INFO] agent: Joining cluster...")
	attempts := cfg.RetryMaxAttempts
	for {
		var args []string
		switch {
		case awscfg.TagKey != "" && awscfg.TagValue != "":
			args = []string{
				"provider=aws",
				"region=" + awscfg.Region,
				"tag_key=" + awscfg.TagKey,
				"tag_value=" + awscfg.TagValue,
				"access_key_id=" + awscfg.AccessKeyID,
				"secret_access_key=" + awscfg.SecretAccessKey,
			}

		case gcecfg.TagValue != "":
			args = []string{
				"provider=gce",
				"project_name=" + gcecfg.ProjectName,
				"zone_pattern=" + gcecfg.ZonePattern,
				"tag_value=" + gcecfg.TagValue,
				"credentials_file=" + gcecfg.CredentialsFile,
			}

		case azurecfg.TagName != "" && azurecfg.TagValue != "":
			args = []string{
				"provider=azure",
				"tag_name=" + azurecfg.TagName,
				"tag_value=" + azurecfg.TagValue,
				"tenant_id=" + azurecfg.TenantID,
				"client_id=" + azurecfg.ClientID,
				"subscription_id=" + azurecfg.SubscriptionID,
				"secret_access_key=" + azurecfg.SecretAccessKey,
			}
		}

		// do not retry join
		if len(cfg.RetryJoin) == 0 && len(args) == 0 {
			return
		}

		var n int
		var err error
		var servers []string

		discovered, err := discover.Discover(strings.Join(args, " "), a.logger)
		if err != nil {
			goto Retry
		}
		servers = discovered

		servers = append(servers, cfg.RetryJoin...)
		if len(servers) == 0 {
			err = fmt.Errorf("No servers to join")
			goto Retry
		}

		n, err = a.JoinLAN(servers)
		if err == nil {
			a.logger.Printf("[INFO] agent: Join completed. Synced with %d initial agents", n)
			return
		}

	Retry:
		attempts--
		if attempts <= 0 {
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
