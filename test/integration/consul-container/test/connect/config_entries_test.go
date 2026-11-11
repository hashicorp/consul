// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

// selfSignedCert returns a PEM encoded certificate and private key.
func selfSignedCert(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "example.com"},
		DNSNames:     []string{"example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM
}

// configEntryCases returns one valid entry per supported kind, plus a function
// that produces a changed copy used to exercise updates.
type configEntryCase struct {
	entry  api.ConfigEntry
	modify func(api.ConfigEntry)
	// check asserts a field set by modify is present on the entry read back.
	check func(t *testing.T, got api.ConfigEntry)
	// setup writes prerequisite entries (e.g. service protocol) before the case runs.
	setup func(t *testing.T, c *api.Client)
}

// httpServices returns a setup step marking the services as HTTP, which routers
// and splitters require.
func httpServices(names ...string) func(*testing.T, *api.Client) {
	return func(t *testing.T, c *api.Client) {
		for _, name := range names {
			setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: name, Protocol: "http"})
		}
	}
}

func configEntryCases(t *testing.T) []configEntryCase {
	certPEM, keyPEM := selfSignedCert(t)
	jwks := base64.StdEncoding.EncodeToString([]byte(`{"keys":[{"kty":"oct","kid":"k1","k":"c2VjcmV0"}]}`))

	return []configEntryCase{
		{
			entry:  &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "cfg-svc", Protocol: "tcp"},
			modify: func(e api.ConfigEntry) { e.(*api.ServiceConfigEntry).Protocol = "http" },
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Equal(t, "http", got.(*api.ServiceConfigEntry).Protocol)
			},
		},
		{
			entry:  &api.ProxyConfigEntry{Kind: api.ProxyDefaults, Name: api.ProxyConfigGlobal, Config: map[string]any{"protocol": "tcp"}},
			modify: func(e api.ConfigEntry) { e.(*api.ProxyConfigEntry).Config["protocol"] = "http" },
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Equal(t, "http", got.(*api.ProxyConfigEntry).Config["protocol"])
			},
		},
		{
			entry: &api.ServiceResolverConfigEntry{
				Kind:           api.ServiceResolver,
				Name:           "cfg-resolver",
				ConnectTimeout: 5 * time.Second,
			},
			modify: func(e api.ConfigEntry) { e.(*api.ServiceResolverConfigEntry).ConnectTimeout = 9 * time.Second },
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Equal(t, 9*time.Second, got.(*api.ServiceResolverConfigEntry).ConnectTimeout)
			},
		},
		{
			setup: httpServices("cfg-splitter", "cfg-splitter-canary"),
			entry: &api.ServiceSplitterConfigEntry{
				Kind: api.ServiceSplitter,
				Name: "cfg-splitter",
				Splits: []api.ServiceSplit{
					{Weight: 90, Service: "cfg-splitter"},
					{Weight: 10, Service: "cfg-splitter-canary"},
				},
			},
			modify: func(e api.ConfigEntry) {
				s := e.(*api.ServiceSplitterConfigEntry)
				s.Splits[0].Weight, s.Splits[1].Weight = 50, 50
			},
			check: func(t *testing.T, got api.ConfigEntry) {
				require.EqualValues(t, 50, got.(*api.ServiceSplitterConfigEntry).Splits[1].Weight)
			},
		},
		{
			setup: httpServices("cfg-router", "cfg-router-v1", "cfg-router-v2"),
			entry: &api.ServiceRouterConfigEntry{
				Kind: api.ServiceRouter,
				Name: "cfg-router",
				Routes: []api.ServiceRoute{{
					Match:       &api.ServiceRouteMatch{HTTP: &api.ServiceRouteHTTPMatch{PathPrefix: "/v1"}},
					Destination: &api.ServiceRouteDestination{Service: "cfg-router-v1"},
				}},
			},
			modify: func(e api.ConfigEntry) {
				e.(*api.ServiceRouterConfigEntry).Routes[0].Destination.Service = "cfg-router-v2"
			},
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Equal(t, "cfg-router-v2", got.(*api.ServiceRouterConfigEntry).Routes[0].Destination.Service)
			},
		},
		{
			entry: &api.ServiceIntentionsConfigEntry{
				Kind:    api.ServiceIntentions,
				Name:    "cfg-dest",
				Sources: []*api.SourceIntention{{Name: "cfg-src", Action: api.IntentionActionAllow}},
			},
			modify: func(e api.ConfigEntry) {
				e.(*api.ServiceIntentionsConfigEntry).Sources[0].Action = api.IntentionActionDeny
			},
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Equal(t, api.IntentionActionDeny, got.(*api.ServiceIntentionsConfigEntry).Sources[0].Action)
			},
		},
		{
			entry: &api.IngressGatewayConfigEntry{
				Kind: api.IngressGateway,
				Name: "cfg-ingress",
				Listeners: []api.IngressListener{{
					Port: 8080, Protocol: "tcp", Services: []api.IngressService{{Name: "cfg-ingress-svc"}},
				}},
			},
			modify: func(e api.ConfigEntry) { e.(*api.IngressGatewayConfigEntry).Listeners[0].Port = 8081 },
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Equal(t, 8081, got.(*api.IngressGatewayConfigEntry).Listeners[0].Port)
			},
		},
		{
			entry: &api.TerminatingGatewayConfigEntry{
				Kind:     api.TerminatingGateway,
				Name:     "cfg-terminating",
				Services: []api.LinkedService{{Name: "legacy-db"}},
			},
			modify: func(e api.ConfigEntry) {
				e.(*api.TerminatingGatewayConfigEntry).Services = append(e.(*api.TerminatingGatewayConfigEntry).Services, api.LinkedService{Name: "legacy-cache"})
			},
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Len(t, got.(*api.TerminatingGatewayConfigEntry).Services, 2)
			},
		},
		{
			entry:  &api.MeshConfigEntry{TransparentProxy: api.TransparentProxyMeshConfig{MeshDestinationsOnly: false}},
			modify: func(e api.ConfigEntry) { e.(*api.MeshConfigEntry).TransparentProxy.MeshDestinationsOnly = true },
			check: func(t *testing.T, got api.ConfigEntry) {
				require.True(t, got.(*api.MeshConfigEntry).TransparentProxy.MeshDestinationsOnly)
			},
		},
		{
			entry: &api.JWTProviderConfigEntry{
				Kind:          api.JWTProvider,
				Name:          "cfg-jwt",
				Issuer:        "https://issuer.example.com",
				JSONWebKeySet: &api.JSONWebKeySet{Local: &api.LocalJWKS{JWKS: jwks}},
			},
			modify: func(e api.ConfigEntry) { e.(*api.JWTProviderConfigEntry).Audiences = []string{"api"} },
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Equal(t, []string{"api"}, got.(*api.JWTProviderConfigEntry).Audiences)
			},
		},
		{
			entry: &api.InlineCertificateConfigEntry{
				Kind: api.InlineCertificate, Name: "cfg-cert", Certificate: certPEM, PrivateKey: keyPEM,
			},
			modify: func(e api.ConfigEntry) {
				e.(*api.InlineCertificateConfigEntry).Meta = map[string]string{"owner": "team-a"}
			},
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Equal(t, "team-a", got.(*api.InlineCertificateConfigEntry).Meta["owner"])
			},
		},
		{
			entry: &api.APIGatewayConfigEntry{
				Kind: api.APIGateway,
				Name: "cfg-api-gateway",
				Listeners: []api.APIGatewayListener{{
					Name: "http", Port: 9090, Protocol: "http",
				}},
			},
			modify: func(e api.ConfigEntry) { e.(*api.APIGatewayConfigEntry).Listeners[0].Port = 9091 },
			check: func(t *testing.T, got api.ConfigEntry) {
				require.Equal(t, 9091, got.(*api.APIGatewayConfigEntry).Listeners[0].Port)
			},
		},
	}
}

