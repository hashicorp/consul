// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

// Asserter is a utility to help in reducing boilerplate in invoking test
// assertions against consul-topology Sprawl components.
//
// The methods should largely take in *topology.Service instances in lieu of
// ip/ports if there is only one port that makes sense for the assertion (such
// as use of the envoy admin port 19000).
//
// If it's up to the test (like picking an upstream) leave port as an argument
// but still take the service and use that to grab the local ip from the
// topology.Node.
type Asserter struct {
	sp SprawlLite
}

// *sprawl.Sprawl satisfies this. We don't need anything else.
type SprawlLite interface {
	HTTPClientForCluster(clusterName string) (*http.Client, error)
	APIClientForNode(clusterName string, nid topology.NodeID, token string) (*api.Client, error)
	APIClientForCluster(clusterName string, token string) (*api.Client, error)
	ResourceServiceClientForCluster(clusterName string) pbresource.ResourceServiceClient
	Topology() *topology.Topology
}

// NewAsserter creates a new assertion helper for the provided sprawl.
func NewAsserter(sp SprawlLite) *Asserter {
	return &Asserter{
		sp: sp,
	}
}

func (a *Asserter) mustGetHTTPClient(t testutil.TestingTB, cluster string) *http.Client {
	client, err := a.httpClientFor(cluster)
	require.NoError(t, err)
	return client
}

func (a *Asserter) mustGetAPIClient(t testutil.TestingTB, cluster string) *api.Client {
	clu := a.sp.Topology().Clusters[cluster]
	cl, err := a.sp.APIClientForCluster(clu.Name, "")
	require.NoError(t, err)
	return cl
}

// httpClientFor returns a pre-configured http.Client that proxies requests
// through the embedded squid instance in each LAN.
//
// Use this in methods below to magically pick the right proxied http client
// given the home of each node being checked.
func (a *Asserter) httpClientFor(cluster string) (*http.Client, error) {
	client, err := a.sp.HTTPClientForCluster(cluster)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// UpstreamEndpointStatus validates that proxy was configured with provided clusterName in the healthStatus
//
// Exposes libassert.UpstreamEndpointStatus for use against a Sprawl.
//
// NOTE: this doesn't take a port b/c you always want to use the envoy admin port.
func (a *Asserter) UpstreamEndpointStatus(
	t *testing.T,
	workload *topology.Workload,
	clusterName string,
	healthStatus string,
	count int,
) {
	t.Helper()
	node := workload.Node
	ip := node.LocalAddress()
	port := workload.EnvoyAdminPort
	addr := fmt.Sprintf("%s:%d", ip, port)

	client := a.mustGetHTTPClient(t, node.Cluster)
	libassert.AssertUpstreamEndpointStatusWithClient(t, client, addr, clusterName, healthStatus, count)
}

func (a *Asserter) getEnvoyClient(t *testing.T, workload *topology.Workload) (client *http.Client, addr string) {
	node := workload.Node
	ip := node.LocalAddress()
	port := workload.EnvoyAdminPort
	addr = fmt.Sprintf("%s:%d", ip, port)
	client = a.mustGetHTTPClient(t, node.Cluster)
	return client, addr
}

// AssertEnvoyRunningWithClient asserts that envoy is running by querying its stats page
func (a *Asserter) AssertEnvoyRunningWithClient(t *testing.T, workload *topology.Workload) {
	t.Helper()
	client, addr := a.getEnvoyClient(t, workload)
	libassert.AssertEnvoyRunningWithClient(t, client, addr)
}

// AssertEnvoyPresentsCertURIWithClient makes GET request to /certs endpoint and validates that
// two certificates URI is available in the response
func (a *Asserter) AssertEnvoyPresentsCertURIWithClient(t *testing.T, workload *topology.Workload) {
	t.Helper()
	client, addr := a.getEnvoyClient(t, workload)
	libassert.AssertEnvoyPresentsCertURIWithClient(t, client, addr, workload.ID.Name)
}

func (a *Asserter) AssertEnvoyHTTPrbacFiltersContainIntentions(t *testing.T, workload *topology.Workload) {
	t.Helper()
	client, addr := a.getEnvoyClient(t, workload)
	var (
		dump string
		err  error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 1 * time.Second}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		dump, _, err = libassert.GetEnvoyOutputWithClient(client, addr, "config_dump", map[string]string{})
		if err != nil {
			r.Fatal("could not fetch envoy configuration")
		}
	})
	// the steps below validate that the json result from envoy config dump configured active listeners with rbac and http filters
	filter := `.configs[2].dynamic_listeners[].active_state.listener | "\(.name) \( .filter_chains[].filters[] | select(.name == "envoy.filters.network.http_connection_manager") | .typed_config.http_filters | map(.name) | join(","))"`
	errorStateFilter := `.configs[2].dynamic_listeners[].error_state"`
	configErrorStates, _ := utils.JQFilter(dump, errorStateFilter)
	require.Nil(t, configErrorStates, "there should not be any error states on listener configuration")
	results, err := utils.JQFilter(dump, filter)
	require.NoError(t, err, "could not parse envoy configuration")
	var filteredResult []string
	for _, result := range results {
		parts := strings.Split(strings.ReplaceAll(result, `,`, " "), " ")
		sanitizedResult := append(parts[:0], parts[1:]...)
		filteredResult = append(filteredResult, sanitizedResult...)
	}
	require.Contains(t, filteredResult, "envoy.filters.http.rbac")
	require.Contains(t, filteredResult, "envoy.filters.http.router")
	require.Contains(t, dump, "intentions")
}

