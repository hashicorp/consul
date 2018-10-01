package register

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestStructsToAgentService(t *testing.T) {
	cases := []struct {
		Name   string
		Input  *structs.ServiceDefinition
		Output *api.AgentServiceRegistration
	}{
		{
			"Basic service with port",
			&structs.ServiceDefinition{
				Name: "web",
				Tags: []string{"leader"},
				Port: 1234,
			},
			&api.AgentServiceRegistration{
				Name: "web",
				Tags: []string{"leader"},
				Port: 1234,
			},
		},
		{
			"Service with a check",
			&structs.ServiceDefinition{
				Name: "web",
				Check: structs.CheckType{
					Name: "ping",
				},
			},
			&api.AgentServiceRegistration{
				Name: "web",
				Check: &api.AgentServiceCheck{
					Name: "ping",
				},
			},
		},
		{
			"Service with checks",
			&structs.ServiceDefinition{
				Name: "web",
				Checks: structs.CheckTypes{
					&structs.CheckType{
						Name: "ping",
					},
					&structs.CheckType{
						Name: "pong",
					},
				},
			},
			&api.AgentServiceRegistration{
				Name: "web",
				Checks: api.AgentServiceChecks{
					&api.AgentServiceCheck{
						Name: "ping",
					},
					&api.AgentServiceCheck{
						Name: "pong",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)
			actual, err := serviceToAgentService(tc.Input)
			require.NoError(err)
			require.Equal(tc.Output, actual)
		})
	}
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
