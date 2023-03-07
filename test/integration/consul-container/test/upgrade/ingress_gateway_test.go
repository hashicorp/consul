package upgrade

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests adapt BATS-based tests from test/integration/connect/case-ingress-gateway*

// TestIngressGateway_UpgradeToTarget_fromLatest:
// - starts a cluster with 2 static services,
// - configures an ingress gateway + router
// - performs tests to ensure our routing rules work (namely header manipulation)
// - upgrades the cluster
// - performs these tests again
func TestIngressGateway_UpgradeToTarget_fromLatest(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, oldVersion, targetVersion string) {
		// setup
		// TODO? we don't need a peering cluster, so maybe this is overkill
		cluster, _, client := topology.NewCluster(t, &topology.ClusterConfig{
			NumServers: 1,
			NumClients: 2,
			BuildOpts: &libcluster.BuildOptions{
				Datacenter:    "dc1",
				ConsulVersion: oldVersion,
				// TODO? InjectAutoEncryption: true,
			},
			ApplyDefaultProxySettings: true,
		})

		// upsert config entry making http default protocol for global
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

		// upsert config entry for `service-router` `router`:
		// - prefix matching `/$nameS1` goes to service s1
		// - prefix matching `/$nameS2` goes to service s2
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

		igw, err := libservice.NewGatewayService(context.Background(), nameIG, "ingress", cluster.Servers()[0])
		require.NoError(t, err)
		t.Logf("created gateway: %#v", igw)

		// upsert config entry for ingress-gateway ig1, protocol http, service s1
		// - listener points at service `router`
		// 	- add request headers: 1 new, 1 existing
		// 	- set request headers: 1 existing, 1 new, to client IP
		//  - add response headers: 1 new, 1 existing
		//  - set response headers: 1 existing
		//  - remove response header: 1 existing

		// this must be one of the externally-mapped ports from
		// https://github.com/hashicorp/consul/blob/c5e729e86576771c4c22c6da1e57aaa377319323/test/integration/consul-container/libs/cluster/container.go#L521-L525
		const portRouter = 8080
		require.NoError(t, cluster.ConfigEntryWrite(&api.IngressGatewayConfigEntry{
			Kind: api.IngressGateway,
			Name: nameIG,
			Listeners: []api.IngressListener{
				{
					Port:     portRouter,
					Protocol: "http",
					Services: []api.IngressService{
						{
							Name: nameRouter,
							// TODO: extract these header values to consts to test
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

		// tests
		tests := func(t *testing.T) {
			// fortio name should be $nameS<X> for /$nameS<X> prefix on router
			portRouterMapped, _ := cluster.Servers()[0].GetPod().MappedPort(
				context.Background(),
				nat.Port(fmt.Sprintf("%d/tcp", portRouter)),
			)
			reqHost := fmt.Sprintf("router.ingress.consul:%d", portRouter)
			libassert.AssertFortioName(t,
				fmt.Sprintf("http://localhost:%d/%s", portRouterMapped.Int(), nameS1), nameS1, reqHost)
			libassert.AssertFortioName(t,
				fmt.Sprintf("http://localhost:%d/%s", portRouterMapped.Int(), nameS2), nameS2, reqHost)
			urlbaseS2 := fmt.Sprintf("http://%s/%s", reqHost, nameS2)

			t.Run("request header manipulation", func(t *testing.T) {
				resp := mappedHTTPGET(t, fmt.Sprintf("%s/debug?env=dump", urlbaseS2), portRouterMapped.Int(), http.Header(map[string][]string{
					"X-Existing-1": {"original"},
					"X-Existing-2": {"original"},
					"X-Bad-Req":    {"true"},
				}))
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
				// TODO: This 172. is the prefix of the IP for the gateway for our docker network.
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
		}
		t.Run(fmt.Sprintf("pre-upgrade from %s to %s", oldVersion, targetVersion), func(t *testing.T) {
			tests(t)
		})

		if t.Failed() {
			t.Fatal("failing fast: failed assertions pre-upgrade")
		}

		// Upgrade the cluster to targetVersion
		t.Logf("Upgrade to version %s", targetVersion)
		err = cluster.StandardUpgrade(t, context.Background(), targetVersion)
		require.NoError(t, err)
		require.NoError(t, igw.Restart())

		t.Run(fmt.Sprintf("post-upgrade from %s to %s", oldVersion, targetVersion), func(t *testing.T) {
			tests(t)
		})
	}

	for _, oldVersion := range UpgradeFromVersions {
		// copy to avoid lint loopclosure
		oldVersion := oldVersion

		t.Run(fmt.Sprintf("Upgrade from %s to %s", oldVersion, utils.TargetVersion),
			func(t *testing.T) {
				t.Parallel()
				run(t, oldVersion, utils.TargetVersion)
			})
		time.Sleep(1 * time.Second)
	}
}

func mappedHTTPGET(t *testing.T, uri string, mappedPort int, header http.Header) *http.Response {
	t.Helper()
	var hostHdr string
	u, _ := url.Parse(uri)
	hostHdr = u.Host
	u.Host = fmt.Sprintf("localhost:%d", mappedPort)
	uri = u.String()
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	var resp *http.Response
	retry.RunWith(&retry.Timer{Timeout: 1 * time.Minute, Wait: 50 * time.Millisecond}, t, func(r *retry.R) {
		req, err := http.NewRequest("GET", uri, nil)
		if header != nil {
			req.Header = header
		}
		if err != nil {
			r.Fatal("could not make request to service ", uri)
		}
		if hostHdr != "" {
			req.Host = hostHdr
		}

		resp, err = client.Do(req)
		if err != nil {
			r.Fatal("could not make call to service ", uri)
		}
	})
	return resp
}
