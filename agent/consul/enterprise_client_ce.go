//go:build !consulent
// +build !consulent

package consul

import (
	"github.com/hashicorp/serf/serf"
)

type EnterpriseClient struct{}

func (c *Client) initEnterprise(_ Deps) error {
	return nil
}

func enterpriseModifyClientSerfConfigLAN(_ *Config, _ *serf.Config) {
	// nothing
}

func (c *Client) startEnterprise() error {
	return nil
}

func (c *Client) handleEnterpriseUserEvents(event serf.UserEvent) bool {
	return false
}

func (c *Client) enterpriseStats() map[string]map[string]string {
	return nil
}
