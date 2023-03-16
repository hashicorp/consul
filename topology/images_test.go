package topology

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImages_EnvoyConsulImage(t *testing.T) {
	type testcase struct {
		consul, envoy string
		expect        string
	}

	run := func(t *testing.T, tc testcase) {
		i := Images{Consul: tc.consul, Envoy: tc.envoy}
		j := i.EnvoyConsulImage()
		require.Equal(t, tc.expect, j)
	}

	cases := []testcase{
		{
			consul: "",
			envoy:  "",
			expect: "",
		},
		{
			consul: "consul",
			envoy:  "",
			expect: "",
		},
		{
			consul: "",
			envoy:  "envoy",
			expect: "",
		},
		{
			consul: "consul",
			envoy:  "envoy",
			expect: "local/consul-and-envoy:latest-with-latest",
		},
		// repos
		{
			consul: "hashicorp/consul",
			envoy:  "envoy",
			expect: "local/hashicorp-consul-and-envoy:latest-with-latest",
		},
		{
			consul: "consul",
			envoy:  "envoyproxy/envoy",
			expect: "local/consul-and-envoyproxy-envoy:latest-with-latest",
		},
		{
			consul: "hashicorp/consul",
			envoy:  "envoyproxy/envoy",
			expect: "local/hashicorp-consul-and-envoyproxy-envoy:latest-with-latest",
		},
		// tags
		{
			consul: "consul:1.15.0",
			envoy:  "envoy",
			expect: "local/consul-and-envoy:1.15.0-with-latest",
		},
		{
			consul: "consul",
			envoy:  "envoy:v1.26.1",
			expect: "local/consul-and-envoy:latest-with-v1.26.1",
		},
		{
			consul: "consul:1.15.0",
			envoy:  "envoy:v1.26.1",
			expect: "local/consul-and-envoy:1.15.0-with-v1.26.1",
		},
		// repos+tags
		{
			consul: "hashicorp/consul:1.15.0",
			envoy:  "envoy:v1.26.1",
			expect: "local/hashicorp-consul-and-envoy:1.15.0-with-v1.26.1",
		},
		{
			consul: "consul:1.15.0",
			envoy:  "envoyproxy/envoy:v1.26.1",
			expect: "local/consul-and-envoyproxy-envoy:1.15.0-with-v1.26.1",
		},
		{
			consul: "hashicorp/consul:1.15.0",
			envoy:  "envoyproxy/envoy:v1.26.1",
			expect: "local/hashicorp-consul-and-envoyproxy-envoy:1.15.0-with-v1.26.1",
		},
	}

	for i, tc := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			run(t, tc)
		})
	}
}
