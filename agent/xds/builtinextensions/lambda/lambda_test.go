package lambda

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

func TestMakeLambdaExtension(t *testing.T) {
	kind := api.ServiceKindTerminatingGateway
	cases := map[string]struct {
		extensionName      string
		arn                string
		payloadPassthrough bool
		region             string
		expected           lambda
		ok                 bool
	}{
		"no arguments": {
			ok: false,
		},
		"a bad name": {
			arn:           "arn",
			region:        "blah",
			extensionName: "bad",
			ok:            false,
		},
		"missing arn": {
			region: "blah",
			ok:     false,
		},
		"missing region": {
			arn: "arn",
			ok:  false,
		},
		"including payload passthrough": {
			arn:                "arn",
			region:             "blah",
			payloadPassthrough: true,
			expected: lambda{
				ARN:                "arn",
				PayloadPassthrough: true,
				Region:             "blah",
				Kind:               kind,
			},
			ok: true,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			extensionName := api.BuiltinAWSLambdaExtension
			if tc.extensionName != "" {
				extensionName = tc.extensionName
			}
			svc := api.CompoundServiceName{Name: "svc"}
			ext := xdscommon.ExtensionConfiguration{
				ServiceName: svc,
				Upstreams: map[api.CompoundServiceName]xdscommon.UpstreamData{
					svc: {OutgoingProxyKind: kind},
				},
				EnvoyExtension: api.EnvoyExtension{
					Name: extensionName,
					Arguments: map[string]interface{}{
						"ARN":                tc.arn,
						"Region":             tc.region,
						"PayloadPassthrough": tc.payloadPassthrough,
					},
				},
			}

			plugin, err := MakeLambdaExtension(ext)

			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expected, plugin)
			} else {
				require.Error(t, err)
			}
		})
	}
}
