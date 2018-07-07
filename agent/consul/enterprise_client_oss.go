// +build !ent

package consul

import (
	"github.com/hashicorp/serf/serf"
)

type EnterpriseClient struct{}

func (c *Client) initEnterprise() error {
	return nil
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
