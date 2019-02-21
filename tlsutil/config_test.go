package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/require"
)

func TestConfig_AppendCA_None(t *testing.T) {
	conf := &Config{}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(pool.Subjects()) != 0 {
		t.Fatalf("bad: %v", pool.Subjects())
	}
}

func TestConfig_CACertificate_Valid(t *testing.T) {
	conf := &Config{
		CAFile: "../test/ca/root.cer",
	}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(pool.Subjects()) == 0 {
		t.Fatalf("expected cert")
	}
}

func TestConfig_KeyPair_None(t *testing.T) {
	conf := &Config{}
	cert, err := conf.KeyPair()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cert != nil {
		t.Fatalf("bad: %v", cert)
	}
}

func TestConfig_KeyPair_Valid(t *testing.T) {
	conf := &Config{
		CertFile: "../test/key/ourdomain.cer",
		KeyFile:  "../test/key/ourdomain.key",
	}
	cert, err := conf.KeyPair()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cert == nil {
		t.Fatalf("expected cert")
	}
}

func TestConfigurator_OutgoingTLS_MissingCA(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
	}
	c := NewConfigurator(conf)
	tlsConf, err := c.OutgoingRPCConfig()
	require.Error(t, err)
	require.Nil(t, tlsConf)
}

func TestConfigurator_OutgoingTLS_OnlyCA(t *testing.T) {
	conf := &Config{
		CAFile: "../test/ca/root.cer",
	}
	c := NewConfigurator(conf)
	tlsConf, err := c.OutgoingRPCConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
}

func TestConfigurator_OutgoingTLS_VerifyOutgoing(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
	}
	c := NewConfigurator(conf)
	tlsConf, err := c.OutgoingRPCConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	require.Equal(t, len(tlsConf.RootCAs.Subjects()), 1)
	require.Empty(t, tlsConf.ServerName)
	require.True(t, tlsConf.InsecureSkipVerify)
}

func TestConfig_SkipBuiltinVerify(t *testing.T) {
	type variant struct {
		config Config
		result bool
	}
	table := []variant{
		variant{Config{ServerName: "", VerifyServerHostname: true}, false},
		variant{Config{ServerName: "", VerifyServerHostname: false}, true},
		variant{Config{ServerName: "consul", VerifyServerHostname: true}, false},
		variant{Config{ServerName: "consul", VerifyServerHostname: false}, false},
	}

	for _, v := range table {
		require.Equal(t, v.result, v.config.skipBuiltinVerify())
	}
}

func TestConfigurator_OutgoingTLS_ServerName(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
		ServerName:     "consul.example.com",
	}
	c := NewConfigurator(conf)
	tlsConf, err := c.OutgoingRPCConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	require.Equal(t, len(tlsConf.RootCAs.Subjects()), 1)
	require.Equal(t, tlsConf.ServerName, "consul.example.com")
	require.False(t, tlsConf.InsecureSkipVerify)
}

func TestConfigurator_OutgoingTLS_VerifyHostname(t *testing.T) {
	conf := &Config{
		VerifyOutgoing:       true,
		VerifyServerHostname: true,
		CAFile:               "../test/ca/root.cer",
	}
	c := NewConfigurator(conf)
	tlsConf, err := c.OutgoingRPCConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	require.Equal(t, len(tlsConf.RootCAs.Subjects()), 1)
	require.False(t, tlsConf.InsecureSkipVerify)
}

func TestConfigurator_OutgoingTLS_WithKeyPair(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}
	c := NewConfigurator(conf)
	tlsConf, err := c.OutgoingRPCConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	require.True(t, tlsConf.InsecureSkipVerify)
	require.Equal(t, len(tlsConf.Certificates), 1)
}

func TestConfigurator_OutgoingTLS_TLSMinVersion(t *testing.T) {
	tlsVersions := []string{"tls10", "tls11", "tls12"}
	for _, version := range tlsVersions {
		conf := &Config{
			VerifyOutgoing: true,
			CAFile:         "../test/ca/root.cer",
			TLSMinVersion:  version,
		}
		c := NewConfigurator(conf)
		tlsConf, err := c.OutgoingRPCConfig()
		require.NoError(t, err)
		require.NotNil(t, tlsConf)
		require.Equal(t, tlsConf.MinVersion, TLSLookup[version])
	}
}