// HTTPServiceEchoes verifies that a post to the given ip/port combination
// returns the data in the response body. Optional path can be provided to
// differentiate requests.
//
// Exposes libassert.HTTPServiceEchoes for use against a Sprawl.
//
// NOTE: this takes a port b/c you may want to reach this via your choice of upstream.
func (a *Asserter) HTTPServiceEchoes(
	t *testing.T,
	workload *topology.Workload,
	port int,
	path string,
) {
	t.Helper()
	require.True(t, port > 0)

	node := workload.Node
	ip := node.LocalAddress()
	addr := fmt.Sprintf("%s:%d", ip, port)

	client := a.mustGetHTTPClient(t, node.Cluster)
	libassert.HTTPServiceEchoesWithClient(t, client, addr, path)
}

// HTTPServiceEchoesResHeader verifies that a post to the given ip/port combination
// returns the data in the response body with expected response headers.
// Optional path can be provided to differentiate requests.
//
// Exposes libassert.HTTPServiceEchoes for use against a Sprawl.
//
// NOTE: this takes a port b/c you may want to reach this via your choice of upstream.
func (a *Asserter) HTTPServiceEchoesResHeader(
	t *testing.T,
	workload *topology.Workload,
	port int,
	path string,
	expectedResHeader map[string]string,
) {
	t.Helper()
	require.True(t, port > 0)

	node := workload.Node
	ip := node.LocalAddress()
	addr := fmt.Sprintf("%s:%d", ip, port)

	client := a.mustGetHTTPClient(t, node.Cluster)
	libassert.HTTPServiceEchoesResHeaderWithClient(t, client, addr, path, expectedResHeader)
}

