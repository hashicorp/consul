//go:build !consulent
// +build !consulent

package agent

import (
	autoconf "github.com/hashicorp/consul/agent/auto-config"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
)

// initEnterpriseBaseDeps is responsible for initializing the enterprise dependencies that
// will be utilized throughout the whole Consul Agent.
func initEnterpriseBaseDeps(d BaseDeps, _ *config.RuntimeConfig) (BaseDeps, error) {
	return d, nil
}

// initEnterpriseAutoConfig is responsible for setting up auto-config for enterprise
func initEnterpriseAutoConfig(_ consul.EnterpriseDeps, _ *config.RuntimeConfig) autoconf.EnterpriseConfig {
	return autoconf.EnterpriseConfig{}
}
