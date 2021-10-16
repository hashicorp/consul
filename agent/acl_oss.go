// +build !consulent

package agent

import (
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/acl"
)

func serfMemberFillAuthzContext(m *serf.Member, ctx *acl.AuthorizerContext) {
	// no-op
}
