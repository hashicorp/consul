package bootstrap

import (
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/sdk/testutil"
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
			checkManagementToken(test.bootstrapConfig)

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

func TestBootstrapConfigLoader(t *testing.T) {
	r := require.New(t)
	bootstrapConfig := `{"acl":{"tokens":{"initial_management":"foo"}}}`

	baseLoader := func(source config.Source) (config.LoadResult, error) {
		return config.Load(config.LoadOpts{
			DefaultConfig: source,
			HCL: []string{
				`server = true`,
				`node_name = "test"`,
				`data_dir = "/tmp/consul-data"`,
				`acl { tokens { initial_management = "bar" } }`,
			},
		})
	}

	bootstrapLoader := func(source config.Source) (config.LoadResult, error) {
		return bootstrapConfigLoader(baseLoader, &RawBootstrapConfig{
			ConfigJSON:      bootstrapConfig,
			ManagementToken: "foo",
		})(source)
	}

	logger := testutil.NewLogBuffer(t)
	bd, err := agent.NewBaseDeps(bootstrapLoader, logger, nil)
	r.NoError(err)

	r.Equal("bar", bd.RuntimeConfig.ACLInitialManagementToken)
	r.Equal("foo", bd.RuntimeConfig.Cloud.ManagementToken)
}
