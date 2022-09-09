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
		expected []patcher
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
			expected: []patcher{
				lambdaPatcher{
					arn:                "arn",
					region:             "region",
					payloadPassthrough: false,
					kind:               api.ServiceKindTerminatingGateway,
				},
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
			patchers := getPatchersBySNI(config, tc.sni)

			if tc.expected == nil {
				require.Empty(t, patchers)
			} else {
				require.Equal(t, tc.expected, patchers)
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
				Meta: map[string]string{
					lambdaEnabledTag: "true",
					lambdaArnTag:     "arn",
					lambdaRegionTag:  "region",
				},
			},
			disabledLambdaService: {
				Kind: api.ServiceKindTerminatingGateway,
				Meta: map[string]string{
					lambdaEnabledTag: "false",
					lambdaArnTag:     "arn",
					lambdaRegionTag:  "region",
				},
			},
			invalidLambdaService: {
				Kind: api.ServiceKindTerminatingGateway,
				Meta: map[string]string{
					lambdaEnabledTag: "true",
				},
			},
		},
		SNIToServiceName: map[string]api.CompoundServiceName{
			"lambda-sni": lambdaService,
		},
	}
}
