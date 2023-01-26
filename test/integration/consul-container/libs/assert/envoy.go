package assert

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetEnvoyListenerTCPFilters validates that proxy was configured with tcp protocol and one rbac listener filter
func GetEnvoyListenerTCPFilters(t *testing.T, adminPort int) {
	var (
		dump string
		err  error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 1 * time.Second}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		dump, err = libservice.GetEnvoyConfigDump(adminPort, "")
		if err != nil {
			r.Fatal("could not fetch envoy configuration")
		}
	})

	// The services configured for the tests have proxy tcp protocol configured, therefore the HTTP request is on tcp protocol
	// the steps below validate that the json result from envoy config dump returns active listener with rbac and tcp_proxy configured
	filter := `.configs[2].dynamic_listeners[].active_state.listener | "\(.name) \( .filter_chains[0].filters | map(.name) | join(","))"`
	results, err := utils.JQFilter(dump, filter)
	require.NoError(t, err, "could not parse envoy configuration")

	if len(results) != 2 {
		require.Error(t, fmt.Errorf("s1 proxy should have been configured with one rbac listener filter. Got %d listener(s)", len(results)))
	}

	var filteredResult []string
	for _, result := range results {
		santizedResult := sanitizeResult(result)
		filteredResult = append(filteredResult, santizedResult...)
	}

	require.Contains(t, filteredResult, "envoy.filters.network.rbac")
	require.Contains(t, filteredResult, "envoy.filters.network.tcp_proxy")
}

// AssertUpstreamEndpointStatus validates that proxy was configured with provided clusterName in the healthStatus
func AssertUpstreamEndpointStatus(t *testing.T, adminPort int, clusterName, healthStatus string, count int) {
	var (
		clusters string
		err      error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 500 * time.Millisecond}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		clusters, err = libservice.GetEnvoyClusters(adminPort)
		if err != nil {
			r.Fatal("could not fetch envoy configuration")
		}

		filter := fmt.Sprintf(`.cluster_statuses[] | select(.name|contains("%s")) | [.host_statuses[].health_status.eds_health_status] | [select(.[] == "%s")] | length`, clusterName, healthStatus)
		results, err := utils.JQFilter(clusters, filter)
		require.NoError(r, err, "could not parse envoy configuration")
		require.Equal(r, count, len(results))
	})
}

// GetEnvoyHTTPrbacFilters validates that proxy was configured with an http connection manager
// this assertion is currently unused current tests use http protocol
func GetEnvoyHTTPrbacFilters(t *testing.T, port int) {
	var (
		dump string
		err  error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 1 * time.Second}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		dump, err = libservice.GetEnvoyConfigDump(port, "")
		if err != nil {
			r.Fatal("could not fetch envoy configuration")
		}
	})

	// the steps below validate that the json result from envoy config dump configured active listeners with rbac and http filters
	filter := `.configs[2].dynamic_listeners[].active_state.listener | "\(.name) \( .filter_chains[0].filters[] | select(.name == "envoy.filters.network.http_connection_manager") | .typed_config.http_filters | map(.name) | join(","))"`
	results, err := utils.JQFilter(dump, filter)
	require.NoError(t, err, "could not parse envoy configuration")

	if len(results) != 2 {
		require.Error(t, fmt.Errorf("s1 proxy should have been configured with one rbac listener filter. Got %d listener(s)", len(results)))
	}

	var filteredResult []string
	for _, result := range results {
		sanitizedResult := sanitizeResult(result)
		filteredResult = append(filteredResult, sanitizedResult...)
	}
	require.Contains(t, filteredResult, "envoy.filters.http.rbac")
	assert.Contains(t, filteredResult, "envoy.filters.http.header_to_metadata")
	assert.Contains(t, filteredResult, "envoy.filters.http.router")
}

// sanitizeResult takes the value returned from config_dump json and cleans it up to remove special characters
// e.g public_listener:0.0.0.0:21001 envoy.filters.network.rbac,envoy.filters.network.tcp_proxy
// returns [envoy.filters.network.rbac envoy.filters.network.tcp_proxy]
func sanitizeResult(s string) []string {
	result := strings.Split(strings.ReplaceAll(s, `,`, " "), " ")
	return append(result[:0], result[1:]...)
}
