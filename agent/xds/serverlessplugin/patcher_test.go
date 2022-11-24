package serverlessplugin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

func TestGetPatcherBySNI(t *testing.T) {
	cases := []struct {
		name     string
		sni      string
		kind     api.ServiceKind
		expected patcher
		config   *xdscommon.PluginConfiguration
	}{
		{
			name: "no sni match",
			sni:  "not-matching",
		},
		{
			name:   "no patcher",
			config: &xdscommon.PluginConfiguration{},
			sni:    "lambda-sni",
		},
		{
			name: "no kind match",
			kind: api.ServiceKindIngressGateway,
			sni:  "lambda-sni",
		},
		{
			name: "full match",
			sni:  "lambda-sni",
			kind: api.ServiceKindTerminatingGateway,
			expected: lambdaPatcher{
				ARN:                "arn",
				Region:             "region",
				PayloadPassthrough: false,
				Kind:               api.ServiceKindTerminatingGateway,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := sampleConfig()
			config.Kind = tc.kind
			if tc.config != nil {
				config = *tc.config
			}
			patcher := getPatcherBySNI(config, tc.sni)

			if tc.expected == nil {
				require.Empty(t, patcher)
			} else {
				require.Equal(t, tc.expected, patcher)
			}
		})
	}
}

var (
	lambdaService         = api.CompoundServiceName{Name: "lambda"}
	disabledLambdaService = api.CompoundServiceName{Name: "disabled-lambda"}
	invalidLambdaService  = api.CompoundServiceName{Name: "invalid-lambda"}
)

func sampleConfig() xdscommon.PluginConfiguration {
	return xdscommon.PluginConfiguration{
		Kind: api.ServiceKindTerminatingGateway,
		ServiceConfigs: map[api.CompoundServiceName]xdscommon.ServiceConfig{
			lambdaService: {
				Kind: api.ServiceKindTerminatingGateway,
				EnvoyExtensions: []api.EnvoyExtension{
					{
						Name: "builtin/aws/lambda",
						Arguments: map[string]interface{}{
							"ARN":    "arn",
							"Region": "region",
						},
					},
				},
			},
			disabledLambdaService: {
				Kind: api.ServiceKindTerminatingGateway,
				// No extension.
			},
			invalidLambdaService: {
				Kind: api.ServiceKindTerminatingGateway,
				EnvoyExtensions: []api.EnvoyExtension{
					{
						Name:      "builtin/aws/lambda",
						Arguments: map[string]interface{}{}, // ARN, etc missing
					},
				},
			},
		},
		SNIToServiceName: map[string]api.CompoundServiceName{
			"lambda-sni": lambdaService,
		},
	}
}
