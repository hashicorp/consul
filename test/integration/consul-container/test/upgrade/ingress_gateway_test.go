// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package upgrade

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// These tests adapt BATS-based tests from test/integration/connect/case-ingress-gateway*

// TestIngressGateway_UpgradeToTarget_fromLatest:
// - starts a cluster with 2 static services,
// - configures an ingress gateway + router with TLS
// - performs tests:
//   - envoy is configured with thresholds (e.g max connections) and health checks
//   - HTTP header manipulation
//   - per-service and wildcard and custom hostnames work
//
// - upgrades the cluster
// - performs these tests again
func TestIngressGateway_UpgradeToTarget_fromLatest(t *testing.T) {
	t.Parallel()

	cluster, _, client := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers: 1,
		NumClients: 2,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:      "dc1",
			ConsulImageName: utils.GetLatestImageName(),
			ConsulVersion:   utils.LatestVersion,
		},
		ApplyDefaultProxySettings: true,
	})

	require.NoError(t, cluster.ConfigEntryWrite(&api.ProxyConfigEntry{
		Name: api.ProxyConfigGlobal,
		Kind: api.ProxyDefaults,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))

	const (
		nameIG     = "ingress-gateway"
		nameRouter = "router"
	)

	const nameS1 = libservice.StaticServerServiceName
	const nameS2 = libservice.StaticServer2ServiceName
	require.NoError(t, cluster.ConfigEntryWrite(&api.ServiceRouterConfigEntry{
		Kind: api.ServiceRouter,
		// This is a "virtual" service name and will not have a backing
		// service definition. It must match the name defined in the ingress
		// configuration.
		Name: nameRouter,
		Routes: []api.ServiceRoute{
			{
				Match: &api.ServiceRouteMatch{
					HTTP: &api.ServiceRouteHTTPMatch{
						PathPrefix: fmt.Sprintf("/%s/", nameS1),
					},
				},
				Destination: &api.ServiceRouteDestination{
					Service:       nameS1,
					PrefixRewrite: "/",
				},
			},
			{
				Match: &api.ServiceRouteMatch{
					HTTP: &api.ServiceRouteHTTPMatch{
						PathPrefix: fmt.Sprintf("/%s/", nameS2),
					},
				},
				Destination: &api.ServiceRouteDestination{
					Service:       nameS2,
					PrefixRewrite: "/",
				},
			},
		},
	}))

	gwCfg := libservice.GatewayConfig{
		Name: nameIG,
		Kind: "ingress",
	}
	igw, err := libservice.NewGatewayService(context.Background(), gwCfg, cluster.Servers()[0])
	require.NoError(t, err)

	// these must be one of the externally-mapped ports from
	// https://github.com/hashicorp/consul/blob/c5e729e86576771c4c22c6da1e57aaa377319323/test/integration/consul-container/libs/cluster/container.go#L521-L525
	const portRouter = 8080
	const portWildcard = 9997
	const portS1Direct = 9998
	const portS1DirectCustomHostname = 9999
	const hostnameS1DirectCustom = "test.example.com"
	// arbitrary numbers
	var (
		overrideOffset              = uint32(10000)
		igwDefaultMaxConns          = uint32(3572)
		igwDefaultMaxPendingReqs    = uint32(7644)
		igwDefaultMaxConcurrentReqs = uint32(7637)
		// PHC = PassiveHealthCheck
		igwDefaultPHCMaxFailures = uint32(7382)
		s1MaxConns               = igwDefaultMaxConns + overrideOffset
		s1MaxPendingReqs         = igwDefaultMaxConcurrentReqs + overrideOffset
	)
	const (
		igwDefaultPHCIntervalS = 7820
	)
	igwDefaults := api.IngressServiceConfig{
		MaxConnections:        &igwDefaultMaxConns,
		MaxPendingRequests:    &igwDefaultMaxPendingReqs,
		MaxConcurrentRequests: &igwDefaultMaxConcurrentReqs,
	}
	// passive health checks were introduced in 1.15
	if utils.VersionGTE(utils.LatestVersion, "1.15") {
		igwDefaults.PassiveHealthCheck = &api.PassiveHealthCheck{
			Interval:    igwDefaultPHCIntervalS * time.Second,
			MaxFailures: igwDefaultPHCMaxFailures,
		}
	}
	require.NoError(t, cluster.ConfigEntryWrite(&api.IngressGatewayConfigEntry{
		Kind:     api.IngressGateway,
		Name:     nameIG,
		Defaults: &igwDefaults,
		TLS: api.GatewayTLSConfig{
			Enabled:       true,
			TLSMinVersion: "TLSv1_2",
		},
		Listeners: []api.IngressListener{
			{
				Port:     portRouter,
				Protocol: "http",
				Services: []api.IngressService{
					{
						Name: nameRouter,
						// for "request header manipulation" subtest
						RequestHeaders: &api.HTTPHeaderModifiers{
							Add: map[string]string{
								"x-foo":        "bar-req",
								"x-existing-1": "appended-req",
							},
							Set: map[string]string{
								"x-existing-2": "replaced-req",
								"x-client-ip":  "%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT%",
							},
							Remove: []string{"x-bad-req"},
						},
						// for "response header manipulation" subtest
						ResponseHeaders: &api.HTTPHeaderModifiers{
							Add: map[string]string{
								"x-foo":        "bar-resp",
								"x-existing-1": "appended-resp",
							},
							Set: map[string]string{
								"x-existing-2": "replaced-resp",
							},
							Remove: []string{"x-bad-resp"},
						},
					},
				},
			},
			// for "envoy config/thresholds" subtest
			{
				Port:     portS1Direct,
				Protocol: "http",
				Services: []api.IngressService{
					{
						Name:               libservice.StaticServerServiceName,
						MaxConnections:     &s1MaxConns,
						MaxPendingRequests: &s1MaxPendingReqs,
					},
				},
			},
			// for "hostname=custom" subtest
			{
				Port:     portS1DirectCustomHostname,
				Protocol: "http",
				Services: []api.IngressService{
					{
						Name:  libservice.StaticServerServiceName,
						Hosts: []string{hostnameS1DirectCustom},
					},
				},
			},
			// for "hostname=*" subtest
			{
				Port:     portWildcard,
				Protocol: "http",
				Services: []api.IngressService{
					{
						Name: "*",
					},
				},
			},
		},
	}))

	// create s1
	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(
		cluster.Clients()[0],
		&libservice.ServiceOpts{
			Name:     nameS1,
			ID:       nameS1,
			HTTPPort: 8080,
			GRPCPort: 8079,
		},
	)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, nameS1, nil)

	// create s2
	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(
		cluster.Clients()[1],
		&libservice.ServiceOpts{
			Name:     nameS2,
			ID:       nameS2,
			HTTPPort: 8080,
			GRPCPort: 8079,
		},
	)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, nameS2, nil)

	// checks
	// TODO: other checks from verify.bats
	// ingress-gateway proxy admin up
	// s1 proxy admin up
	// s2 proxy admin up
	// s1 proxy listener has right cert
	// s2 proxy listener has right cert
	// ig1 has healthy endpoints for s1
	// ig1 has healthy endpoints for s2
	// TODO ^ ??? s1 and s2 aren't direct listeners, only in `router`, so why are they endpoints?

	roots, _, err := client.Connect().CARoots(&api.QueryOptions{})
	var root *api.CARoot
	for _, r := range roots.Roots {
		if r.Active {
			root = r
			break
		}
	}
	require.NotNil(t, root, "no active CA root found")

	// tests
	tests := func(t *testing.T) {
		// fortio name should be $nameS<X> for /$nameS<X> prefix on router
		portRouterMapped, _ := cluster.Servers()[0].GetPod().MappedPort(
			context.Background(),
			nat.Port(fmt.Sprintf("%d/tcp", portRouter)),
		)
		reqHost := fmt.Sprintf("router.ingress.consul:%d", portRouter)

		httpClient := httpClientWithCA(t, reqHost, root.RootCertPEM)

		t.Run("fortio name", func(t *testing.T) {
			// TODO: occasionally (1 in 5 or so), service 1 gets stuck throwing 503s
			// - direct connection works fine
			// - its envoy has some 503s in stats, and some 200s
			// - igw envoy says all 503s in stats
			libassert.AssertFortioNameWithClient(t,
				fmt.Sprintf("https://localhost:%d/%s", portRouterMapped.Int(), nameS1), nameS1, reqHost, httpClient)
			libassert.AssertFortioNameWithClient(t,
				fmt.Sprintf("https://localhost:%d/%s", portRouterMapped.Int(), nameS2), nameS2, reqHost, httpClient)
		})
		urlbaseS2 := fmt.Sprintf("https://%s/%s", reqHost, nameS2)

		t.Run("envoy config", func(t *testing.T) {
			var dump string
			_, adminPort := igw.GetAdminAddr()
			retry.RunWith(&retry.Timer{Timeout: 30 * time.Second, Wait: 1 * time.Second}, t, func(r *retry.R) {
				dump, _, err = libassert.GetEnvoyOutput(adminPort, "config_dump", map[string]string{})
				if err != nil {
					r.Fatal("could not fetch envoy configuration")
				}
			})
			var m interface{}
			err = json.Unmarshal([]byte(dump), &m)
			require.NoError(t, err)

			q, err := gojq.Parse(fmt.Sprintf(`.configs[1].dynamic_active_clusters[]
				| select(.cluster.name|startswith("%s."))
				| .cluster`, nameS1))
			require.NoError(t, err)
			it := q.Run(m)
			v, ok := it.Next()
			require.True(t, ok)
			t.Run("thresholds", func(t *testing.T) {
				// TODO: these fail about 10% of the time on my machine, giving me only the defaults, not the override
				// writing the config again (with a different value) usually works
				// https://hashicorp.slack.com/archives/C03UNBBDELS/p1677621125567219
				t.Skip("BUG? thresholds not set about 10% of the time")
				thresholds := v.(map[string]any)["circuit_breakers"].(map[string]any)["thresholds"].([]map[string]any)[0]
				assert.Equal(t, float64(s1MaxConns), thresholds["max_connections"].(float64), "max conns from override")
				assert.Equal(t, float64(s1MaxPendingReqs), thresholds["max_pending_requests"].(float64), "max pending conns from override")
				assert.Equal(t, float64(*igwDefaults.MaxConcurrentRequests), thresholds["max_requests"].(float64), "max requests from defaults")
			})
			t.Run("outlier detection", func(t *testing.T) {
				if utils.VersionLT(utils.LatestVersion, "1.15") {
					t.Skipf("version %s (< 1.15) IGW doesn't support Defaults.PassiveHealthCheck", utils.LatestVersion)
				}
				// BATS checks against S2, but we're doing S1 just to avoid more jq
				o := v.(map[string]any)["outlier_detection"].(map[string]any)
				assert.Equal(t,
					fmt.Sprintf("%ds", igwDefaultPHCIntervalS),
					o["interval"].(string),
					"interval: s1 == default",
				)
				assert.Equal(t, float64(igwDefaultPHCMaxFailures), o["consecutive_5xx"].(float64), "s1 max failures == default")
				_, ec5xx_ok := o["enforcing_consecutive_5xx"]
				assert.False(t, ec5xx_ok, "s1 enforcing_consective_5xx: unset")
			})
		})

		t.Run("request header manipulation", func(t *testing.T) {
			resp := mappedHTTPGET(t, fmt.Sprintf("%s/debug?env=dump", urlbaseS2), portRouterMapped.Int(), http.Header(map[string][]string{
				"X-Existing-1": {"original"},
				"X-Existing-2": {"original"},
				"X-Bad-Req":    {"true"},
			}), nil, httpClient)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// The following check the body, which should echo the headers received
			// by the fortio container
			assert.Contains(t, string(body), "X-Foo: bar-req",
				"Ingress should have added the new request header")
			assert.Contains(t, string(body), "X-Existing-1: original,appended-req",
				"Ingress should have appended the first existing header - both should be present")
			assert.Contains(t, string(body), "X-Existing-2: replaced-req",
				"Ingress should have replaced the second existing header")
			// This 172. is the prefix of the IP for the gateway for our docker network.
			// Perhaps there's some way to look this up.
			// This is a deviation from BATS, because their tests run inside Docker, and ours run outside.
			assert.Contains(t, string(body), "X-Client-Ip: 172.",
				"Ingress should have set the client ip from dynamic Envoy variable")
			assert.NotContains(t, string(body), "X-Bad-Req: true",
				"Ingress should have removed the bad request header")

		})

		t.Run("response header manipulation", func(t *testing.T) {
			const params = "?header=x-bad-resp:true&header=x-existing-1:original&header=x-existing-2:original"
			resp := mappedHTTPGET(t,
				fmt.Sprintf("%s/echo%s", urlbaseS2, params),
				portRouterMapped.Int(),
				nil,
				nil,
				httpClient,
			)
			defer resp.Body.Close()

			assert.Contains(t, resp.Header.Values("x-foo"), "bar-resp",
				"Ingress should have added the new response header")
			assert.Contains(t, resp.Header.Values("x-existing-1"), "original",
				"Ingress should have appended the first existing header - both should be present")
			assert.Contains(t, resp.Header.Values("x-existing-1"), "appended-resp",
				"Ingress should have appended the first existing header - both should be present")
			assert.Contains(t, resp.Header.Values("x-existing-2"), "replaced-resp",
				"Ingress should have replaced the second existing header")
			assert.NotContains(t, resp.Header.Values("x-existing-2"), "original",
				"x-existing-2 response header should have been overridden")
			assert.NotContains(t, resp.Header.Values("x-bad-resp"), "true",
				"X-Bad-Resp response header should have been stripped")
		})

		t.Run("hostname=custom", func(t *testing.T) {
			pm, _ := cluster.Servers()[0].GetPod().MappedPort(
				context.Background(),
				nat.Port(fmt.Sprintf("%d/tcp", portS1DirectCustomHostname)),
			)
			h := fmt.Sprintf("%s:%d", hostnameS1DirectCustom, portS1DirectCustomHostname)
			clS1Direct := httpClientWithCA(t, h, root.RootCertPEM)
			const data = "secret password"
			resp := mappedHTTPGET(t,
				"https://"+h,
				pm.Int(),
				nil,
				strings.NewReader(data),
				clS1Direct,
			)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, []byte(data), body)
		})

		t.Run("hostname=<service>.ingress.consul", func(t *testing.T) {
			pm, _ := cluster.Servers()[0].GetPod().MappedPort(
				context.Background(),
				nat.Port(fmt.Sprintf("%d/tcp", portS1Direct)),
			)
			h := fmt.Sprintf("%s.ingress.consul:%d", libservice.StaticServerServiceName, portS1Direct)
			clS1Direct := httpClientWithCA(t, h, root.RootCertPEM)
			const data = "secret password"
			resp := mappedHTTPGET(t,
				"https://"+h,
				pm.Int(),
				nil,
				strings.NewReader(data),
				clS1Direct,
			)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, []byte(data), body)
		})
		t.Run("hostname=*", func(t *testing.T) {
			pm, _ := cluster.Servers()[0].GetPod().MappedPort(
				context.Background(),
				nat.Port(fmt.Sprintf("%d/tcp", portWildcard)),
			)

			t.Run("s1 HTTPS echo validates against our CA", func(t *testing.T) {
				h := fmt.Sprintf("%s.ingress.consul:%d", libservice.StaticServerServiceName, portWildcard)
				cl := httpClientWithCA(t, h, root.RootCertPEM)
				data := fmt.Sprintf("secret-%s", libservice.StaticClientServiceName)
				resp := mappedHTTPGET(t,
					"https://"+h,
					pm.Int(),
					nil,
					strings.NewReader(data),
					cl,
				)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Equal(t, []byte(data), body)
			})

			t.Run("s2 HTTPS echo validates against our CA", func(t *testing.T) {
				h := fmt.Sprintf("%s.ingress.consul:%d", libservice.StaticServer2ServiceName, portWildcard)
				cl := httpClientWithCA(t, h, root.RootCertPEM)
				data := fmt.Sprintf("secret-%s", libservice.StaticClientServiceName)
				resp := mappedHTTPGET(t,
					"https://"+h,
					pm.Int(),
					nil,
					strings.NewReader(data),
					cl,
				)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Equal(t, []byte(data), body)
			})
		})
	}
	t.Run("pre-upgrade", func(t *testing.T) {
		tests(t)
	})

	if t.Failed() {
		t.Fatal("failing fast: failed assertions pre-upgrade")
	}

	// Upgrade the cluster to utils.utils.TargetVersion
	t.Logf("Upgrade to version %s", utils.TargetVersion)
	err = cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), utils.TargetVersion)
	require.NoError(t, err)
	require.NoError(t, igw.Restart())

	t.Run("post-upgrade", func(t *testing.T) {
		tests(t)
	})
}

