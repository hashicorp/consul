package utils

import (
	"github.com/hashicorp/consul/api"
)

func ApplyDefaultProxySettings(c *api.Client) (bool, error) {
	req := &api.ProxyConfigEntry{
		Name: "global",
		Kind: "proxy-defaults",
		Config: map[string]any{
			"protocol": "tcp",
		},
	}
	ok, _, err := c.ConfigEntries().Set(req, &api.WriteOptions{})
	return ok, err
}
