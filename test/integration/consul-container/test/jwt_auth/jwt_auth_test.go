// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jwt_auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2/jwt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

func TestJWTAuthConnectService(t *testing.T) {
	t.Parallel()

	clusterConfig := &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			AllowHTTPAnyway:        true,
		},
	}

	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)
	clientService, serverService := createServices(t, cluster)

	_, clientPort := clientService.GetAddr()
	_, clientAdminPort := clientService.GetAdminAddr()

	// _, serverPort := serverService.GetAddr()
	_, serverAdminPort := serverService.GetAdminAddr()

	claims := jwt.Claims{
		Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
		Audience:  jwt.Audience{"https://consul.test"},
		Issuer:    "https://legit.issuer.internal/",
		NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(60 * time.Minute)),
	}

	jwks, jwt := makeJWKSAndJWT(t, claims)
	t.Logf("jwks = %v", jwks)
	t.Logf("jwt = %v", jwt)

	// Configure the JWT provider
	configureProxyDefaults(t, cluster)
	configureJWTProvider(t, cluster, jwks, claims)
	configureIntentions(t, cluster)

	// TODO: Have the client send the request with a JWT
	libassert.AssertUpstreamEndpointStatus(t, clientAdminPort, "static-server.default", "HEALTHY", 1)
	logEnvoyConfigDump(t, serverAdminPort)

	libassert.AssertContainerState(t, clientService, "running")

	headers := map[string][]string{
		"Authorization ": {fmt.Sprintf("Bearer %s", jwt)},
	}
	doHTTPServiceEchoes(t, "localhost", clientPort, headers)

	// // TODO: Hack
	client := &http.Client{
		Transport: &TransportWithHeaders{
			RoundTripper: cleanhttp.DefaultTransport(),
			Headers:      headers,
		},
		Timeout: 5 * time.Second,
	}
	libassert.AssertFortioNameWithClient(t, fmt.Sprintf("http://localhost:%d", clientPort), "static-server", "", client)
}

type TransportWithHeaders struct {
	http.RoundTripper

	Headers http.Header
}

func createServices(t *testing.T, cluster *libcluster.Cluster) (libservice.Service, libservice.Service) {
	node := cluster.Agents[0]
	client := node.GetClient()
	// Create a service and proxy instance
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}

	// Create a service and proxy instance
	_, serverService, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy", nil)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", nil)

	return clientConnectProxy, serverService
}

func configureProxyDefaults(t *testing.T, cluster *libcluster.Cluster) {
	node := cluster.Agents[0]
	client := node.GetClient()

	ok, _, err := client.ConfigEntries().Set(&api.ProxyConfigEntry{
		Kind: api.ProxyDefaults,
		Name: api.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

func configureJWTProvider(t *testing.T, cluster *libcluster.Cluster, jwks string, claims jwt.Claims) {
	node := cluster.Agents[0]
	client := node.GetClient()

	jwksB64 := base64.StdEncoding.EncodeToString([]byte(jwks))

	ok, _, err := client.ConfigEntries().Set(&api.JWTProviderConfigEntry{
		Kind: api.JWTProvider,
		Name: "test-jwt",
		JSONWebKeySet: &api.JSONWebKeySet{
			Local: &api.LocalJWKS{
				JWKS: jwksB64,
			},
		},
		Issuer:    claims.Issuer,
		Audiences: claims.Audience,
		//Locations:        []*api.JWTLocation{},
		//Forwarding:       &api.JWTForwardingConfig{},
		//ClockSkewSeconds: 0,
		//CacheConfig:      &api.JWTCacheConfig{},
		//Meta:             map[string]string{},
		//CreateIndex:      0,
		//ModifyIndex:      0,
		//Partition:        "",
		//Namespace:        "",
	}, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

func configureIntentions(t *testing.T, cluster *libcluster.Cluster) {
	node := cluster.Agents[0]
	client := node.GetClient()

	ok, _, err := client.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
		Kind: "service-intentions",
		Name: libservice.StaticServerServiceName,
		Sources: []*api.SourceIntention{
			{
				Name:   libservice.StaticClientServiceName,
				Action: api.IntentionActionAllow,
			},
		},
		JWT: &api.IntentionJWTRequirement{
			Providers: []*api.IntentionJWTProvider{
				{
					Name:         "test-jwt",
					VerifyClaims: []*api.IntentionJWTClaimVerification{},
				},
			},
		},
		Meta:        map[string]string{},
		CreateIndex: 0,
		ModifyIndex: 0,
	}, nil)
	require.NoError(t, err)
	require.True(t, ok)

	entries, _, err := client.ConfigEntries().List("service-intentions", nil)
	require.NoError(t, err)
	t.Logf("intentions list:")
	for i, e := range entries {
		intention := e.(*api.ServiceIntentionsConfigEntry)
		intentionJson, err := json.Marshal(intention)
		require.NoError(t, err)
		t.Logf("%d: %s", i, string(intentionJson))
	}
}

func makeJWKSAndJWT(t *testing.T, claims jwt.Claims) (string, string) {
	pub, priv, err := libutils.GenerateKey()
	require.NoError(t, err)

	jwks, err := libutils.NewJWKS(pub)
	require.NoError(t, err)

	jwksJson, err := json.Marshal(jwks)
	require.NoError(t, err)

	type orgs struct {
		Primary string `json:"primary"`
	}
	privateCl := struct {
		FirstName string   `json:"first_name"`
		Org       orgs     `json:"org"`
		Groups    []string `json:"groups"`
	}{
		FirstName: "jeff2",
		Org:       orgs{"engineering"},
		Groups:    []string{"foo", "bar"},
	}

	jwt, err := libutils.SignJWT(priv, claims, privateCl)
	require.NoError(t, err)
	return string(jwksJson), jwt
}

func logEnvoyConfigDump(t *testing.T, adminPort int) {
	var (
		dump string
		err  error
	)
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 1 * time.Second}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		dump, _, err = libassert.GetEnvoyOutput(adminPort, "config_dump", map[string]string{})
		if err != nil {
			r.Fatal("could not fetch envoy configuration")
		}
	})

	t.Logf("config_dump = %s", dump)
}

// HTTPServiceEchoes verifies that a post to the given ip/port combination returns the data
// in the response body. Optional path can be provided to differentiate requests.
func doHTTPServiceEchoes(t *testing.T, ip string, port int, headers map[string][]string) {
	const phrase = "hello"

	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 120 * time.Second, Wait: 500 * time.Millisecond}
	}

	client := cleanhttp.DefaultClient()
	url := fmt.Sprintf("http://%s:%d", ip, port)

	retry.RunWith(failer(), t, func(r *retry.R) {
		t.Logf("making call to %s", url)
		reader := strings.NewReader(phrase)

		req, err := http.NewRequest("POST", url, reader)
		require.NoError(r, err)

		req.Header = headers

		res, err := client.Do(req)
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
