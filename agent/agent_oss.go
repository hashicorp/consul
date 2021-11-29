//go:build !consulent
// +build !consulent

package agent

import (
	"context"

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

// reloadEnterprise is a noop stub for the func defined agent_ent.go
func (a *Agent) reloadEnterprise(conf *config.RuntimeConfig) error {
	return nil
}

// enterpriseConsulConfig is a noop stub for the func defined in agent_ent.go
func enterpriseConsulConfig(_ *consul.Config, _ *config.RuntimeConfig) {
}

// WriteEvent is a noop stub for the func defined agent_ent.go
func (a *Agent) WriteEvent(eventType string, payload interface{}) {
}

// startLicenseManager is used to start the license management process
func (a *Agent) startLicenseManager(_ context.Context) error {
	return nil
}

// stopLicenseManager is used to stop the license management go routines
func (a *Agent) stopLicenseManager() {}

// enterpriseStats outputs all the Agent stats specific to Consul Enterprise
func (a *Agent) enterpriseStats() map[string]map[string]string {
	return nil
}

func (a *Agent) AgentEnterpriseMeta() *structs.EnterpriseMeta {
	return structs.NodeEnterpriseMetaInDefaultPartition()
}

func (a *Agent) registerEntCache() {}