// mappedHTTPGET performs an HTTP GET to the given uri, but actually uses
// "localhost:<mappedPort>" to connect the host, and sends the host from uri
// in the [http.Request.Host] field.
//
// Extra headers may be specified in header. body is the request body.
//
// client is used as the [http.Client], for example, one returned by
// [httpClientWithCA].
//
// It retries for up to 1 minute, with a 50ms wait.
func mappedHTTPGET(t *testing.T, uri string, mappedPort int, header http.Header, body io.Reader, client *http.Client) *http.Response {
	t.Helper()
	var hostHdr string
	u, _ := url.Parse(uri)
	hostHdr = u.Host
	u.Host = fmt.Sprintf("localhost:%d", mappedPort)
	uri = u.String()
	var resp *http.Response
	retry.RunWith(&retry.Timer{Timeout: 1 * time.Minute, Wait: 50 * time.Millisecond}, t, func(r *retry.R) {
		req, err := http.NewRequest("GET", uri, body)
		if header != nil {
			req.Header = header
		}
		if err != nil {
			r.Fatalf("could not make call to service %q: %s", uri, err)
		}
		if hostHdr != "" {
			req.Host = hostHdr
		}

		resp, err = client.Do(req)
		if err != nil {
			r.Fatalf("could not make call to service %q: %s", uri, err)
		}
	})
	return resp
}

