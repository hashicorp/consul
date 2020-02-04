// +build !consulent

package agent

import (
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// fillAgentServiceEnterpriseMeta stub
func fillAgentServiceEnterpriseMeta(_ *api.AgentService, _ *structs.EnterpriseMeta) {}

// fillHealthCheckEnterpriseMeta stub
func fillHealthCheckEnterpriseMeta(_ *api.HealthCheck, _ *structs.EnterpriseMeta) {}

func (a *Agent) initEnterprise(consulCfg *consul.Config) {
}

func (a *Agent) loadEnterpriseTokens(conf *config.RuntimeConfig) {
}