func startTLSServer(config *Config) (net.Conn, chan error) {
	errc := make(chan error, 1)

	c := NewConfigurator(config)
	tlsConfigServer, err := c.IncomingRPCConfig()
	if err != nil {
		errc <- err
		return nil, errc
	}

	client, server := net.Pipe()

	// Use yamux to buffer the reads, otherwise it's easy to deadlock
	muxConf := yamux.DefaultConfig()
	serverSession, _ := yamux.Server(server, muxConf)
	clientSession, _ := yamux.Client(client, muxConf)
	clientConn, _ := clientSession.Open()
	serverConn, _ := serverSession.Accept()

	go func() {
		tlsServer := tls.Server(serverConn, tlsConfigServer)
		if err := tlsServer.Handshake(); err != nil {
			errc <- err
		}
		close(errc)

		// Because net.Pipe() is unbuffered, if both sides
		// Close() simultaneously, we will deadlock as they
		// both send an alert and then block. So we make the
		// server read any data from the client until error or
		// EOF, which will allow the client to Close(), and
		// *then* we Close() the server.
		io.Copy(ioutil.Discard, tlsServer)
		tlsServer.Close()
	}()
	return clientConn, errc
}

func TestConfigurator_outgoingWrapper_OK(t *testing.T) {
	config := &Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Alice.crt",
		KeyFile:              "../test/hostname/Alice.key",
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		Domain:               "consul",
	}

	client, errc := startTLSServer(config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c := NewConfigurator(config)
	wrap, err := c.OutgoingRPCWrapper()
	require.NoError(t, err)

	tlsClient, err := wrap("dc1", client)
	require.NoError(t, err)

	defer tlsClient.Close()
	err = tlsClient.(*tls.Conn).Handshake()
	require.NoError(t, err)

	err = <-errc
	require.NoError(t, err)
}

func TestConfigurator_outgoingWrapper_BadDC(t *testing.T) {
	config := &Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Alice.crt",
		KeyFile:              "../test/hostname/Alice.key",
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		Domain:               "consul",
	}

	client, errc := startTLSServer(config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c := NewConfigurator(config)
	wrap, err := c.OutgoingRPCWrapper()
	require.NoError(t, err)

	tlsClient, err := wrap("dc2", client)
	require.NoError(t, err)

	err = tlsClient.(*tls.Conn).Handshake()
	_, ok := err.(x509.HostnameError)
	require.True(t, ok)
	tlsClient.Close()

	<-errc
}