// httpClientWithCA returns an [http.Client] configured to trust cacertPEM
// as a CA, and with reqHost set as the [http.Transport.TLSClientConfig.ServerName].
func httpClientWithCA(t *testing.T, reqHost string, cacertPEM string) *http.Client {
	t.Helper()
	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM([]byte(cacertPEM))
	require.True(t, ok)

	tr := http.Transport{
		DisableKeepAlives: true,
		// BUG: our *.ingress.consul certs have a SNI name of `*.ingress.consul.`. Note the trailing
		// dot. Go's [crypto/x509.Certificate.VerifyHostname] doesn't like the trailing dot, and
		// so won't evaluate the wildcard. As a workaround, we disable Go's builtin verification and do it
		// ourselves
		// https://groups.google.com/g/golang-checkins/c/K510gi92v8M explains the rationale for not
		// treating names with trailing dots as hostnames
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			RootCAs:            pool,
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				require.Equal(t, 1, len(rawCerts), "expected 1 cert")
				cert, err := x509.ParseCertificate(rawCerts[0])
				require.NoError(t, err)
				for i, s := range cert.DNSNames {
					cert.DNSNames[i] = strings.TrimSuffix(s, ".")
				}
				_, err = cert.Verify(x509.VerifyOptions{Roots: pool})
				require.NoError(t, err, "cert validation")
				return nil
			},
		},
	}
	reqHostNoPort, _, _ := strings.Cut(reqHost, ":")
	if reqHost != "" {
		tr.TLSClientConfig.ServerName = reqHostNoPort
	}
	client := http.Client{
		Transport: &tr,
	}
	return &client
}
