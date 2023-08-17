// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package jwtauth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"

	"github.com/go-jose/go-jose/v3/jwt"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/go-cleanhttp"
	"testing"
	"time"
)

// TestJWTAuthConnectService summary:
// This test ensures that when we have an intention referencing a JWT, requests
// without JWT authorization headers are denied and requests with the correct JWT
// Authorization header are successful.
//
// Steps:
// - Creates a single agent cluster
// - Generates a JWKS and 2 JWTs with different claims
// - Generates another JWKS with a single JWT
// - Configures proxy defaults, providers and intentions
// - Creates a static-server and sidecar containers
// - Registers the created static-server and sidecar with consul
// - Create a static-client and sidecar containers
// - Registers the static-client and sidecar with consul
// - Ensure client sidecar is running as expected
// - Runs a couple of scenarios to ensure jwt validation works as expected
func TestJWTAuthConnectService(t *testing.T) {
	t.Parallel()

	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers:                1,
		NumClients:                1,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectCerts:            true,
			InjectGossipEncryption: false,
			AllowHTTPAnyway:        true,
			ACLEnabled:             true,
		},
	})

	// generate jwks and 2 jwts with different claims for provider 1
	jwksOne, privOne := makeJWKS(t)
	claimsOne := makeTestClaims("https://legit.issuer.internal/", "https://consul.test")
	jwtOne := makeJWT(t, privOne, claimsOne, testClaimPayload{UserType: "admin", FirstName: "admin"})
	jwtOneAdmin := makeJWT(t, privOne, claimsOne, testClaimPayload{UserType: "client", FirstName: "non-admin"})
	provider1 := makeTestJWTProvider("okta", jwksOne, claimsOne)

	// generate another jwks and jwt for provider 2
	jwksTwo, privTwo := makeJWKS(t)
	claimsTwo := makeTestClaims("https://another.issuer.internal/", "https://consul.test")
	jwtTwo := makeJWT(t, privTwo, claimsTwo, testClaimPayload{})
	provider2 := makeTestJWTProvider("auth0", jwksTwo, claimsTwo)

	// configure proxy-defaults, jwt providers and intentions
	configureProxyDefaults(t, cluster)
	configureJWTProviders(t, cluster, provider1, provider2)
	configureIntentions(t, cluster, provider1, provider2)

	clientService := createServices(t, cluster)
	_, clientPort := clientService.GetAddr()
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertContainerState(t, clientService, "running")
	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)

	// request to restricted endpoint with no jwt should be denied
	doRequest(t, fmt.Sprintf("http://localhost:%d/restricted/foo", clientPort), http.StatusForbidden, "")

	// request with jwt 1 /restricted/foo should be disallowed
	doRequest(t, fmt.Sprintf("http://localhost:%d/restricted/foo", clientPort), http.StatusForbidden, jwtOne)

	// request with jwt 1 /other/foo should be allowed
	libassert.HTTPServiceEchoesWithHeaders(t, "localhost", clientPort, "other/foo", makeAuthHeaders(jwtOne))

	// request with jwt 1 /other/foo with mismatched claims should be disallowed
	doRequest(t, fmt.Sprintf("http://localhost:%d/other/foo", clientPort), http.StatusForbidden, jwtOneAdmin)

	// request with provider 1 /foo should be allowed
	libassert.HTTPServiceEchoesWithHeaders(t, "localhost", clientPort, "foo", makeAuthHeaders(jwtOne))

	// request with jwt 2 to /foo should be denied
	doRequest(t, fmt.Sprintf("http://localhost:%d/foo", clientPort), http.StatusForbidden, jwtTwo)

	// request with jwt 2 to /restricted/foo should be allowed
	libassert.HTTPServiceEchoesWithHeaders(t, "localhost", clientPort, "restricted/foo", makeAuthHeaders(jwtTwo))

	// request with jwt 2 to /other/foo should be denied
	doRequest(t, fmt.Sprintf("http://localhost:%d/other/foo", clientPort), http.StatusForbidden, jwtTwo)
}

func makeAuthHeaders(jwt string) map[string]string {
	return map[string]string{"Authorization": fmt.Sprintf("Bearer %s", jwt)}
}

