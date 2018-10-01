package register

import (
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/mapstructure"
)

// configToAgentService converts a ServiceDefinition struct to an
// AgentServiceRegistration API struct.
func configToAgentService(svc *config.ServiceDefinition) (*api.AgentServiceRegistration, error) {
	// mapstructure can do this for us, but we encapsulate it in this
	// helper function in case we need to change the logic in the future.
	var result api.AgentServiceRegistration
	return &result, mapstructure.Decode(svc, &result)
}
