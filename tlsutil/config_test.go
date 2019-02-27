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
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)
	require.Empty(t, tlsConf.ServerName)
	require.True(t, tlsConf.InsecureSkipVerify)
}

func TestConfigurator_OutgoingRPC_ServerName(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
		ServerName:     "consul.example.com",
	}
	c := NewConfigurator(conf)
	tlsConf, err := c.OutgoingRPCConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)
	require.Empty(t, tlsConf.ServerName)
	require.True(t, tlsConf.InsecureSkipVerify)
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
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)
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
	require.Len(t, tlsConf.Certificates, 1)
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
	require.Len(t, tlsConf.ClientCAs.Subjects(), 2)
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
	require.Len(t, tlsConf.ClientCAs.Subjects(), 1)
	require.Equal(t, tlsConf.ClientAuth, tls.RequireAndVerifyClientCert)
	require.Len(t, tlsConf.Certificates, 1)
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
	require.Nil(t, tlsConf.ClientCAs)
	require.Equal(t, tlsConf.ClientAuth, tls.NoClientCert)
	require.Empty(t, tlsConf.Certificates)
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

	c := NewConfigurator(&Config{CAPath: "../test/ca_path"})
	tlsConf, err := c.IncomingHTTPSConfig()
	require.NoError(t, err)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 2)
}

func TestConfigurator_CommonTLSConfigNoBaseConfig(t *testing.T) {
	c := NewConfigurator(nil)
	_, err := c.commonTLSConfig(false)
	require.Error(t, err)
}

func TestConfigurator_CommonTLSConfigServerNameNodeName(t *testing.T) {
	type variant struct {
		config *Config
		result string
	}
	variants := []variant{
		{config: &Config{NodeName: "node", ServerName: "server"},
			result: "server"},
		{config: &Config{ServerName: "server"},
			result: "server"},
		{config: &Config{NodeName: "node"},
			result: "node"},
	}
	for _, v := range variants {
		c := NewConfigurator(v.config)
		tlsConf, err := c.commonTLSConfig(false)
		require.NoError(t, err)
		require.Empty(t, tlsConf.ServerName)
	}
}

func TestConfigurator_CommonTLSConfigCipherSuites(t *testing.T) {
	c := NewConfigurator(&Config{})
	tlsConfig, err := c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Empty(t, tlsConfig.CipherSuites)

	conf := &Config{CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305}}
	c.Update(conf)
	tlsConfig, err = c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Equal(t, conf.CipherSuites, tlsConfig.CipherSuites)
}

func TestConfigurator_CommonTLSConfigCertKey(t *testing.T) {
	c := NewConfigurator(&Config{})
	tlsConf, err := c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Empty(t, tlsConf.Certificates)

	c.Update(&Config{CertFile: "/something/bogus", KeyFile: "/more/bogus"})
	tlsConf, err = c.commonTLSConfig(false)
	require.Error(t, err)

	c.Update(&Config{CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Len(t, tlsConf.Certificates, 1)
}

func TestConfigurator_CommonTLSConfigTLSMinVersion(t *testing.T) {
	tlsVersions := []string{"tls10", "tls11", "tls12"}
	for _, version := range tlsVersions {
		c := NewConfigurator(&Config{TLSMinVersion: version})
		tlsConf, err := c.commonTLSConfig(false)
		require.NoError(t, err)
		require.Equal(t, tlsConf.MinVersion, TLSLookup[version])
	}

	c := NewConfigurator(&Config{TLSMinVersion: "tlsBOGUS"})
	_, err := c.commonTLSConfig(false)
	require.Error(t, err)
}

func TestConfigurator_CommonTLSConfigValidateVerifyOutgoingCA(t *testing.T) {
	c := NewConfigurator(&Config{VerifyOutgoing: true})
	_, err := c.commonTLSConfig(false)
	require.Error(t, err)
}

func TestConfigurator_CommonTLSConfigLoadCA(t *testing.T) {
	c := NewConfigurator(&Config{})
	tlsConf, err := c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Nil(t, tlsConf.RootCAs)
	require.Nil(t, tlsConf.ClientCAs)

	c.Update(&Config{CAFile: "/something/bogus"})
	_, err = c.commonTLSConfig(false)
	require.Error(t, err)

	c.Update(&Config{CAPath: "/something/bogus/"})
	_, err = c.commonTLSConfig(false)
	require.Error(t, err)

	c.Update(&Config{CAFile: "../test/ca/root.cer"})
	tlsConf, err = c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 1)

	c.Update(&Config{CAPath: "../test/ca_path"})
	tlsConf, err = c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Len(t, tlsConf.RootCAs.Subjects(), 2)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 2)

	c.Update(&Config{CAFile: "../test/ca/root.cer", CAPath: "../test/ca_path"})
	tlsConf, err = c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 1)
}