func (a *Asserter) HTTPStatus(
	t *testing.T,
	workload *topology.Workload,
	port int,
	status int,
) {
	t.Helper()
	require.True(t, port > 0)

	node := workload.Node
	ip := node.LocalAddress()
	addr := fmt.Sprintf("%s:%d", ip, port)

	client := a.mustGetHTTPClient(t, node.Cluster)

	url := "http://" + addr

	retry.RunWith(&retry.Timer{Timeout: 30 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
		resp, err := client.Get(url)
		if err != nil {
			r.Fatalf("could not make request to %q: %v", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != status {
			r.Fatalf("expected status %d, got %d", status, resp.StatusCode)
		}
	})
}

// asserts that the service sid in cluster and exported by peer localPeerName is passing health checks,
func (a *Asserter) HealthyWithPeer(t *testing.T, cluster string, sid topology.ID, peerName string) {
	t.Helper()
	cl := a.mustGetAPIClient(t, cluster)
	retry.RunWith(&retry.Timer{Timeout: time.Minute * 1, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
		svcs, _, err := cl.Health().Service(
			sid.Name,
			"",
			true,
			utils.CompatQueryOpts(&api.QueryOptions{
				Partition: sid.Partition,
				Namespace: sid.Namespace,
				Peer:      peerName,
			}),
		)
		require.NoError(r, err)
		assert.GreaterOrEqual(r, len(svcs), 1)
	})
}

type testingT interface {
	require.TestingT
	Helper()
}

// does a fortio /fetch2 to the given fortio service, targetting the given upstream. Returns
// the body, and response with response.Body already Closed.
//
// We treat 400, 503, and 504s as retryable errors
func (a *Asserter) fortioFetch2Upstream(
	t testutil.TestingTB,
	client *http.Client,
	addr string,
	us *topology.Upstream,
	path string,
) (body []byte, res *http.Response) {
	t.Helper()

	err, res := getFortioFetch2UpstreamResponse(t, client, addr, us, path, nil)
	require.NoError(t, err)
	defer res.Body.Close()

	// not sure when these happen, suspect it's when the mesh gateway in the peer is not yet ready
	require.NotEqual(t, http.StatusServiceUnavailable, res.StatusCode)
	require.NotEqual(t, http.StatusGatewayTimeout, res.StatusCode)
	// not sure when this happens, suspect it's when envoy hasn't configured the local upstream yet
	require.NotEqual(t, http.StatusBadRequest, res.StatusCode)
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)

	return body, res
}

func getFortioFetch2UpstreamResponse(t testutil.TestingTB, client *http.Client, addr string, us *topology.Upstream, path string, headers map[string]string) (error, *http.Response) {
	actualURL := fmt.Sprintf("http://localhost:%d/%s", us.LocalPort, path)

	url := fmt.Sprintf("http://%s/fortio/fetch2?url=%s", addr,
		url.QueryEscape(actualURL),
	)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	for h, v := range headers {
		req.Header.Set(h, v)
	}
	res, err := client.Do(req)
	require.NoError(t, err)
	return err, res
}

// uses the /fortio/fetch2 endpoint to do a header echo check against an
// upstream fortio
func (a *Asserter) FortioFetch2HeaderEcho(t *testing.T, fortioWrk *topology.Workload, us *topology.Upstream) {
	const kPassphrase = "x-passphrase"
	const passphrase = "hello"
	path := (fmt.Sprintf("/?header=%s:%s", kPassphrase, passphrase))

	var (
		node   = fortioWrk.Node
		addr   = fmt.Sprintf("%s:%d", node.LocalAddress(), fortioWrk.Port)
		client = a.mustGetHTTPClient(t, node.Cluster)
	)

	retry.RunWith(&retry.Timer{Timeout: 60 * time.Second, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
		_, res := a.fortioFetch2Upstream(r, client, addr, us, path)
		require.Equal(r, http.StatusOK, res.StatusCode)
		v := res.Header.Get(kPassphrase)
		require.Equal(r, passphrase, v)
	})
}

// similar to libassert.AssertFortioName,
// uses the /fortio/fetch2 endpoint to hit the debug endpoint on the upstream,
// and assert that the FORTIO_NAME == name
func (a *Asserter) FortioFetch2FortioName(
	t *testing.T,
	fortioWrk *topology.Workload,
	us *topology.Upstream,
	clusterName string,
	sid topology.ID,
) {
	t.Helper()

	var (
		node   = fortioWrk.Node
		addr   = fmt.Sprintf("%s:%d", node.LocalAddress(), fortioWrk.Port)
		client = a.mustGetHTTPClient(t, node.Cluster)
	)

	var fortioNameRE = regexp.MustCompile(("\nFORTIO_NAME=(.+)\n"))
	path := "/debug?env=dump"

	retry.RunWith(&retry.Timer{Timeout: 60 * time.Second, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
		body, res := a.fortioFetch2Upstream(r, client, addr, us, path)

		require.Equal(r, http.StatusOK, res.StatusCode)

		// TODO: not sure we should retry these?
		m := fortioNameRE.FindStringSubmatch(string(body))
		require.GreaterOrEqual(r, len(m), 2)
		// TODO: dedupe from NewFortioService
		require.Equal(r, fmt.Sprintf("%s::%s", clusterName, sid.String()), m[1])
	})
}

func (a *Asserter) FortioFetch2ServiceUnavailable(t *testing.T, fortioWrk *topology.Workload, us *topology.Upstream) {
	const kPassphrase = "x-passphrase"
	const passphrase = "hello"
	path := (fmt.Sprintf("/?header=%s:%s", kPassphrase, passphrase))
	a.FortioFetch2ServiceStatusCodes(t, fortioWrk, us, path, nil, []int{http.StatusServiceUnavailable})
}

// FortioFetch2ServiceStatusCodes uses the /fortio/fetch2 endpoint to do a header echo check against a upstream
// fortio and asserts that the returned status code matches the desired one(s)
func (a *Asserter) FortioFetch2ServiceStatusCodes(t *testing.T, fortioWrk *topology.Workload, us *topology.Upstream, path string, headers map[string]string, statuses []int) {
	var (
		node   = fortioWrk.Node
		addr   = fmt.Sprintf("%s:%d", node.LocalAddress(), fortioWrk.Port)
		client = a.mustGetHTTPClient(t, node.Cluster)
	)

	retry.RunWith(&retry.Timer{Timeout: 60 * time.Second, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
		_, res := getFortioFetch2UpstreamResponse(r, client, addr, us, path, headers)
		defer res.Body.Close()
		require.Contains(r, statuses, res.StatusCode)
	})
}

// CatalogServiceExists is the same as libassert.CatalogServiceExists, except that it uses
// a proxied API client
func (a *Asserter) CatalogServiceExists(t *testing.T, cluster string, svc string, opts *api.QueryOptions) {
	t.Helper()
	cl := a.mustGetAPIClient(t, cluster)
	libassert.CatalogServiceExists(t, cl, svc, opts)
}

// HealthServiceEntries asserts the service has the expected number of instances and checks
func (a *Asserter) HealthServiceEntries(t *testing.T, cluster string, node *topology.Node, svc string, passingOnly bool, opts *api.QueryOptions, expectedInstance int, expectedChecks int) []*api.ServiceEntry {
	t.Helper()
	cl, err := a.sp.APIClientForNode(cluster, node.ID(), "")
	require.NoError(t, err)
	health := cl.Health()

	var serviceEntries []*api.ServiceEntry
	retry.RunWith(&retry.Timer{Timeout: 60 * time.Second, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
		serviceEntries, _, err = health.Service(svc, "", passingOnly, opts)
		require.NoError(r, err)
		require.Equalf(r, expectedInstance, len(serviceEntries), "dc: %s, service: %s", cluster, serviceEntries[0].Service.Service)
		require.Equalf(r, expectedChecks, len(serviceEntries[0].Checks), "dc: %s, service: %s", cluster, serviceEntries[0].Service.Service)
	})

	return serviceEntries
}

// TokenExist asserts the token exists in the cluster and identical to the expected token
func (a *Asserter) TokenExist(t *testing.T, cluster string, node *topology.Node, expectedToken *api.ACLToken) {
	t.Helper()
	cl, err := a.sp.APIClientForNode(cluster, node.ID(), "")
	require.NoError(t, err)
	acl := cl.ACL()
	retry.RunWith(&retry.Timer{Timeout: 60 * time.Second, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
		retrievedToken, _, err := acl.TokenRead(expectedToken.AccessorID, &api.QueryOptions{})
		require.NoError(r, err)
		require.True(r, cmp.Equal(expectedToken, retrievedToken), "token %s", expectedToken.Description)
	})
}

// AutopilotHealth asserts the autopilot health endpoint return expected state
func (a *Asserter) AutopilotHealth(t *testing.T, cluster *topology.Cluster, leaderName string, expectedHealthy bool) {
	t.Helper()

	retry.RunWith(&retry.Timer{Timeout: 60 * time.Second, Wait: time.Millisecond * 10}, t, func(r *retry.R) {
		var out *api.OperatorHealthReply
		for _, node := range cluster.Nodes {
			if node.Name == leaderName {
				client, err := a.sp.APIClientForNode(cluster.Name, node.ID(), "")
				if err != nil {
					r.Log("err at node", node.Name, err)
					continue
				}
				operator := client.Operator()
				out, err = operator.AutopilotServerHealth(&api.QueryOptions{})
				if err != nil {
					r.Log("err at node", node.Name, err)
					continue
				}

				// Got the Autopilot server health response, break the loop
				r.Log("Got response at node", node.Name)
				break
			}
		}
		r.Log("out", out, "health", out.Healthy)
		require.Equal(r, expectedHealthy, out.Healthy)
	})
	return
}

type AuditEntry struct {
	CreatedAt time.Time `json:"created_at"`
	EventType string    `json:"event_type"`
	Payload   Payload   `json:"payload"`
}

type Payload struct {
	ID        string    `json:"id"`
	Version   string    `json:"version"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Auth      Auth      `json:"auth"`
	Request   Request   `json:"request"`
	Response  Response  `json:"response,omitempty"` // Response is not present in OperationStart log
	Stage     string    `json:"stage"`
}

type Auth struct {
	AccessorID  string    `json:"accessor_id"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"create_time"`
}

type Request struct {
	Operation   string            `json:"operation"`
	Endpoint    string            `json:"endpoint"`
	RemoteAddr  string            `json:"remote_addr"`
	UserAgent   string            `json:"user_agent"`
	Host        string            `json:"host"`
	QueryParams map[string]string `json:"query_params,omitempty"` // QueryParams might not be present in all cases
}

type Response struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"` // Error field is optional,shows up only with the status is non 2xx
}

func (a *Asserter) ValidateHealthEndpointAuditLog(t *testing.T, filePath string) {
	boolIsErrorJSON := false
	jsonData, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Print details of each AuditEntry
	var entry AuditEntry
	events := strings.Split(strings.TrimSpace(string(jsonData)), "\n")

	// function to validate the error object when the health endpoint is returning 429
	isErrorJSONObject := func(errorString string) error {
		// Attempt to unmarshal the error string into a map[string]interface{}
		var m map[string]interface{}
		err := json.Unmarshal([]byte(errorString), &m)
		return err
	}

	for _, event := range events {
		if strings.Contains(event, "/v1/operator/autopilot/health") && strings.Contains(event, "OperationComplete") {
			err = json.Unmarshal([]byte(event), &entry)
			require.NoError(t, err, "Error unmarshalling JSON: %v\", err")
			if entry.Payload.Response.Status == "429" {
				boolIsErrorJSON = true
				errResponse := entry.Payload.Response.Error
				err = isErrorJSONObject(errResponse)
				require.NoError(t, err, "Autopilot Health endpoint error response in the audit log is in unexpected format: %v\", err")
				break
			}
		}
	}
	require.Equal(t, boolIsErrorJSON, true, "Unable to verify audit log health endpoint error message")
}
