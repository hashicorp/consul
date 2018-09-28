package register

import (
	"fmt"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/mapstructure"
)

// configToAgentService converts a ServiceDefinition struct to an
// AgentServiceRegistration API struct.
func configToAgentService(svc *config.ServiceDefinition) (*api.AgentServiceRegistration, error) {
	var result api.AgentServiceRegistration
	var m map[string]interface{}
	err := mapstructure.Decode(svc, &m)
	if err == nil {
		println(fmt.Sprintf("%#v", m))
		err = mapstructure.Decode(m, &result)
	}
	return &result, err
}
