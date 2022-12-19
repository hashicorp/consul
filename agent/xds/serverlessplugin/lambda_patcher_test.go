package serverlessplugin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

func TestMakeLambdaPatcher(t *testing.T) {
	kind := api.ServiceKindTerminatingGateway
	cases := []struct {
		name               string
		arn                string
		payloadPassthrough bool
		region             string
		expected           lambdaPatcher
		ok                 bool
	}{
		{
			name: "no extension",
			ok:   false,
		},
		{
			name:   "missing arn",
			region: "blah",
			ok:     false,
		},
		{
			name: "missing region",
			arn:  "arn",
			ok:   false,
		},
		{
			name:               "including payload passthrough",
			arn:                "arn",
			region:             "blah",
			payloadPassthrough: true,
			expected: lambdaPatcher{
				ARN:                "arn",
				PayloadPassthrough: true,
				Region:             "blah",
				Kind:               kind,
			},
			ok: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := xdscommon.ServiceConfig{
				Kind: kind,
				EnvoyExtensions: []api.EnvoyExtension{
					{
						Name: "builtin/aws/lambda",
						Arguments: map[string]interface{}{
							"ARN":                tc.arn,
							"Region":             tc.region,
							"PayloadPassthrough": tc.payloadPassthrough,
						},
					},
				},
			}

			patcher, ok := makeLambdaPatcher(config)

			require.Equal(t, tc.ok, ok)

			if tc.ok {
				require.Equal(t, tc.expected, patcher)
			}
		})
	}
}
