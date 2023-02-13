package assert

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/stretchr/testify/assert"
)

const (
	defaultHTTPTimeout = 120 * time.Second
	defaultHTTPWait    = defaultWait
)

// CatalogServiceExists verifies the service name exists in the Consul catalog
func CatalogServiceExists(t *testing.T, c *api.Client, svc string) {
	retry.Run(t, func(r *retry.R) {
		services, _, err := c.Catalog().Service(svc, "", nil)
		if err != nil {
			r.Fatal("error reading service data")
		}
		if len(services) == 0 {
			r.Fatal("did not find catalog entry for ", svc)
		}
	})
}

// CatalogServiceExists verifies the node name exists in the Consul catalog
func CatalogNodeExists(t *testing.T, c *api.Client, nodeName string) {
	retry.Run(t, func(r *retry.R) {
		node, _, err := c.Catalog().Node(nodeName, nil)
		if err != nil {
			r.Fatal("error reading node data")
		}
		if node == nil {
			r.Fatal("did not find node entry for", nodeName)
		}
	})
}

// HTTPServiceEchoes verifies that a post to the given ip/port combination returns the data
// in the response body. Optional path can be provided to differentiate requests.
func HTTPServiceEchoes(t *testing.T, ip string, port int, path string) {
	const phrase = "hello"

	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: defaultHTTPTimeout, Wait: defaultHTTPWait}
	}

	client := cleanhttp.DefaultClient()
	url := fmt.Sprintf("http://%s:%d", ip, port)

	if path != "" {
		url += "/" + path
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		t.Logf("making call to %s", url)
		reader := strings.NewReader(phrase)
		res, err := client.Post(url, "text/plain", reader)
		if err != nil {
			r.Fatal("could not make call to service ", url)
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			r.Fatal("could not read response body ", url)
		}

		if !strings.Contains(string(body), phrase) {
			r.Fatal("received an incorrect response ", string(body))
		}
	})
}

// ServiceLogContains returns true if the service container has the target string in its logs
func ServiceLogContains(t *testing.T, service libservice.Service, target string) bool {
	logs, err := service.GetLogs()
	require.NoError(t, err)
	return strings.Contains(logs, target)
}

// AssertFortioName asserts that the fortio service replying at urlbase/debug
// has a `FORTIO_NAME` env variable set. This validates that the client is sending
// traffic to the right envoy proxy.
//
// It retries with timeout defaultHTTPTimeout and wait defaultHTTPWait.
func AssertFortioName(t *testing.T, urlbase string, name string) {
	t.Helper()
	var fortioNameRE = regexp.MustCompile(("\nFORTIO_NAME=(.+)\n"))
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	retry.RunWith(&retry.Timer{Timeout: defaultHTTPTimeout, Wait: defaultHTTPWait}, t, func(r *retry.R) {
		fullurl := fmt.Sprintf("%s/debug?env=dump", urlbase)
		t.Logf("making call to %s", fullurl)
		req, err := http.NewRequest("GET", fullurl, nil)
		if err != nil {
			r.Fatal("could not make request to service ", fullurl)
		}

		resp, err := client.Do(req)
		if err != nil {
			r.Fatal("could not make call to service ", fullurl)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			r.Error(err)
			return
		}

		m := fortioNameRE.FindStringSubmatch(string(body))
		require.GreaterOrEqual(r, len(m), 2)
		t.Logf("got response from server name %s", m[1])
		assert.Equal(r, name, m[1])
	})
}

// AssertContainerState validates service container status
func AssertContainerState(t *testing.T, service libservice.Service, state string) {
	containerStatus, err := service.GetStatus()
	require.NoError(t, err)
	require.Equal(t, containerStatus, state, fmt.Sprintf("Expected: %s. Got %s", containerStatus, state))
}