// TestConfigEntries_CRUDAndCAS exercises create, read, list, update, check-and-set
// and delete for each supported config entry kind through the HTTP API.
func TestConfigEntries_CRUDAndCAS(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	c := e.server

	for _, tc := range configEntryCases(t) {
		tc := tc
		kind, name := tc.entry.GetKind(), tc.entry.GetName()

		t.Run(kind, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t, c)
			}
			entries := c.ConfigEntries()

			ok, _, err := entries.Set(tc.entry, nil)
			require.NoError(t, err)
			require.True(t, ok)

			got, _, err := entries.Get(kind, name, nil)
			require.NoError(t, err)
			require.Equal(t, name, got.GetName())
			require.NotZero(t, got.GetCreateIndex())

			listed, _, err := entries.List(kind, nil)
			require.NoError(t, err)
			var names []string
			for _, l := range listed {
				names = append(names, l.GetName())
			}
			require.Contains(t, names, name)

			// Update in place.
			tc.modify(got)
			ok, _, err = entries.Set(got, nil)
			require.NoError(t, err)
			require.True(t, ok)

			updated, _, err := entries.Get(kind, name, nil)
			require.NoError(t, err)
			tc.check(t, updated)
			require.Greater(t, updated.GetModifyIndex(), got.GetCreateIndex())

			// CAS with a stale index must be refused and leave the entry untouched.
			// The CAS body differs from the stored entry (it is the pre-update
			// version) because servers skip writes that would not change anything.
			ok, _, err = entries.CAS(tc.entry, updated.GetModifyIndex()-1, nil)
			require.NoError(t, err)
			require.False(t, ok, "CAS with a stale index must not apply")
			unchanged, _, err := entries.Get(kind, name, nil)
			require.NoError(t, err)
			require.Equal(t, updated.GetModifyIndex(), unchanged.GetModifyIndex())

			// CAS with the current index succeeds.
			ok, _, err = entries.CAS(tc.entry, updated.GetModifyIndex(), nil)
			require.NoError(t, err)
			require.True(t, ok)

			current, _, err := entries.Get(kind, name, nil)
			require.NoError(t, err)
			require.Greater(t, current.GetModifyIndex(), updated.GetModifyIndex())

			// Delete with a stale index is refused; with the current index it succeeds.
			ok, _, err = entries.DeleteCAS(kind, name, updated.GetModifyIndex(), nil)
			require.NoError(t, err)
			require.False(t, ok, "DeleteCAS with a stale index must not delete")

			ok, _, err = entries.DeleteCAS(kind, name, current.GetModifyIndex(), nil)
			require.NoError(t, err)
			require.True(t, ok)

			_, _, err = entries.Get(kind, name, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "404")
		})
	}
}

