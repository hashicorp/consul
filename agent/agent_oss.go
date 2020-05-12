// +build !consulent

package agent

import (
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// enterpriseAgent embeds fields that we only access in consul-enterprise builds
type enterpriseAgent struct{}

// fillAgentServiceEnterpriseMeta is a noop stub for the func defined agent_ent.go
func fillAgentServiceEnterpriseMeta(_ *api.AgentService, _ *structs.EnterpriseMeta) {}

// fillHealthCheckEnterpriseMeta is a noop stub for the func defined agent_ent.go
func fillHealthCheckEnterpriseMeta(_ *api.HealthCheck, _ *structs.EnterpriseMeta) {}

// initEnterprise is a noop stub for the func defined agent_ent.go
func (a *Agent) initEnterprise(consulCfg *consul.Config) error {
	return nil
}

// loadEnterpriseTokens is a noop stub for the func defined agent_ent.go
func (a *Agent) loadEnterpriseTokens(conf *config.RuntimeConfig) {
}

// reloadEnterprise is a noop stub for the func defined agent_ent.go
func (a *Agent) reloadEnterprise(conf *config.RuntimeConfig) error {
	return nil
}

// enterpriseConsulConfig is a noop stub for the func defined in agent_ent.go
func (a *Agent) enterpriseConsulConfig(base *consul.Config) (*consul.Config, error) {
	return base, nil
}

// WriteEvent is a noop stub for the func defined agent_ent.go
func (a *Agent) WriteEvent(eventType string, payload interface{}) {
}
