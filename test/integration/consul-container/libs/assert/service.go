// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package assert

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testing/deployer/util"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
)

const (
	defaultHTTPTimeout = 120 * time.Second
	defaultHTTPWait    = defaultWait
)

// CatalogV2ServiceExists verifies the service name exists in the Consul catalog
func CatalogV2ServiceExists(t *testing.T, client pbresource.ResourceServiceClient, svc string, tenancy *pbresource.Tenancy) {
	t.Helper()
	CatalogV2ServiceHasEndpointCount(t, client, svc, tenancy, -1)
}

// CatalogV2ServiceDoesNotExist verifies the service name does not exist in the Consul catalog
func CatalogV2ServiceDoesNotExist(t *testing.T, client pbresource.ResourceServiceClient, svc string, tenancy *pbresource.Tenancy) {
	t.Helper()
	ctx := testutil.TestContext(t)
	retry.Run(t, func(r *retry.R) {
		got, err := util.GetDecodedResource[*pbcatalog.Service](ctx, client, &pbresource.ID{
			Type:    pbcatalog.ServiceType,
			Name:    svc,
			Tenancy: tenancy,
		})
		require.NoError(r, err, "error reading service data")
		require.Nil(r, got, "unexpectedly found Service resource for %q", svc)

		got2, err := util.GetDecodedResource[*pbcatalog.ServiceEndpoints](ctx, client, &pbresource.ID{
			Type:    pbcatalog.ServiceEndpointsType,
			Name:    svc,
			Tenancy: tenancy,
		})
		require.NotNil(r, err, "error reading service data")
		require.Nil(r, got2, "unexpectedly found ServiceEndpoints resource for %q", svc)
	})
}

// CatalogV2ServiceHasEndpointCount verifies the service name exists in the Consul catalog and has the specified
// number of workload endpoints.
func CatalogV2ServiceHasEndpointCount(t *testing.T, client pbresource.ResourceServiceClient, svc string, tenancy *pbresource.Tenancy, count int) {
	t.Helper()

	ctx := testutil.TestContext(t)
	retry.Run(t, func(r *retry.R) {
		got, err := util.GetDecodedResource[*pbcatalog.Service](ctx, client, &pbresource.ID{
			Type:    pbcatalog.ServiceType,
			Name:    svc,
			Tenancy: tenancy,
		})
		require.NoError(r, err, "error reading service data")
		require.NotNil(r, got, "did not find Service resource for %q", svc)

		got2, err := util.GetDecodedResource[*pbcatalog.ServiceEndpoints](ctx, client, &pbresource.ID{
			Type:    pbcatalog.ServiceEndpointsType,
			Name:    svc,
			Tenancy: tenancy,
		})
		require.NoError(r, err, "error reading service data")
		require.NotNil(r, got2, "did not find ServiceEndpoints resource for %q", svc)
		require.NotEmpty(r, got2.Data.Endpoints, "did not find any workload data in the ServiceEndpoints resource for %q", svc)
		if count > 0 {
			require.Len(r, got2.Data.Endpoints, count)
		}
	})
}

// CatalogServiceExists verifies the service name exists in the Consul catalog
func CatalogServiceExists(t *testing.T, c *api.Client, svc string, opts *api.QueryOptions) {
	retry.Run(t, func(r *retry.R) {
		services, _, err := c.Catalog().Service(svc, "", opts)
		if err != nil {
			r.Fatal("error reading service data")
		}
		if len(services) == 0 {
			r.Fatalf("did not find catalog entry for %q with opts %#v", svc, opts)
		}
	})
}

// CatalogServiceDoesNotExist verifies the service name does not exist in the Consul catalog
func CatalogServiceDoesNotExist(t *testing.T, c *api.Client, svc string, opts *api.QueryOptions) {
	retry.Run(t, func(r *retry.R) {
		services, _, err := c.Catalog().Service(svc, "", opts)
		require.NoError(r, err, "error reading service data")
		require.Empty(r, services)
	})
}

// CatalogServiceHasInstanceCount verifies the service name exists in the Consul catalog and has the specified
// number of instances.
func CatalogServiceHasInstanceCount(t *testing.T, c *api.Client, svc string, count int, opts *api.QueryOptions) {
	retry.Run(t, func(r *retry.R) {
		services, _, err := c.Catalog().Service(svc, "", opts)
		if err != nil {
			r.Fatal("error reading service data")
		}
		if len(services) != count {
			r.Fatalf("did not find %d catalog entries for %s", count, svc)
		}
	})
}

