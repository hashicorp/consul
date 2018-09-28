package register

import (
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestConfigToAgentService(t *testing.T) {
	cases := []struct {
		Name   string
		Input  *config.ServiceDefinition
		Output *api.AgentServiceRegistration
	}{
		{
			"Basic service with port",
			&config.ServiceDefinition{
				Name: strPtr("web"),
				Tags: []string{"leader"},
				Port: intPtr(1234),
			},
			&api.AgentServiceRegistration{
				Name: "web",
				Tags: []string{"leader"},
				Port: 1234,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)
			actual, err := configToAgentService(tc.Input)
			require.NoError(err)
			require.Equal(tc.Output, actual)
		})
	}
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
