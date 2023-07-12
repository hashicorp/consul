package serverlessplugin

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

func TestMakeLambdaPatcher(t *testing.T) {
	kind := api.ServiceKindTerminatingGateway
	cases := []struct {
		name               string
		enabled            bool
		arn                string
		payloadPassthrough bool
		region             string
		expected           lambdaPatcher
		ok                 bool
	}{
		{
			name: "no meta",
			ok:   true,
		},
		{
			name:    "lambda disabled",
			enabled: false,
			ok:      true,
		},
		{
			name:    "missing arn",
			enabled: true,
			region:  "blah",
			ok:      false,
		},
		{
			name:    "missing region",
			enabled: true,
			region:  "arn",
			ok:      false,
		},
		{
			name:               "including payload passthrough",
			enabled:            true,
			arn:                "arn",
			region:             "blah",
			payloadPassthrough: true,
			expected: lambdaPatcher{
				arn:                "arn",
				payloadPassthrough: true,
				region:             "blah",
				kind:               kind,
			},
			ok: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := xdscommon.ServiceConfig{
				Kind: kind,
				Meta: map[string]string{
					lambdaEnabledTag: strconv.FormatBool(tc.enabled),
					lambdaArnTag:     tc.arn,
					lambdaRegionTag:  tc.region,
				},
			}

			if tc.payloadPassthrough {
				config.Meta[lambdaPayloadPassthroughTag] = strconv.FormatBool(tc.payloadPassthrough)
			}

			patcher, ok := makeLambdaPatcher(config)

			require.Equal(t, tc.ok, ok)

			require.Equal(t, tc.expected, patcher)
		})
	}
}
