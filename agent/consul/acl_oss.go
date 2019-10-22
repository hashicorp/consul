// +build !consulent

package consul

import (
	"log"

	"github.com/hashicorp/consul/acl"
)

func newEnterpriseACLConfig(*log.Logger) *acl.EnterpriseACLConfig {
	return nil
}
