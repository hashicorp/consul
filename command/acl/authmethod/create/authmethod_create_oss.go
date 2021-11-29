//go:build !consulent
// +build !consulent

package authmethodcreate

import "github.com/hashicorp/consul/api"

type enterpriseCmd struct {
}

func (c *cmd) initEnterpriseFlags() {}

func (c *cmd) enterprisePopulateAuthMethod(method *api.ACLAuthMethod) error {
	return nil
}