func createServices(t *testing.T, cluster *libcluster.Cluster) libservice.Service {
	node := cluster.Agents[0]
	client := node.GetClient()
	// Create a service and proxy instance
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}
	apiOpts := &api.QueryOptions{Token: cluster.TokenBootstrap}

	// Create a service and proxy instance
	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy", apiOpts)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, apiOpts)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", apiOpts)

	return clientConnectProxy
}

func makeJWKS(t *testing.T) (string, string) {
	pub, priv, err := libutils.GenerateKey()
	require.NoError(t, err)

	jwks, err := libutils.NewJWKS(pub)
	require.NoError(t, err)

	jwksJson, err := json.Marshal(jwks)
	require.NoError(t, err)

	return string(jwksJson), priv
}

type testClaimPayload struct {
	UserType  string
	FirstName string
}

func makeJWT(t *testing.T, priv string, claims jwt.Claims, payload testClaimPayload) string {
	jwt, err := libutils.SignJWT(priv, claims, payload)
	require.NoError(t, err)

	return jwt
}

// configures the protocol to http as this is needed for jwt-auth
func configureProxyDefaults(t *testing.T, cluster *libcluster.Cluster) {
	require.NoError(t, cluster.ConfigEntryWrite(&api.ProxyConfigEntry{
		Kind: api.ProxyDefaults,
		Name: api.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))
}

func makeTestJWTProvider(name string, jwks string, claims jwt.Claims) *api.JWTProviderConfigEntry {
	return &api.JWTProviderConfigEntry{
		Kind: api.JWTProvider,
		Name: name,
		JSONWebKeySet: &api.JSONWebKeySet{
			Local: &api.LocalJWKS{
				JWKS: base64.StdEncoding.EncodeToString([]byte(jwks)),
			},
		},
		Issuer:    claims.Issuer,
		Audiences: claims.Audience,
	}
}

// creates a JWT local provider
func configureJWTProviders(t *testing.T, cluster *libcluster.Cluster, providers ...*api.JWTProviderConfigEntry) {
	for _, prov := range providers {
		require.NoError(t, cluster.ConfigEntryWrite(prov))
	}
}

// creates an intention referencing the jwt provider
func configureIntentions(t *testing.T, cluster *libcluster.Cluster, provider1, provider2 *api.JWTProviderConfigEntry) {
	intention := api.ServiceIntentionsConfigEntry{
		Kind: "service-intentions",
		Name: libservice.StaticServerServiceName,
		Sources: []*api.SourceIntention{
			{
				Name: libservice.StaticClientServiceName,
				Permissions: []*api.IntentionPermission{
					{
						Action: api.IntentionActionAllow,
						HTTP: &api.IntentionHTTPPermission{
							PathPrefix: "/restricted/",
						},
						JWT: &api.IntentionJWTRequirement{
							Providers: []*api.IntentionJWTProvider{
								{
									Name: provider2.Name,
								},
							},
						},
					},
					{
						Action: api.IntentionActionAllow,
						HTTP: &api.IntentionHTTPPermission{
							PathPrefix: "/",
						},
						JWT: &api.IntentionJWTRequirement{
							Providers: []*api.IntentionJWTProvider{
								{
									Name: provider1.Name,
									VerifyClaims: []*api.IntentionJWTClaimVerification{
										{
											Path:  []string{"UserType"},
											Value: "admin",
										},
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "other-client",
				Permissions: []*api.IntentionPermission{
					{
						Action: api.IntentionActionAllow,
						HTTP: &api.IntentionHTTPPermission{
							PathPrefix: "/other/",
						},
						JWT: &api.IntentionJWTRequirement{
							Providers: []*api.IntentionJWTProvider{
								{
									Name: provider2.Name,
								},
							},
						},
					},
				},
			},
		},
	}
	require.NoError(t, cluster.ConfigEntryWrite(&intention))
}

func doRequest(t *testing.T, url string, expStatus int, jwt string) {
	retry.RunWith(&retry.Timer{Timeout: 5 * time.Second, Wait: time.Second}, t, func(r *retry.R) {
		client := cleanhttp.DefaultClient()
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(r, err)
		if jwt != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))
		}
		resp, err := client.Do(req)
		require.NoError(r, err)
		require.Equal(r, expStatus, resp.StatusCode)
	})
}

func makeTestClaims(issuer, audience string) jwt.Claims {
	return jwt.Claims{
		Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
		Audience:  jwt.Audience{audience},
		Issuer:    issuer,
		NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(60 * time.Minute)),
	}
}
