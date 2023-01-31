package assert

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-cleanhttp"

	"github.com/hashicorp/consul/sdk/testutil/retry"
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
		dump, err = GetEnvoyOutput(adminPort, "config_dump", map[string]string{})
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
		clusters, err = GetEnvoyOutput(adminPort, "clusters", map[string]string{"format": "json"})
		if err != nil {
			r.Fatal("could not fetch envoy clusters")
		}

		filter := fmt.Sprintf(`.cluster_statuses[] | select(.name|contains("%s")) | [.host_statuses[].health_status.eds_health_status] | [select(.[] == "%s")] | length`, clusterName, healthStatus)
		results, err := utils.JQFilter(clusters, filter)
		require.NoErrorf(r, err, "could not found cluster name %s", clusterName)
		require.Equal(r, count, len(results))
	})
}

// AssertEnvoyMetricAtMost assert the filered metric by prefix and metric is >= count
func AssertEnvoyMetricAtMost(t *testing.T, adminPort int, prefix, metric string, count int) {
	var (
		stats string
		err   error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 500 * time.Millisecond}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		stats, err = GetEnvoyOutput(adminPort, "stats", nil)
		if err != nil {
			r.Fatal("could not fetch envoy stats")
		}
		lines := strings.Split(stats, "\n")
		err = processMetrics(lines, prefix, metric, func(v int) bool {
			return v <= count
		})
		require.NoError(r, err)
	})
}

func processMetrics(metrics []string, prefix, metric string, condition func(v int) bool) error {
	for _, line := range metrics {
		if strings.Contains(line, prefix) &&
			strings.Contains(line, metric) {

			metric := strings.Split(line, ":")

			v, err := strconv.Atoi(strings.TrimSpace(metric[1]))
			if err != nil {
				return fmt.Errorf("err parse metric value %s: %s", metric[1], err)
			}

			if condition(v) {
				return nil
			}
		}
	}
	return fmt.Errorf("error processing stats")
}

// AssertEnvoyMetricAtLeast assert the filered metric by prefix and metric is <= count
func AssertEnvoyMetricAtLeast(t *testing.T, adminPort int, prefix, metric string, count int) {
	var (
		stats string
		err   error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 500 * time.Millisecond}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		stats, err = GetEnvoyOutput(adminPort, "stats", nil)
		if err != nil {
			r.Fatal("could not fetch envoy stats")
		}
		lines := strings.Split(stats, "\n")

		err = processMetrics(lines, prefix, metric, func(v int) bool {
			return v >= count
		})
		require.NoError(r, err)
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
		dump, err = GetEnvoyOutput(port, "config_dump", map[string]string{})
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

func GetEnvoyOutput(port int, path string, query map[string]string) (string, error) {
	client := cleanhttp.DefaultClient()
	var u url.URL
	u.Host = fmt.Sprintf("localhost:%d", port)
	u.Scheme = "http"
	if path != "" {
		u.Path = path
	}
	q := u.Query()
	for k, v := range query {
		q.Add(k, v)
	}
	if query != nil {
		u.RawQuery = q.Encode()
	}

	res, err := client.Get(u.String())
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
