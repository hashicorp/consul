// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

const serverCertName = "server.dc1.consul"

// debugConfig returns the agent's effective configuration, which shows the TLS
// settings that are actually in force.
func debugConfig(t *testing.T, e *env, server bool) map[string]any {
	t.Helper()

	c := e.client
	if server {
		c = e.server
	}
	self, err := c.Agent().Self()
	require.NoError(t, err)
	cfg, ok := self["DebugConfig"]
	require.True(t, ok, "the agent did not return its debug configuration")
	return cfg
}

func section(t *testing.T, cfg map[string]any, path ...string) map[string]any {
	t.Helper()
	var cur any = cfg
	for _, p := range path {
		m, ok := cur.(map[string]any)
		require.True(t, ok, "expected an object at %q", p)
		cur, ok = m[p]
		require.True(t, ok, "missing %q", p)
	}
	m, ok := cur.(map[string]any)
	require.True(t, ok)
	return m
}

// TestEncryption_Posture verifies that the datacenter really runs with
// gossip encryption, verified RPC TLS and auto-encrypt, by reading the
// configuration the agents are running with.
func TestEncryption_Posture(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")

	t.Run("the server verifies peers in both directions and checks hostnames", func(t *testing.T) {
		cfg := debugConfig(t, e, true)
		rpc := section(t, cfg, "TLS", "InternalRPC")

		require.Equal(t, true, rpc["VerifyIncoming"], "servers require a client certificate on the RPC port")
		require.Equal(t, true, rpc["VerifyOutgoing"])
		require.Equal(t, true, rpc["VerifyServerHostname"], "servers must present a name like server.<dc>.<domain>")
		require.NotEmpty(t, rpc["CAFile"])
		require.NotEmpty(t, rpc["CertFile"])
		require.Equal(t, "TLSv1_2", rpc["TLSMinVersion"])
		require.Equal(t, true, cfg["AutoEncryptAllowTLS"], "the server hands certificates to clients")
		require.Equal(t, "hidden", cfg["EncryptKey"], "the gossip key is set and never echoed")
	})

	t.Run("the client gets its certificate from the servers", func(t *testing.T) {
		cfg := debugConfig(t, e, false)
		rpc := section(t, cfg, "TLS", "InternalRPC")

		require.Equal(t, true, cfg["AutoEncryptTLS"])
		require.Equal(t, true, section(t, cfg, "TLS")["AutoTLS"])
		require.Equal(t, true, rpc["VerifyOutgoing"])
		require.Equal(t, true, rpc["VerifyServerHostname"])
		require.Empty(t, rpc["CertFile"], "no certificate is configured on the client; it is requested at startup")
		require.Equal(t, "hidden", cfg["EncryptKey"])
	})

	t.Run("the encrypted channels carry real traffic", func(t *testing.T) {
		// The client reaches the servers only over verified TLS, so any request that
		// goes through it proves the client's certificate was accepted.
		_, err := e.client.KV().Put(&api.KVPair{Key: "posture/check", Value: []byte("ok")}, nil)
		require.NoError(t, err)

		members, err := e.client.Agent().Members(false)
		require.NoError(t, err)
		require.Len(t, members, 2)
	})
}

// dialTLS completes a TLS handshake with addr using cfg and returns the
// certificates the server presented.
func dialTLS(addr string, cfg *tls.Config) ([]*x509.Certificate, error) {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn.ConnectionState().PeerCertificates, nil
}

func poolFromPEM(t *testing.T, pem string) *x509.CertPool {
	t.Helper()
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM([]byte(pem)), "the CA is not valid PEM")
	return pool
}

// TestTLS_HTTPSServerIdentity verifies what clients of the HTTPS API can rely
// on: the server proves its identity with a certificate from the cluster's CA,
// and clients that do not trust that CA, or expect another name, are refused.
func TestTLS_HTTPSServerIdentity(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	addr := e.httpsAddr(t)
	roots := poolFromPEM(t, e.caPEM())

	t.Run("the certificate chains to the cluster CA and names the server", func(t *testing.T) {
		certs, err := dialTLS(addr, &tls.Config{RootCAs: roots, ServerName: serverCertName, MinVersion: tls.VersionTLS12})
		require.NoError(t, err)
		require.NotEmpty(t, certs)

		leaf := certs[0]
		require.Equal(t, serverCertName, leaf.Subject.CommonName)
		require.Contains(t, leaf.DNSNames, serverCertName)
		require.True(t, strings.HasPrefix(leaf.Issuer.CommonName, "Consul Agent CA"), "issuer is %q", leaf.Issuer.CommonName)
		require.True(t, time.Now().Before(leaf.NotAfter), "the certificate must not be expired")
	})

	t.Run("a client that does not trust the CA is refused", func(t *testing.T) {
		_, err := dialTLS(addr, &tls.Config{ServerName: serverCertName, MinVersion: tls.VersionTLS12})

		require.Error(t, err)
		require.Regexp(t, `unknown authority|not trusted`, err.Error())
	})

	t.Run("a client that expects another name is refused", func(t *testing.T) {
		_, err := dialTLS(addr, &tls.Config{RootCAs: roots, ServerName: "payments.example.com", MinVersion: tls.VersionTLS12})

		require.Error(t, err)
		require.Contains(t, err.Error(), "not payments.example.com")
	})

	t.Run("a request over verified TLS reaches the API", func(t *testing.T) {
		client := &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{
				RootCAs: roots, ServerName: serverCertName, MinVersion: tls.VersionTLS12,
			}},
		}
		resp, err := client.Get("https://" + addr + "/v1/status/leader")
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NotEmpty(t, string(body))
	})

	t.Run("plain HTTP on the TLS port is not served", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get("http://" + addr + "/v1/status/leader")
		if err == nil {
			defer resp.Body.Close()
			require.Equal(t, http.StatusBadRequest, resp.StatusCode, "the TLS listener must not answer a plaintext request")
		}
	})
}

// TestTLS_HTTPSClientCertificates verifies tls.https.verify_incoming: when it
// is on, API clients must present a certificate signed by the cluster CA.
func TestTLS_HTTPSClientCertificates(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "tls { https { verify_incoming = true } }")
	addr := e.httpsAddr(t)
	roots := poolFromPEM(t, e.caPEM())

	get := func(clientCert *tls.Certificate) error {
		cfg := &tls.Config{RootCAs: roots, ServerName: serverCertName, MinVersion: tls.VersionTLS12}
		if clientCert != nil {
			cfg.Certificates = []tls.Certificate{*clientCert}
		}
		client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: cfg}}
		resp, err := client.Get("https://" + addr + "/v1/status/leader")
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}

	t.Run("a client with no certificate is rejected", func(t *testing.T) {
		err := get(nil)

		require.Error(t, err)
		require.Regexp(t, `certificate required|bad certificate|handshake failure|EOF|reset`, err.Error())
	})

	t.Run("a client certificate from an unknown CA is rejected", func(t *testing.T) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		template := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject:      pkix.Name{CommonName: "intruder"},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		require.NoError(t, err)

		err = get(&tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key})

		require.Error(t, err)
		require.Regexp(t, `certificate required|unknown certificate authority|bad certificate|handshake failure|EOF|reset`, err.Error())
	})

	t.Run("the plaintext API port is unaffected", func(t *testing.T) {
		_, err := e.server.Status().Leader()
		require.NoError(t, err)
	})
}
