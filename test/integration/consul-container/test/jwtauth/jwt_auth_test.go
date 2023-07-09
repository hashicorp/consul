// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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

// TestJWTAuthConnectService summary
// This test ensures that when we have an intention referencing a JWT, requests
// without JWT authorization headers are denied. And requests with the correct JWT
// Authorization header are successful
//
// Steps:
// - Creates a single agent cluster
// - Creates a static-server and sidecar containers
// - Registers the created static-server and sidecar with consul
// - Create a static-client and sidecar containers
// - Registers the static-client and sidecar with consul
// - Ensure client sidecar is running as expected
// - Make a request without the JWT Authorization header and expects 401 StatusUnauthorized
// - Make a request with the JWT Authorization header and expects a 200
func TestJWTAuthConnectService(t *testing.T) {
	t.Parallel()

	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers:                1,
		NumClients:                1,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
		},
	})

	clientService := createServices(t, cluster)
	_, clientPort := clientService.GetAddr()
	_, clientAdminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, clientAdminPort, "static-server.default", "HEALTHY", 1)
	libassert.AssertContainerState(t, clientService, "running")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", clientPort), "static-server", "")

	claims := jwt.Claims{
		Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
		Audience:  jwt.Audience{"https://consul.test"},
		Issuer:    "https://legit.issuer.internal/",
		NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(60 * time.Minute)),
	}

	jwks, jwt := makeJWKSAndJWT(t, claims)

	// configure proxy-defaults, jwt-provider and intention
	configureProxyDefaults(t, cluster)
	configureJWTProvider(t, cluster, jwks, claims)
	configureIntentions(t, cluster)

	baseURL := fmt.Sprintf("http://localhost:%d", clientPort)
	// fails without jwt headers
	doRequest(t, baseURL, http.StatusUnauthorized, "")
	// succeeds with jwt
	doRequest(t, baseURL, http.StatusOK, jwt)
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

	// Create a service and proxy instance
	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy", nil)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", nil)

	return clientConnectProxy
}

// creates a JWKS and JWT that will be used for validation
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

// configures the protocol to http as this is needed for jwt-auth
func configureProxyDefaults(t *testing.T, cluster *libcluster.Cluster) {
	client := cluster.Agents[0].GetClient()

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

// creates a JWT local provider
func configureJWTProvider(t *testing.T, cluster *libcluster.Cluster, jwks string, claims jwt.Claims) {
	client := cluster.Agents[0].GetClient()

	ok, _, err := client.ConfigEntries().Set(&api.JWTProviderConfigEntry{
		Kind: api.JWTProvider,
		Name: "test-jwt",
		JSONWebKeySet: &api.JSONWebKeySet{
			Local: &api.LocalJWKS{
				JWKS: base64.StdEncoding.EncodeToString([]byte(jwks)),
			},
		},
		Issuer:    claims.Issuer,
		Audiences: claims.Audience,
	}, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

// creates an intention referencing the jwt provider
func configureIntentions(t *testing.T, cluster *libcluster.Cluster) {
	client := cluster.Agents[0].GetClient()

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
	}, nil)
	require.NoError(t, err)
	require.True(t, ok)
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