// CatalogNodeExists verifies the node name exists in the Consul catalog
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

// CatalogNodeDoesNotExist verifies the node name does not exist in the Consul catalog
func CatalogNodeDoesNotExist(t *testing.T, c *api.Client, nodeName string) {
	retry.Run(t, func(r *retry.R) {
		node, _, err := c.Catalog().Node(nodeName, nil)
		if err != nil {
			r.Fatal("error reading node data")
		}
		require.Nil(r, node)
	})
}

// CatalogServiceIsHealthy verifies the service name exists and all instances pass healthchecks
func CatalogServiceIsHealthy(t *testing.T, c *api.Client, svc string, opts *api.QueryOptions) {
	CatalogServiceExists(t, c, svc, opts)

	retry.Run(t, func(r *retry.R) {
		services, _, err := c.Health().Service(svc, "", false, opts)
		if err != nil {
			r.Fatal("error reading service health data")
		}
		if len(services) == 0 {
			r.Fatal("did not find catalog entry for ", svc)
		}

		for _, svc := range services {
			for _, check := range svc.Checks {
				if check.Status != api.HealthPassing {
					r.Fatal("at least one check is not PASSING for service", svc.Service.Service)
				}
			}
		}

	})
}

func HTTPServiceEchoes(t *testing.T, ip string, port int, path string) {
	doHTTPServiceEchoes(t, ip, port, path, nil, nil)
}

func HTTPServiceEchoesWithHeaders(t *testing.T, ip string, port int, path string, headers map[string]string) {
	doHTTPServiceEchoes(t, ip, port, path, headers, nil)
}

func HTTPServiceEchoesWithClient(t *testing.T, client *http.Client, addr string, path string) {
	doHTTPServiceEchoesWithClient(t, client, addr, path, nil, nil)
}

func HTTPServiceEchoesResHeader(t *testing.T, ip string, port int, path string, expectedResHeader map[string]string) {
	doHTTPServiceEchoes(t, ip, port, path, nil, expectedResHeader)
}

func HTTPServiceEchoesResHeaderWithClient(t *testing.T, client *http.Client, addr string, path string, expectedResHeader map[string]string) {
	doHTTPServiceEchoesWithClient(t, client, addr, path, nil, expectedResHeader)
}

// HTTPServiceEchoes verifies that a post to the given ip/port combination returns the data
// in the response body. Optional path can be provided to differentiate requests.
func doHTTPServiceEchoes(t *testing.T, ip string, port int, path string, requestHeaders map[string]string, expectedResHeader map[string]string) {
	client := cleanhttp.DefaultClient()
	addr := fmt.Sprintf("%s:%d", ip, port)
	doHTTPServiceEchoesWithClient(t, client, addr, path, requestHeaders, expectedResHeader)
}

func doHTTPServiceEchoesWithClient(
	t *testing.T,
	client *http.Client,
	addr string,
	path string,
	requestHeaders map[string]string,
	expectedResHeader map[string]string,
) {
	const phrase = "hello"

	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: defaultHTTPTimeout, Wait: defaultHTTPWait}
	}

	url := "http://" + addr

	if path != "" {
		url += "/" + path
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		r.Logf("making call to %s", url)

		reader := strings.NewReader(phrase)
		req, err := http.NewRequest("POST", url, reader)
		require.NoError(r, err, "could not construct request")

		for k, v := range requestHeaders {
			req.Header.Add(k, v)

			if k == "Host" {
				req.Host = v
			}
		}

		res, err := client.Do(req)
		if err != nil {
			r.Fatal("could not make call to service ", url)
		}
		defer res.Body.Close()

		statusCode := res.StatusCode
		r.Logf("...got response code %d", statusCode)
		require.Equal(r, 200, statusCode)

		body, err := io.ReadAll(res.Body)
		if err != nil {
			r.Fatal("could not read response body ", url)
		}

		if !strings.Contains(string(body), phrase) {
			r.Fatal("received an incorrect response ", string(body))
		}

		for k, v := range expectedResHeader {
			if headerValues, ok := res.Header[k]; !ok {
				r.Fatal("expected header not found", k)
			} else {
				found := false
				for _, value := range headerValues {
					if value == v {
						found = true
						break
					}
				}

				if !found {
					r.Fatalf("header %s value not match want %s got %s ", k, v, headerValues)
				}
			}
		}
	})
}

