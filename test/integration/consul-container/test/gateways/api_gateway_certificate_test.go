// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package gateways

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// newSelfSignedCert returns a PEM encoded certificate and private key valid for
// dnsName.
func newSelfSignedCert(t *testing.T, dnsName string) (certPEM, keyPEM string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: dnsName},
		DNSNames:     []string{dnsName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
}

func inlineCertificate(t *testing.T, name, dnsName string) *api.InlineCertificateConfigEntry {
	certPEM, keyPEM := newSelfSignedCert(t, dnsName)
	return &api.InlineCertificateConfigEntry{
		Kind:        api.InlineCertificate,
		Name:        name,
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	}
}

func tlsListener(name string, port int, certNames ...string) api.APIGatewayListener {
	var refs []api.ResourceReference
	for _, n := range certNames {
		refs = append(refs, api.ResourceReference{Kind: api.InlineCertificate, Name: n})
	}
	return api.APIGatewayListener{
		Name:     name,
		Port:     port,
		Protocol: "http",
		TLS:      api.APIGatewayTLSConfiguration{Certificates: refs},
	}
}

// TestAPIGateway_InlineCertificateLifecycle verifies how a TLS listener tracks
// the inline-certificate it references: the gateway is only accepted while the
// certificate exists, and replacing the certificate (rotation) keeps it
// accepted.
func TestAPIGateway_InlineCertificateLifecycle(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, false)
	c := e.server

	t.Run("a gateway referencing a missing certificate is not accepted", func(t *testing.T) {
		setEntry(t, c, apiGateway("tls-gw", tlsListener("https", 7443, "gw-cert")))

		requireCondition(t, c, api.APIGateway, "tls-gw", "Accepted", "", api.ConditionStatusFalse, "InvalidCertificates")
		requireCondition(t, c, api.APIGateway, "tls-gw", "ResolvedRefs", "https", api.ConditionStatusFalse, "InvalidCertificateRef")

		cond := findCondition(entryConditions(t, c, api.APIGateway, "tls-gw"), "ResolvedRefs", "https")
		require.Contains(t, cond.Message, `certificate "gw-cert" not found`)
	})

	t.Run("creating the certificate makes the gateway accepted", func(t *testing.T) {
		setEntry(t, c, inlineCertificate(t, "gw-cert", "one.example.com"))

		requireCondition(t, c, api.APIGateway, "tls-gw", "Accepted", "", api.ConditionStatusTrue, "Accepted")
		requireCondition(t, c, api.APIGateway, "tls-gw", "ResolvedRefs", "https", api.ConditionStatusTrue, "ResolvedRefs")
	})

	t.Run("rotating the certificate keeps the gateway accepted", func(t *testing.T) {
		before, _, err := c.ConfigEntries().Get(api.InlineCertificate, "gw-cert", nil)
		require.NoError(t, err)

		rotated := inlineCertificate(t, "gw-cert", "two.example.com")
		setEntry(t, c, rotated)

		after, _, err := c.ConfigEntries().Get(api.InlineCertificate, "gw-cert", nil)
		require.NoError(t, err)
		require.NotEqual(t, before.(*api.InlineCertificateConfigEntry).Certificate, after.(*api.InlineCertificateConfigEntry).Certificate)
		require.Greater(t, after.GetModifyIndex(), before.GetModifyIndex())

		requireCondition(t, c, api.APIGateway, "tls-gw", "Accepted", "", api.ConditionStatusTrue, "Accepted")
		requireCondition(t, c, api.APIGateway, "tls-gw", "ResolvedRefs", "https", api.ConditionStatusTrue, "ResolvedRefs")
	})

	t.Run("one certificate can serve several gateways", func(t *testing.T) {
		setEntry(t, c, apiGateway("tls-gw-2", tlsListener("https", 7444, "gw-cert")))

		requireCondition(t, c, api.APIGateway, "tls-gw-2", "Accepted", "", api.ConditionStatusTrue, "Accepted")
	})

	t.Run("deleting the certificate breaks every gateway that uses it", func(t *testing.T) {
		_, err := c.ConfigEntries().Delete(api.InlineCertificate, "gw-cert", nil)
		require.NoError(t, err)

		for _, gw := range []string{"tls-gw", "tls-gw-2"} {
			requireCondition(t, c, api.APIGateway, gw, "Accepted", "", api.ConditionStatusFalse, "InvalidCertificates")
		}
	})

	t.Run("restoring the certificate repairs them", func(t *testing.T) {
		setEntry(t, c, inlineCertificate(t, "gw-cert", "three.example.com"))

		for _, gw := range []string{"tls-gw", "tls-gw-2"} {
			requireCondition(t, c, api.APIGateway, gw, "Accepted", "", api.ConditionStatusTrue, "Accepted")
		}
	})

	t.Run("an unusable certificate is rejected when written", func(t *testing.T) {
		certPEM, _ := newSelfSignedCert(t, "mismatch.example.com")
		_, otherKey := newSelfSignedCert(t, "other.example.com")

		for name, entry := range map[string]*api.InlineCertificateConfigEntry{
			"not pem":        {Kind: api.InlineCertificate, Name: "junk", Certificate: "nope", PrivateKey: "nope"},
			"mismatched key": {Kind: api.InlineCertificate, Name: "mismatch", Certificate: certPEM, PrivateKey: otherKey},
		} {
			ok, _, err := c.ConfigEntries().Set(entry, nil)
			require.Error(t, err, name)
			require.False(t, ok, name)
		}
	})
}

