//go:build !consulent
// +build !consulent

package login

type enterpriseCmd struct {
}

func (c *cmd) initEnterpriseFlags() {
}

func (c *cmd) login() int {
	return c.bearerTokenLogin()
}
