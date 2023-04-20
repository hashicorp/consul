package assert

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/go-cleanhttp"
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
		dump, _, err = GetEnvoyOutput(adminPort, "config_dump", map[string]string{})
		if err != nil {
			r.Fatal("could not fetch envoy configuration")
		}
	})

	// The services configured for the tests have proxy tcp protocol configured, therefore the HTTP request is on tcp protocol
	// the steps below validate that the json result from envoy config dump returns active listener with rbac and tcp_proxy configured
	filter := `.configs[2].dynamic_listeners[].active_state.listener | "\(.name) \( .filter_chains[0].filters | map(.name) | join(","))"`
	results, err := utils.JQFilter(dump, filter)
	require.NoError(t, err, "could not parse envoy configuration")
	require.Len(t, results, 2, "static-server proxy should have been configured with two listener filters")

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
		clusters, _, err = GetEnvoyOutput(adminPort, "clusters", map[string]string{"format": "json"})
		if err != nil {
			r.Fatal("could not fetch envoy clusters")
		}

		filter := fmt.Sprintf(`.cluster_statuses[] | select(.name|contains("%s")) | [.host_statuses[].health_status.eds_health_status] | [select(.[] == "%s")] | length`, clusterName, healthStatus)
		results, err := utils.JQFilter(clusters, filter)
		require.NoErrorf(r, err, "could not found cluster name %s", clusterName)

		resultToString := strings.Join(results, " ")
		result, err := strconv.Atoi(resultToString)
		assert.NoError(r, err)
		require.Equal(r, count, result)
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
		stats, _, err = GetEnvoyOutput(adminPort, "stats", nil)
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
	var err error
	for _, line := range metrics {
		if strings.Contains(line, prefix) &&
			strings.Contains(line, metric) {
			var value int
			metric := strings.Split(line, ":")

			value, err = strconv.Atoi(strings.TrimSpace(metric[1]))
			if err != nil {
				return fmt.Errorf("err parse metric value %s: %s", metric[1], err)
			}

			if condition(value) {
				return nil
			} else {
				return fmt.Errorf("metric value doesn's satisfy condition: %d", value)
			}
		}
	}
	return fmt.Errorf("error metric %s %s not found", prefix, metric)
}

// AssertEnvoyMetricAtLeast assert the filered metric by prefix and metric is <= count
func AssertEnvoyMetricAtLeast(t *testing.T, adminPort int, prefix, metric string, count int) {
	var (
		stats string
		err   error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 60 * time.Second, Wait: 500 * time.Millisecond}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		stats, _, err = GetEnvoyOutput(adminPort, "stats", nil)
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
// AssertEnvoyHTTPrbacFilters validates that proxy was configured with an http connection manager
// this assertion is currently unused current tests use http protocol
func AssertEnvoyHTTPrbacFilters(t *testing.T, port int) {
	var (
		dump string
		err  error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 1 * time.Second}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		dump, _, err = GetEnvoyOutput(port, "config_dump", map[string]string{})
		if err != nil {
			r.Fatal("could not fetch envoy configuration")
		}
	})

	// the steps below validate that the json result from envoy config dump configured active listeners with rbac and http filters
	filter := `.configs[2].dynamic_listeners[].active_state.listener | "\(.name) \( .filter_chains[0].filters[] | select(.name == "envoy.filters.network.http_connection_manager") | .typed_config.http_filters | map(.name) | join(","))"`
	results, err := utils.JQFilter(dump, filter)
	require.NoError(t, err, "could not parse envoy configuration")
	require.Len(t, results, 1, "static-server proxy should have been configured with two listener filters.")

	var filteredResult []string
	for _, result := range results {
		sanitizedResult := sanitizeResult(result)
		filteredResult = append(filteredResult, sanitizedResult...)
	}
	require.Contains(t, filteredResult, "envoy.filters.http.rbac")
	assert.Contains(t, filteredResult, "envoy.filters.http.header_to_metadata")
	assert.Contains(t, filteredResult, "envoy.filters.http.router")
}

// AssertEnvoyPresentsCertURI makes GET request to /certs endpoint and validates that
// two certificates URI is available in the response
func AssertEnvoyPresentsCertURI(t *testing.T, port int, serviceName string) {
	var (
		dump string
		err  error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 1 * time.Second}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		dump, _, err = GetEnvoyOutput(port, "certs", nil)
		if err != nil {
			r.Fatal("could not fetch envoy configuration")
		}
		require.NotNil(r, dump)
	})

	// Validate certificate uri
	filter := `.certificates[] | .cert_chain[].subject_alt_names[].uri`
	results, err := utils.JQFilter(dump, filter)
	require.NoError(t, err, "could not parse envoy configuration")
	if len(results) >= 1 {
		require.Error(t, fmt.Errorf("client and server proxy should have been configured with certificate uri"))
	}

	for _, cert := range results {
		cert, err := regexp.MatchString(fmt.Sprintf("spiffe://[a-zA-Z0-9-]+.consul/ns/%s/dc/%s/svc/%s", "default", "dc1", serviceName), cert)
		require.NoError(t, err)
		assert.True(t, cert)
	}
}

// AssertEnvoyRunning assert the envoy is running by querying its stats page
func AssertEnvoyRunning(t *testing.T, port int) {
	var (
		err error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 10 * time.Second, Wait: 500 * time.Millisecond}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		_, _, err = GetEnvoyOutput(port, "stats", nil)
		if err != nil {
			r.Fatal("could not fetch envoy stats")
		}
	})
}

func GetEnvoyOutput(port int, path string, query map[string]string) (string, int, error) {
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
		return "", 0, err
	}
	statusCode := res.StatusCode
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", statusCode, err
	}

	return string(body), statusCode, nil
}

// sanitizeResult takes the value returned from config_dump json and cleans it up to remove special characters
// e.g public_listener:0.0.0.0:21001 envoy.filters.network.rbac,envoy.filters.network.tcp_proxy
// returns [envoy.filters.network.rbac envoy.filters.network.tcp_proxy]
func sanitizeResult(s string) []string {
	result := strings.Split(strings.ReplaceAll(s, `,`, " "), " ")
	return append(result[:0], result[1:]...)
}

// AssertServiceHasHealthyInstances asserts the number of instances of service equals count for a given service.
// https://developer.hashicorp.com/consul/docs/connect/config-entries/service-resolver#onlypassing
func AssertServiceHasHealthyInstances(t *testing.T, node libcluster.Agent, service string, onlypassing bool, count int) {
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 10 * time.Second, Wait: 500 * time.Millisecond}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		services, _, err := node.GetClient().Health().Service(service, "", onlypassing, nil)
		require.NoError(r, err)
		for _, v := range services {
			fmt.Printf("%s service status: %s\n", v.Service.ID, v.Checks.AggregatedStatus())
		}
		require.Equal(r, count, len(services))
	})
}