// peerCertificateCommonName completes a TLS handshake with addr and returns
// the common name of the certificate the server presented. Verification is
// skipped on purpose: the certificates are self-signed and only the identity
// that was served matters here.
func peerCertificateCommonName(addr, serverName string) (string, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 5 * time.Second},
		Config:    &tls.Config{InsecureSkipVerify: true, ServerName: serverName},
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", net.UnknownNetworkError("no peer certificate presented")
	}
	return certs[0].Subject.CommonName, nil
}

// TestAPIGateway_InlineCertificateRotationIsServed verifies, with a real Envoy
// API gateway, that the certificate presented to clients on a TLS listener is
// the one in the inline-certificate entry and that it changes when the entry is
// rotated, without restarting the gateway.
func TestAPIGateway_InlineCertificateRotationIsServed(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	gatewayName := randomName("gateway", 16)
	routeName := randomName("route", 16)
	certName := randomName("cert", 16)
	serviceName := randomName("service", 16)
	listenerPort := 6000
	serviceHTTPPort := 6001
	serviceGRPCPort := 6002

	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			AllowHTTPAnyway:        true,
		},
		ExposedPorts: []int{listenerPort, serviceHTTPPort, serviceGRPCPort},
	})
	client := cluster.APIClient(0)
	namespace := getOrCreateNamespace(t, client)

	gateway, err := libservice.NewGatewayService(context.Background(), libservice.GatewayConfig{
		Kind:      "api",
		Namespace: namespace,
		Name:      gatewayName,
	}, cluster.Agents[0], listenerPort)
	require.NoError(t, err)
	libassert.CatalogServiceIsHealthy(t, client, gatewayName, &api.QueryOptions{Namespace: namespace})

	cert := inlineCertificate(t, certName, "one.example.com")
	cert.Namespace = namespace
	require.NoError(t, cluster.ConfigEntryWrite(cert))

	require.NoError(t, cluster.ConfigEntryWrite(&api.APIGatewayConfigEntry{
		Kind:      api.APIGateway,
		Namespace: namespace,
		Name:      gatewayName,
		Listeners: []api.APIGatewayListener{{
			Name:     "listener",
			Port:     listenerPort,
			Protocol: "tcp",
			TLS: api.APIGatewayTLSConfiguration{
				Certificates: []api.ResourceReference{{Kind: api.InlineCertificate, Name: certName, Namespace: namespace}},
			},
		}},
	}))

	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(cluster.Agents[0], &libservice.ServiceOpts{
		ID:        serviceName,
		Name:      serviceName,
		Namespace: namespace,
		HTTPPort:  serviceHTTPPort,
		GRPCPort:  serviceGRPCPort,
	})
	require.NoError(t, err)

	require.NoError(t, cluster.ConfigEntryWrite(&api.TCPRouteConfigEntry{
		Kind:      api.TCPRoute,
		Name:      routeName,
		Namespace: namespace,
		Parents:   []api.ResourceReference{{Kind: api.APIGateway, Namespace: namespace, Name: gatewayName}},
		Services:  []api.TCPService{{Namespace: namespace, Name: serviceName}},
	}))
	checkGatewayConfigEntry(t, client, gatewayName, &api.QueryOptions{Namespace: namespace})
	checkTCPRouteConfigEntry(t, client, routeName, namespace)

	mappedPort, err := gateway.GetPort(listenerPort)
	require.NoError(t, err)
	addr := net.JoinHostPort("localhost", strconv.Itoa(mappedPort))

	servedCommonName := func(r retry.TestingTB) string {
		cn, err := peerCertificateCommonName(addr, "one.example.com")
		require.NoError(r, err)
		return cn
	}

	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		require.Equal(r, "one.example.com", servedCommonName(r))
	})

	rotated := inlineCertificate(t, certName, "two.example.com")
	rotated.Namespace = namespace
	require.NoError(t, cluster.ConfigEntryWrite(rotated))

	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		require.Equal(r, "two.example.com", servedCommonName(r), "the gateway must serve the rotated certificate")
	})
}