// ServiceLogContains returns true if the service container has the target string in its logs
func ServiceLogContains(t *testing.T, service libservice.Service, target string) bool {
	logs, err := service.GetLogs()
	require.NoError(t, err)
	return strings.Contains(logs, target)
}

// AssertFortioName is a convenience function for [AssertFortioNameWithClient], using a [cleanhttp.DefaultClient()]
func AssertFortioName(t *testing.T, urlbase string, name string, reqHost string) {
	t.Helper()
	client := cleanhttp.DefaultClient()
	AssertFortioNameWithClient(t, urlbase, name, reqHost, client)
}

// AssertFortioNameWithClient asserts that the fortio service replying at urlbase/debug
// has a `FORTIO_NAME` env variable set. This validates that the client is sending
// traffic to the right envoy proxy.
//
// If reqHost is set, the Host field of the HTTP request will be set to its value.
//
// It retries with timeout defaultHTTPTimeout and wait defaultHTTPWait.
//
// client must be a custom http.Client
func AssertFortioNameWithClient(t *testing.T, urlbase string, name string, reqHost string, client *http.Client) {
	t.Helper()
	foundName, err := FortioNameWithClient(t, urlbase, name, reqHost, client)
	require.NoError(t, err)
	t.Logf("got response from server name %q expect %q", foundName, name)
	assert.Equal(t, name, foundName)
}

// WaitForFortioName is a convenience function for [WaitForFortioNameWithClient], using a [cleanhttp.DefaultClient()]
func WaitForFortioName(t *testing.T, r retry.Retryer, urlbase string, name string, reqHost string) {
	t.Helper()
	client := cleanhttp.DefaultClient()
	WaitForFortioNameWithClient(t, r, urlbase, name, reqHost, client)
}

// WaitForFortioNameWithClient enables waiting for FortioNameWithClient to return a specific
// value. It uses the provided Retryer to wait for the expected name and only fails when
// retries are exhausted.
//
// This is useful when performing failovers in tests and in other eventual consistency
// scenarios that may take multiple seconds to resolve.
//
// Note that the underlying FortioNameWithClient has its own retry for successfully making
// an HTTP request, which will be counted against the timeout of the provided Retryer if it
// is a Timer, or incorporated into each attempt if it is a Counter.
func WaitForFortioNameWithClient(t *testing.T, r retry.Retryer, urlbase string, name string, reqHost string, client *http.Client) {
	t.Helper()
	retry.RunWith(r, t, func(r *retry.R) {
		actual, err := FortioNameWithClient(r, urlbase, name, reqHost, client)
		require.NoError(r, err)
		if name != actual {
			r.Errorf("name %s did not match expected %s", name, actual)
		}
	})
}

// FortioNameWithClient returns the `FORTIO_NAME` returned by the fortio service at
// urlbase/debug. This can be used to validate that the client is sending traffic to
// the right envoy proxy.
//
// If reqHost is set, the Host field of the HTTP request will be set to its value.
//
// It retries with timeout defaultHTTPTimeout and wait defaultHTTPWait.
//
// client must be a custom http.Client
func FortioNameWithClient(t retry.TestingTB, urlbase string, name string, reqHost string, client *http.Client) (string, error) {
	t.Helper()
	var fortioNameRE = regexp.MustCompile("\nFORTIO_NAME=(.+)\n")
	var body []byte

	retry.RunWith(&retry.Timer{Timeout: defaultHTTPTimeout, Wait: defaultHTTPWait}, t, func(r *retry.R) {
		fullurl := fmt.Sprintf("%s/debug?env=dump", urlbase)
		req, err := http.NewRequest("GET", fullurl, nil)
		if err != nil {
			r.Fatalf("could not build request to %q: %v", fullurl, err)
		}
		if reqHost != "" {
			req.Host = reqHost
		}

		resp, err := client.Do(req)
		if err != nil {
			r.Fatalf("could not make request to %q: %v", fullurl, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			r.Fatalf("could not make request to %q: status %d", fullurl, resp.StatusCode)
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			r.Fatalf("failed to read response body from %q: %v", fullurl, err)
		}
	})

	m := fortioNameRE.FindStringSubmatch(string(body))
	if len(m) < 2 {
		return "", fmt.Errorf("fortio name not found %s", name)
	}
	return m[1], nil
}

// AssertContainerState validates service container status
func AssertContainerState(t *testing.T, service libservice.Service, state string) {
	containerStatus, err := service.GetStatus()
	require.NoError(t, err)
	require.Equal(t, containerStatus, state, fmt.Sprintf("Expected: %s. Got %s", state, containerStatus))
}
