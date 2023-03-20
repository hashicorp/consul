package bootstrap

import (
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/stretchr/testify/require"
)

func TestCheckManagementToken(t *testing.T) {
	for _, test := range []struct {
		name            string
		expectedToken   string
		runtimeConfig   *config.RuntimeConfig
		bootstrapConfig map[string]interface{}
	}{
		{
			name:          "EmptyRuntimeConfig",
			expectedToken: "foo",
			runtimeConfig: &config.RuntimeConfig{},
			bootstrapConfig: map[string]interface{}{
				"acl": map[string]interface{}{
					"tokens": map[string]interface{}{
						"initial_management": "foo",
					},
				},
			},
		},
		{
			name:          "DeleteBootstrapToken",
			expectedToken: "",
			runtimeConfig: &config.RuntimeConfig{
				ACLInitialManagementToken: "bar",
			},
			bootstrapConfig: map[string]interface{}{
				"acl": map[string]interface{}{
					"tokens": map[string]interface{}{
						"initial_management": "foo",
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)
			checkManagementToken(test.runtimeConfig, test.bootstrapConfig)

			acl, ok := test.bootstrapConfig["acl"].(map[string]interface{})
			if !ok {
				t.Error("`acl` was not found in bootstrap config")
			}

			tokens, ok := acl["tokens"].(map[string]interface{})
			if !ok {
				t.Error("`acl.tokens` was not found in bootstrap config")
			}

			bootstrapToken, _ := tokens["initial_management"].(string)
			r.Equal(test.expectedToken, bootstrapToken)
		})
	}
}
