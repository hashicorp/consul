package xds

import (
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/stretchr/testify/require"
)

func TestUnusedExtensions(t *testing.T) {
	// This test asserts that some key protobuf structs are usable by escape
	// hatches despite not being directly used by Consul itself.

	type testcase struct {
		name  string
		input string
	}

	cases := []testcase{
		{
			"type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz",
			` {
			  "@type": "type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz",
			  "http_service": {
				"server_uri": {
				  "uri": "localhost:8090",
				  "cluster": "ext-authz",
				  "timeout": "0.250s"
				}
			  }
			} `,
		},
		{
			"type.googleapis.com/envoy.config.trace.v3.ZipkinConfig",
			` {
			  "@type": "type.googleapis.com/envoy.config.trace.v3.ZipkinConfig",
			  "collector_cluster": "zipkin",
			  "collector_endpoint_version": "HTTP_JSON",
			  "collector_endpoint": "/api/v1/spans",
			  "shared_span_context": false
			} `,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var any any.Any
			require.NoError(t, jsonpb.UnmarshalString(tc.input, &any))
			require.Equal(t, tc.name, any.TypeUrl)
		})
	}
}
