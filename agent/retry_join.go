package agent

import (
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/go-discover"
)

// RetryJoin is used to handle retrying a join until it succeeds or all
// retries are exhausted.
func (a *Agent) retryJoin() {
	cfg := a.config
	awscfg := cfg.RetryJoinEC2
	azurecfg := cfg.RetryJoinAzure
	gcecfg := cfg.RetryJoinGCE

	q := url.QueryEscape

	a.logger.Printf("[INFO] agent: Joining cluster...")
	attempts := cfg.RetryMaxAttempts
	for {
		args := ""
		switch {
		case awscfg.TagKey != "" && awscfg.TagValue != "":
			args = fmt.Sprintf("provider=aws region=%s tag_key=%s tag_value=%s access_key_id=%s secret_access_key=%s",
				q(awscfg.Region), q(awscfg.TagKey), q(awscfg.TagValue), q(awscfg.AccessKeyID), q(awscfg.SecretAccessKey))

		case gcecfg.TagValue != "":
			args = fmt.Sprintf("provider=gce project_name=%s zone_pattern=%s tag_value=%s credentials_file=%s",
				q(gcecfg.ProjectName), q(gcecfg.ZonePattern), q(gcecfg.TagValue), q(gcecfg.CredentialsFile))

		case azurecfg.TagName != "" && azurecfg.TagValue != "":
			args = fmt.Sprintf("provider=azure tenant_id=%s subscription_id=%s client_id=%s tag_name=%s tag_value=%s secret_access_key=%s",
				q(azurecfg.TenantID), q(azurecfg.SubscriptionID), q(azurecfg.ClientID), q(azurecfg.TagName), q(azurecfg.TagValue), q(azurecfg.SecretAccessKey))
		}

		// do not retry join
		if len(cfg.RetryJoin) == 0 && args == "" {
			return
		}

		var n int
		var err error
		var servers []string

		discovered, err := discover.Discover(args, a.logger)
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
