package sarama

import (
	"math/big"
	"net"
	"testing"
	"time"

	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
)

func TestTLS(t *testing.T) {
	cakey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	clientkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	hostkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	nvb := time.Now().Add(-1 * time.Hour)
	nva := time.Now().Add(1 * time.Hour)

	caTemplate := &x509.Certificate{
		Subject:      pkix.Name{CommonName: "ca"},
		Issuer:       pkix.Name{CommonName: "ca"},
		SerialNumber: big.NewInt(0),
		NotAfter:     nva,
		NotBefore:    nvb,
		IsCA:         true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDer, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &cakey.PublicKey, cakey)
	if err != nil {
		t.Fatal(err)
	}
	caFinalCert, err := x509.ParseCertificate(caDer)
	if err != nil {
		t.Fatal(err)
	}

	hostDer, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		Subject:      pkix.Name{CommonName: "host"},
		Issuer:       pkix.Name{CommonName: "ca"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		SerialNumber: big.NewInt(0),
		NotAfter:     nva,
		NotBefore:    nvb,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, caFinalCert, &hostkey.PublicKey, cakey)
	if err != nil {
		t.Fatal(err)
	}

	clientDer, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		Subject:      pkix.Name{CommonName: "client"},
		Issuer:       pkix.Name{CommonName: "ca"},
		SerialNumber: big.NewInt(0),
		NotAfter:     nva,
		NotBefore:    nvb,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}, caFinalCert, &clientkey.PublicKey, cakey)
	if err != nil {
		t.Fatal(err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(caFinalCert)

	systemCerts, err := x509.SystemCertPool()
	if err != nil {
		t.Fatal(err)
	}

	// Keep server the same - it's the client that we're testing
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{tls.Certificate{
			Certificate: [][]byte{hostDer},
			PrivateKey:  hostkey,
		}},
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  pool,
	}

	for _, tc := range []struct {
		Succeed        bool
		Server, Client *tls.Config
	}{
		{ // Verify client fails if wrong CA cert pool is specified
			Succeed: false,
			Server:  serverTLSConfig,
			Client: &tls.Config{
				RootCAs: systemCerts,
				Certificates: []tls.Certificate{tls.Certificate{
					Certificate: [][]byte{clientDer},
					PrivateKey:  clientkey,
				}},
			},
		},
		{ // Verify client fails if wrong key is specified
			Succeed: false,
			Server:  serverTLSConfig,
			Client: &tls.Config{
				RootCAs: pool,
				Certificates: []tls.Certificate{tls.Certificate{
					Certificate: [][]byte{clientDer},
					PrivateKey:  hostkey,
				}},
			},
		},
		{ // Verify client fails if wrong cert is specified
			Succeed: false,
			Server:  serverTLSConfig,
			Client: &tls.Config{
				RootCAs: pool,
				Certificates: []tls.Certificate{tls.Certificate{
					Certificate: [][]byte{hostDer},
					PrivateKey:  clientkey,
				}},
			},
		},
		{ // Verify client fails if no CAs are specified
			Succeed: false,
			Server:  serverTLSConfig,
			Client: &tls.Config{
				Certificates: []tls.Certificate{tls.Certificate{
					Certificate: [][]byte{clientDer},
					PrivateKey:  clientkey,
				}},
			},
		},
		{ // Verify client fails if no keys are specified
			Succeed: false,
			Server:  serverTLSConfig,
			Client: &tls.Config{
				RootCAs: pool,
			},
		},
		{ // Finally, verify it all works happily with client and server cert in place
			Succeed: true,
			Server:  serverTLSConfig,
			Client: &tls.Config{
				RootCAs: pool,
				Certificates: []tls.Certificate{tls.Certificate{
					Certificate: [][]byte{clientDer},
					PrivateKey:  clientkey,
				}},
			},
		},
	} {
		doListenerTLSTest(t, tc.Succeed, tc.Server, tc.Client)
	}
}

func doListenerTLSTest(t *testing.T, expectSuccess bool, serverConfig, clientConfig *tls.Config) {
	serverConfig.BuildNameToCertificate()
	clientConfig.BuildNameToCertificate()

	seedListener, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		t.Fatal("cannot open listener", err)
	}

	var childT *testing.T
	if expectSuccess {
		childT = t
	} else {
		childT = &testing.T{} // we want to swallow errors
	}

	seedBroker := NewMockBrokerListener(childT, 1, seedListener)
	defer seedBroker.Close()

	seedBroker.Returns(new(MetadataResponse))

	config := NewConfig()
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = clientConfig

	client, err := NewClient([]string{seedBroker.Addr()}, config)
	if err == nil {
		safeClose(t, client)
	}

	if expectSuccess {
		if err != nil {
			t.Fatal(err)
		}
	} else {
		if err == nil {
			t.Fatal("expected failure")
		}
	}
}