func TestConfigurator_CommonTLSConfigVerifyIncoming(t *testing.T) {
	c := NewConfigurator(&Config{})
	tlsConf, err := c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Equal(t, tls.NoClientCert, tlsConf.ClientAuth)

	c.Update(&Config{VerifyIncoming: true})
	tlsConf, err = c.commonTLSConfig(false)
	require.Error(t, err)

	c.Update(&Config{VerifyIncoming: true, CAFile: "../test/ca/root.cer"})
	tlsConf, err = c.commonTLSConfig(false)
	require.Error(t, err)

	c.Update(&Config{VerifyIncoming: true, CAFile: "../test/ca/root.cer", CertFile: "../test/cert/ourdomain.cer"})
	tlsConf, err = c.commonTLSConfig(false)
	require.Error(t, err)

	c.Update(&Config{VerifyIncoming: true, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.commonTLSConfig(false)
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)

	c.Update(&Config{VerifyIncoming: false, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.commonTLSConfig(true)
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)

	c.Update(&Config{VerifyServerHostname: false, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.commonTLSConfig(false)
	require.NoError(t, err)
	require.True(t, tlsConf.InsecureSkipVerify)

	c.Update(&Config{VerifyServerHostname: true, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.commonTLSConfig(false)
	require.NoError(t, err)
	require.False(t, tlsConf.InsecureSkipVerify)
}

func TestConfigurator_IncomingRPCConfig(t *testing.T) {
	c := NewConfigurator(&Config{})
	tlsConf, err := c.IncomingRPCConfig()
	require.NoError(t, err)
	require.Equal(t, tls.NoClientCert, tlsConf.ClientAuth)

	c.Update(&Config{VerifyIncoming: true, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.IncomingRPCConfig()
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)

	c.Update(&Config{VerifyIncomingRPC: true, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.IncomingRPCConfig()
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)

	c.Update(&Config{VerifyIncomingHTTPS: true, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.IncomingRPCConfig()
	require.NoError(t, err)
	require.Equal(t, tls.NoClientCert, tlsConf.ClientAuth)
}

func TestConfigurator_IncomingHTTPSConfig(t *testing.T) {
	c := NewConfigurator(&Config{})
	tlsConf, err := c.IncomingHTTPSConfig()
	require.NoError(t, err)
	require.Equal(t, tls.NoClientCert, tlsConf.ClientAuth)

	c.Update(&Config{VerifyIncoming: true, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.IncomingHTTPSConfig()
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)

	c.Update(&Config{VerifyIncomingHTTPS: true, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.IncomingHTTPSConfig()
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)

	c.Update(&Config{VerifyIncomingRPC: true, CAFile: "../test/ca/root.cer", CertFile: "../test/key/ourdomain.cer", KeyFile: "../test/key/ourdomain.key"})
	tlsConf, err = c.IncomingHTTPSConfig()
	require.NoError(t, err)
	require.Equal(t, tls.NoClientCert, tlsConf.ClientAuth)
}

func TestConfigurator_OutgoingRPCConfig(t *testing.T) {
	c := NewConfigurator(&Config{})
	tlsConf, err := c.OutgoingRPCConfig()
	require.NoError(t, err)
	require.Nil(t, tlsConf)

	c.Update(&Config{VerifyOutgoing: true})
	tlsConf, err = c.OutgoingRPCConfig()
	require.Error(t, err)

	c.Update(&Config{VerifyOutgoing: true, CAFile: "../test/ca/root.cer"})
	tlsConf, err = c.OutgoingRPCConfig()
	require.NoError(t, err)

	c.Update(&Config{VerifyOutgoing: true, CAPath: "../test/ca_path"})
	tlsConf, err = c.OutgoingRPCConfig()
	require.NoError(t, err)
}

func TestConfigurator_OutgoingTLSConfigForChecks(t *testing.T) {
	c := NewConfigurator(&Config{})
	tlsConf, err := c.OutgoingTLSConfigForCheck("")
	require.NoError(t, err)
	require.False(t, tlsConf.InsecureSkipVerify)

	c.Update(&Config{EnableAgentTLSForChecks: true})
	tlsConf, err = c.OutgoingTLSConfigForCheck("")
	require.NoError(t, err)
	require.False(t, tlsConf.InsecureSkipVerify)

	c.AddCheck("c1", true)
	c.Update(&Config{EnableAgentTLSForChecks: true})
	tlsConf, err = c.OutgoingTLSConfigForCheck("c1")
	require.NoError(t, err)
	require.True(t, tlsConf.InsecureSkipVerify)

	c.AddCheck("c1", false)
	c.Update(&Config{EnableAgentTLSForChecks: true})
	tlsConf, err = c.OutgoingTLSConfigForCheck("c1")
	require.NoError(t, err)
	require.False(t, tlsConf.InsecureSkipVerify)

	c.AddCheck("c1", false)
	c.Update(&Config{EnableAgentTLSForChecks: true})
	tlsConf, err = c.OutgoingTLSConfigForCheck("c1")
	require.NoError(t, err)
	require.False(t, tlsConf.InsecureSkipVerify)

	c.Update(&Config{EnableAgentTLSForChecks: true, NodeName: "node", ServerName: "server"})
	tlsConf, err = c.OutgoingTLSConfigForCheck("")
	require.NoError(t, err)
	require.Equal(t, "server", tlsConf.ServerName)

	c.Update(&Config{EnableAgentTLSForChecks: true, ServerName: "server"})
	tlsConf, err = c.OutgoingTLSConfigForCheck("")
	require.NoError(t, err)
	require.Equal(t, "server", tlsConf.ServerName)

	c.Update(&Config{EnableAgentTLSForChecks: true, NodeName: "node"})
	tlsConf, err = c.OutgoingTLSConfigForCheck("")
	require.NoError(t, err)
	require.Equal(t, "node", tlsConf.ServerName)
}
