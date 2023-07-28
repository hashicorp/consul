//go:build !consulent
// +build !consulent

package agent

import (
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
)

func serfMemberFillAuthzContext(m *serf.Member, ctx *acl.AuthorizerContext) {
	// no-op
}

func agentServiceFillAuthzContext(s *api.AgentService, ctx *acl.AuthorizerContext) {
	// no-op
}