// TestConfigEntries_Validation verifies that servers reject invalid entries
// instead of storing them.
func TestConfigEntries_Validation(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	entries := e.server.ConfigEntries()

	for name, tc := range map[string]struct {
		entry   api.ConfigEntry
		wantErr string
	}{
		"service-defaults with an unknown protocol": {
			entry:   &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "bad-protocol", Protocol: "carrier-pigeon"},
			wantErr: "protocol",
		},
		"proxy-defaults must be named global": {
			entry:   &api.ProxyConfigEntry{Kind: api.ProxyDefaults, Name: "not-global"},
			wantErr: "global",
		},
		"splitter weights must sum to 100": {
			entry: &api.ServiceSplitterConfigEntry{
				Kind: api.ServiceSplitter, Name: "bad-split",
				Splits: []api.ServiceSplit{{Weight: 60, Service: "a"}, {Weight: 20, Service: "b"}},
			},
			wantErr: "100",
		},
		"tcp listener cannot front multiple services": {
			entry: &api.IngressGatewayConfigEntry{
				Kind: api.IngressGateway, Name: "bad-ingress",
				Listeners: []api.IngressListener{{
					Port: 8080, Protocol: "tcp",
					Services: []api.IngressService{{Name: "a"}, {Name: "b"}},
				}},
			},
			wantErr: "Multiple services per listener",
		},
		"inline-certificate needs a valid key pair": {
			entry:   &api.InlineCertificateConfigEntry{Kind: api.InlineCertificate, Name: "bad-cert", Certificate: "nope", PrivateKey: "nope"},
			wantErr: "",
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ok, _, err := entries.Set(tc.entry, nil)
			require.Error(t, err)
			require.False(t, ok)
			require.Contains(t, err.Error(), tc.wantErr)

			_, _, err = entries.Get(tc.entry.GetKind(), tc.entry.GetName(), nil)
			require.Error(t, err, "rejected entry must not be stored")
		})
	}
}