func TestConfigurator_outgoingWrapper_BadCert(t *testing.T) {
	config := &Config{
		CAFile:               "../test/ca/root.cer",
		CertFile:             "../test/key/ourdomain.cer",
		KeyFile:              "../test/key/ourdomain.key",
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		Domain:               "consul",
	}

	client, errc := startTLSServer(config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c := NewConfigurator(config)
	wrap, err := c.OutgoingRPCWrapper()
	require.NoError(t, err)

	tlsClient, err := wrap("dc1", client)
	require.NoError(t, err)

	err = tlsClient.(*tls.Conn).Handshake()
	if _, ok := err.(x509.HostnameError); !ok {
		t.Fatalf("should get hostname err: %v", err)
	}
	tlsClient.Close()

	<-errc
}

func TestConfigurator_wrapTLS_OK(t *testing.T) {
	config := &Config{
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
		VerifyOutgoing: true,
	}

	client, errc := startTLSServer(config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c := NewConfigurator(config)
	clientConfig, err := c.OutgoingRPCConfig()
	require.NoError(t, err)

	tlsClient, err := config.wrapTLSClient(client, clientConfig)
	require.NoError(t, err)

	tlsClient.Close()
	err = <-errc
	require.NoError(t, err)
}

func TestConfigurator_wrapTLS_BadCert(t *testing.T) {
	serverConfig := &Config{
		CertFile: "../test/key/ssl-cert-snakeoil.pem",
		KeyFile:  "../test/key/ssl-cert-snakeoil.key",
	}

	client, errc := startTLSServer(serverConfig)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	clientConfig := &Config{
		CAFile:         "../test/ca/root.cer",
		VerifyOutgoing: true,
	}

	c := NewConfigurator(clientConfig)
	clientTLSConfig, err := c.OutgoingRPCConfig()
	require.NoError(t, err)

	tlsClient, err := clientConfig.wrapTLSClient(client, clientTLSConfig)
	require.Error(t, err)
	require.Nil(t, tlsClient)

	err = <-errc
	require.NoError(t, err)
}

func TestConfig_ParseCiphers(t *testing.T) {
	testOk := strings.Join([]string{
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
		"TLS_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_RSA_WITH_AES_256_GCM_SHA384",
		"TLS_RSA_WITH_AES_128_CBC_SHA256",
		"TLS_RSA_WITH_AES_128_CBC_SHA",
		"TLS_RSA_WITH_AES_256_CBC_SHA",
		"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA",
		"TLS_RSA_WITH_3DES_EDE_CBC_SHA",
		"TLS_RSA_WITH_RC4_128_SHA",
		"TLS_ECDHE_RSA_WITH_RC4_128_SHA",
		"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA",
	}, ",")
	ciphers := []uint16{
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		tls.TLS_RSA_WITH_RC4_128_SHA,
		tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
	}
	v, err := ParseCiphers(testOk)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := v, ciphers; !reflect.DeepEqual(got, want) {
		t.Fatalf("got ciphers %#v want %#v", got, want)
	}

	testBad := "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,cipherX"
	if _, err := ParseCiphers(testBad); err == nil {
		t.Fatal("should fail on unsupported cipherX")
	}
}

func TestConfigurator_IncomingHTTPSConfig_CA_PATH(t *testing.T) {
	conf := &Config{CAPath: "../test/ca_path"}

	c := NewConfigurator(conf)
	tlsConf, err := c.IncomingHTTPSConfig()
	require.NoError(t, err)
	require.Equal(t, len(tlsConf.ClientCAs.Subjects()), 2)
}

func TestConfigurator_IncomingHTTPS(t *testing.T) {
	conf := &Config{
		VerifyIncoming: true,
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}
	c := NewConfigurator(conf)
	tlsConf, err := c.IncomingHTTPSConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	require.Equal(t, len(tlsConf.ClientCAs.Subjects()), 1)
	require.Equal(t, tlsConf.ClientAuth, tls.RequireAndVerifyClientCert)
	require.Equal(t, len(tlsConf.Certificates), 1)
}

func TestConfigurator_IncomingHTTPS_MissingCA(t *testing.T) {
	conf := &Config{
		VerifyIncoming: true,
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}
	c := NewConfigurator(conf)
	_, err := c.IncomingHTTPSConfig()
	require.Error(t, err)
}

func TestConfigurator_IncomingHTTPS_MissingKey(t *testing.T) {
	conf := &Config{
		VerifyIncoming: true,
		CAFile:         "../test/ca/root.cer",
	}
	c := NewConfigurator(conf)
	_, err := c.IncomingHTTPSConfig()
	require.Error(t, err)
}

func TestConfigurator_IncomingHTTPS_NoVerify(t *testing.T) {
	conf := &Config{}
	c := NewConfigurator(conf)
	tlsConf, err := c.IncomingHTTPSConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	require.Equal(t, len(tlsConf.ClientCAs.Subjects()), 0)
	require.Equal(t, tlsConf.ClientAuth, tls.NoClientCert)
	require.Equal(t, len(tlsConf.Certificates), 0)
}

func TestConfigurator_IncomingHTTPS_TLSMinVersion(t *testing.T) {
	tlsVersions := []string{"tls10", "tls11", "tls12"}
	for _, version := range tlsVersions {
		conf := &Config{
			VerifyIncoming: true,
			CAFile:         "../test/ca/root.cer",
			CertFile:       "../test/key/ourdomain.cer",
			KeyFile:        "../test/key/ourdomain.key",
			TLSMinVersion:  version,
		}
		c := NewConfigurator(conf)
		tlsConf, err := c.IncomingHTTPSConfig()
		require.NoError(t, err)
		require.NotNil(t, tlsConf)
		require.Equal(t, tlsConf.MinVersion, TLSLookup[version])
	}
}

func TestConfigurator_IncomingHTTPSCAPath_Valid(t *testing.T) {
	conf := &Config{
		CAPath: "../test/ca_path",
	}

	c := NewConfigurator(conf)
	tlsConf, err := c.IncomingHTTPSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(tlsConf.ClientCAs.Subjects()) != 2 {
		t.Fatalf("expected certs")
	}
}
