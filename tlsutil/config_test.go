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

func TestConfigurator_OutgoingTLS_OnlyCA(t *testing.T) {
	conf := Config{
		CAFile: "../test/ca/root.cer",
	}
	c, err := NewConfigurator(conf, nil)
	require.NoError(t, err)
	tlsConf := c.OutgoingRPCConfig()
	require.NotNil(t, tlsConf)
}

func TestConfigurator_OutgoingTLS_VerifyOutgoing(t *testing.T) {
	conf := Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
	}
	c, err := NewConfigurator(conf, nil)
	require.NoError(t, err)
	tlsConf := c.OutgoingRPCConfig()
	require.NotNil(t, tlsConf)
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)
	require.Empty(t, tlsConf.ServerName)
	require.True(t, tlsConf.InsecureSkipVerify)
}

func TestConfigurator_OutgoingTLS_ServerName(t *testing.T) {
	conf := Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
		ServerName:     "consul.example.com",
	}
	c, err := NewConfigurator(conf, nil)
	require.NoError(t, err)
	tlsConf := c.OutgoingRPCConfig()
	require.NotNil(t, tlsConf)
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)
	require.Empty(t, tlsConf.ServerName)
	require.True(t, tlsConf.InsecureSkipVerify)
}

func TestConfigurator_OutgoingTLS_VerifyHostname(t *testing.T) {
	conf := Config{
		VerifyOutgoing:       true,
		VerifyServerHostname: true,
		CAFile:               "../test/ca/root.cer",
	}
	c, err := NewConfigurator(conf, nil)
	require.NoError(t, err)
	tlsConf := c.OutgoingRPCConfig()
	require.NotNil(t, tlsConf)
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)
	require.False(t, tlsConf.InsecureSkipVerify)
}

func TestConfigurator_OutgoingTLS_WithKeyPair(t *testing.T) {
	conf := Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}
	c, err := NewConfigurator(conf, nil)
	require.NoError(t, err)
	tlsConf := c.OutgoingRPCConfig()
	require.NotNil(t, tlsConf)
	require.True(t, tlsConf.InsecureSkipVerify)
	cert, err := tlsConf.GetCertificate(nil)
	require.NoError(t, err)
	require.NotNil(t, cert)
}

func startTLSServer(config *Config) (net.Conn, chan error) {
	errc := make(chan error, 1)

	c, err := NewConfigurator(*config, nil)
	if err != nil {
		errc <- err
		return nil, errc
	}
	tlsConfigServer := c.IncomingRPCConfig()
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
	config := Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Alice.crt",
		KeyFile:              "../test/hostname/Alice.key",
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		Domain:               "consul",
	}

	client, errc := startTLSServer(&config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)
	wrap := c.OutgoingRPCWrapper()
	require.NotNil(t, wrap)

	tlsClient, err := wrap("dc1", client)
	require.NoError(t, err)

	defer tlsClient.Close()
	err = tlsClient.(*tls.Conn).Handshake()
	require.NoError(t, err)

	err = <-errc
	require.NoError(t, err)
}

func TestConfigurator_outgoingWrapper_BadDC(t *testing.T) {
	config := Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Alice.crt",
		KeyFile:              "../test/hostname/Alice.key",
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		Domain:               "consul",
	}

	client, errc := startTLSServer(&config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)
	wrap := c.OutgoingRPCWrapper()

	tlsClient, err := wrap("dc2", client)
	require.NoError(t, err)

	err = tlsClient.(*tls.Conn).Handshake()
	_, ok := err.(x509.HostnameError)
	require.True(t, ok)
	tlsClient.Close()

	<-errc
}

func TestConfigurator_outgoingWrapper_BadCert(t *testing.T) {
	config := Config{
		CAFile:               "../test/ca/root.cer",
		CertFile:             "../test/key/ourdomain.cer",
		KeyFile:              "../test/key/ourdomain.key",
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		Domain:               "consul",
	}

	client, errc := startTLSServer(&config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)
	wrap := c.OutgoingRPCWrapper()

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
	config := Config{
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
		VerifyOutgoing: true,
	}

	client, errc := startTLSServer(&config)
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)

	tlsClient, err := c.wrapTLSClient("dc1", client)
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

	clientConfig := Config{
		CAFile:         "../test/ca/root.cer",
		VerifyOutgoing: true,
	}

	c, err := NewConfigurator(clientConfig, nil)
	require.NoError(t, err)
	tlsClient, err := c.wrapTLSClient("dc1", client)
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
	conf := Config{CAPath: "../test/ca_path"}

	c, err := NewConfigurator(conf, nil)
	require.NoError(t, err)
	tlsConf := c.IncomingHTTPSConfig()
	require.Len(t, tlsConf.ClientCAs.Subjects(), 2)
}

func TestConfigurator_IncomingHTTPS(t *testing.T) {
	conf := Config{
		VerifyIncoming: true,
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}
	c, err := NewConfigurator(conf, nil)
	require.NoError(t, err)
	tlsConf := c.IncomingHTTPSConfig()
	require.NotNil(t, tlsConf)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 1)
	require.Equal(t, tlsConf.ClientAuth, tls.RequireAndVerifyClientCert)
	cert, err := tlsConf.GetCertificate(nil)
	require.NoError(t, err)
	require.NotNil(t, cert)
}

func TestConfigurator_IncomingHTTPS_MissingCA(t *testing.T) {
	conf := Config{
		VerifyIncoming: true,
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}
	_, err := NewConfigurator(conf, nil)
	require.Error(t, err)
}

func TestConfigurator_IncomingHTTPS_MissingKey(t *testing.T) {
	conf := Config{
		VerifyIncoming: true,
		CAFile:         "../test/ca/root.cer",
	}
	_, err := NewConfigurator(conf, nil)
	require.Error(t, err)
}

func TestConfigurator_IncomingHTTPS_NoVerify(t *testing.T) {
	conf := Config{}
	c, err := NewConfigurator(conf, nil)
	require.NoError(t, err)
	tlsConf := c.IncomingHTTPSConfig()
	require.NotNil(t, tlsConf)
	require.Nil(t, tlsConf.ClientCAs)
	require.Equal(t, tlsConf.ClientAuth, tls.NoClientCert)
	require.Empty(t, tlsConf.Certificates)
}

func TestConfigurator_IncomingHTTPSCAPath_Valid(t *testing.T) {

	c, err := NewConfigurator(Config{CAPath: "../test/ca_path"}, nil)
	require.NoError(t, err)
	tlsConf := c.IncomingHTTPSConfig()
	require.Len(t, tlsConf.ClientCAs.Subjects(), 2)
}

//////////////////////////////////////////////////////////////

func TestConfigurator_CommonTLSConfigServerNameNodeName(t *testing.T) {
	type variant struct {
		config Config
		result string
	}
	variants := []variant{
		{config: Config{NodeName: "node", ServerName: "server"},
			result: "server"},
		{config: Config{ServerName: "server"},
			result: "server"},
		{config: Config{NodeName: "node"},
			result: "node"},
	}
	for _, v := range variants {
		c, err := NewConfigurator(v.config, nil)
		require.NoError(t, err)
		tlsConf := c.commonTLSConfig(false)
		require.Empty(t, tlsConf.ServerName)
	}
}

func TestConfigurator_check(t *testing.T) {
	c := &Configurator{}
	require.NoError(t, c.check(Config{}, nil))

	// test tls min version
	require.Error(t, c.check(Config{TLSMinVersion: "tls9"}, nil))
	require.NoError(t, c.check(Config{TLSMinVersion: ""}, nil))
	require.NoError(t, c.check(Config{TLSMinVersion: "tls10"}, nil))
	require.NoError(t, c.check(Config{TLSMinVersion: "tls11"}, nil))
	require.NoError(t, c.check(Config{TLSMinVersion: "tls12"}, nil))

	// test ca and verifyoutgoing
	require.Error(t, c.check(Config{VerifyOutgoing: true, CAFile: "", CAPath: ""}, nil))
	require.NoError(t, c.check(Config{VerifyOutgoing: false, CAFile: "", CAPath: ""}, nil))
	require.NoError(t, c.check(Config{VerifyOutgoing: false, CAFile: "a", CAPath: ""}, nil))
	require.NoError(t, c.check(Config{VerifyOutgoing: false, CAFile: "", CAPath: "a"}, nil))
	require.NoError(t, c.check(Config{VerifyOutgoing: false, CAFile: "a", CAPath: "a"}, nil))
	require.NoError(t, c.check(Config{VerifyOutgoing: true, CAFile: "a", CAPath: ""}, nil))
	require.NoError(t, c.check(Config{VerifyOutgoing: true, CAFile: "", CAPath: "a"}, nil))
	require.NoError(t, c.check(Config{VerifyOutgoing: true, CAFile: "a", CAPath: "a"}, nil))

	// test ca, cert and verifyIncoming
	require.Error(t, c.check(Config{VerifyIncoming: true, CAFile: "", CAPath: ""}, nil))
	require.Error(t, c.check(Config{VerifyIncomingRPC: true, CAFile: "", CAPath: ""}, nil))
	require.Error(t, c.check(Config{VerifyIncomingHTTPS: true, CAFile: "", CAPath: ""}, nil))
	require.Error(t, c.check(Config{VerifyIncoming: true, CAFile: "a", CAPath: ""}, nil))
	require.Error(t, c.check(Config{VerifyIncoming: true, CAFile: "", CAPath: "a"}, nil))
	require.NoError(t, c.check(Config{VerifyIncoming: true, CAFile: "", CAPath: "a"}, &tls.Certificate{}))
}

func TestConfigurator_loadKeyPair(t *testing.T) {
	cert, err := loadKeyPair("", "")
	require.NoError(t, err)
	require.Nil(t, cert)

	cert, err = loadKeyPair("bogus", "")
	require.NoError(t, err)
	require.Nil(t, cert)

	cert, err = loadKeyPair("", "bogus")
	require.NoError(t, err)
	require.Nil(t, cert)

	cert, err = loadKeyPair("bogus", "bogus")
	require.Error(t, err)
	require.Nil(t, cert)

	cert, err = loadKeyPair("../test/key/ourdomain.cer", "../test/key/ourdomain.key")
	require.NoError(t, err)
	require.NotNil(t, cert)
}

func TestConfigurator_loadCAs(t *testing.T) {
	cas, err := loadCAs("", "")
	require.NoError(t, err)
	require.Nil(t, cas)

	cas, err = loadCAs("bogus", "")
	require.Error(t, err)
	require.Nil(t, cas)

	cas, err = loadCAs("../test/ca/root.cer", "")
	require.NoError(t, err)
	require.NotNil(t, cas)

	cas, err = loadCAs("", "../test/ca_path")
	require.NoError(t, err)
	require.NotNil(t, cas)

	cas, err = loadCAs("../test/ca/root.cer", "../test/ca_path")
	require.NoError(t, err)
	require.NotNil(t, cas)
	require.Len(t, cas.Subjects(), 1)
}

func TestConfigurator_CommonTLSConfigInsecureSkipVerify(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)
	tlsConf := c.commonTLSConfig(false)
	require.True(t, tlsConf.InsecureSkipVerify)

	require.NoError(t, c.Update(Config{VerifyServerHostname: false}))
	tlsConf = c.commonTLSConfig(false)
	require.True(t, tlsConf.InsecureSkipVerify)

	require.NoError(t, c.Update(Config{VerifyServerHostname: true}))
	tlsConf = c.commonTLSConfig(false)
	require.False(t, tlsConf.InsecureSkipVerify)
}

func TestConfigurator_CommonTLSConfigPreferServerCipherSuites(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)
	tlsConf := c.commonTLSConfig(false)
	require.False(t, tlsConf.PreferServerCipherSuites)

	require.NoError(t, c.Update(Config{PreferServerCipherSuites: false}))
	tlsConf = c.commonTLSConfig(false)
	require.False(t, tlsConf.PreferServerCipherSuites)

	require.NoError(t, c.Update(Config{PreferServerCipherSuites: true}))
	tlsConf = c.commonTLSConfig(false)
	require.True(t, tlsConf.PreferServerCipherSuites)
}

func TestConfigurator_CommonTLSConfigCipherSuites(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)
	tlsConf := c.commonTLSConfig(false)
	require.Empty(t, tlsConf.CipherSuites)

	conf := Config{CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305}}
	require.NoError(t, c.Update(conf))
	tlsConf = c.commonTLSConfig(false)
	require.Equal(t, conf.CipherSuites, tlsConf.CipherSuites)
}

func TestConfigurator_CommonTLSConfigGetClientCertificate(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)

	cert, err := c.commonTLSConfig(false).GetCertificate(nil)
	require.NoError(t, err)
	require.Nil(t, cert)

	c.cert = &tls.Certificate{}
	cert, err = c.commonTLSConfig(false).GetCertificate(nil)
	require.NoError(t, err)
	require.Equal(t, c.cert, cert)

	cert, err = c.commonTLSConfig(false).GetClientCertificate(nil)
	require.NoError(t, err)
	require.Equal(t, c.cert, cert)
}

func TestConfigurator_CommonTLSConfigCAs(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)
	require.Nil(t, c.commonTLSConfig(false).ClientCAs)
	require.Nil(t, c.commonTLSConfig(false).RootCAs)

	c.cas = &x509.CertPool{}
	require.Equal(t, c.cas, c.commonTLSConfig(false).ClientCAs)
	require.Equal(t, c.cas, c.commonTLSConfig(false).RootCAs)
}

func TestConfigurator_CommonTLSConfigTLSMinVersion(t *testing.T) {
	c, err := NewConfigurator(Config{TLSMinVersion: ""}, nil)
	require.NoError(t, err)
	require.Equal(t, c.commonTLSConfig(false).MinVersion, TLSLookup["tls10"])

	tlsVersions := []string{"tls10", "tls11", "tls12"}
	for _, version := range tlsVersions {
		require.NoError(t, c.Update(Config{TLSMinVersion: version}))
		require.Equal(t, c.commonTLSConfig(false).MinVersion,
			TLSLookup[version])
	}

	require.Error(t, c.Update(Config{TLSMinVersion: "tlsBOGUS"}))
}

func TestConfigurator_CommonTLSConfigVerifyIncoming(t *testing.T) {
	c := Configurator{base: &Config{}}
	require.Equal(t, tls.NoClientCert, c.commonTLSConfig(false).ClientAuth)
	require.Equal(t, tls.RequireAndVerifyClientCert,
		c.commonTLSConfig(true).ClientAuth)
}

func TestConfigurator_OutgoingRPCTLSDisabled(t *testing.T) {
	c := Configurator{base: &Config{}}
	type variant struct {
		verify   bool
		file     string
		path     string
		expected bool
	}
	variants := []variant{
		{false, "", "", true},
		{false, "a", "", false},
		{false, "", "a", false},
		{false, "a", "a", false},
		{true, "", "", false},
		{true, "a", "", false},
		{true, "", "a", false},
		{true, "a", "a", false},
	}
	for _, v := range variants {
		c.base.VerifyOutgoing = v.verify
		c.base.CAFile = v.file
		c.base.CAPath = v.path
		require.Equal(t, v.expected, c.outgoingRPCTLSDisabled())
	}
}

func TestConfigurator_SomeValuesFromConfig(t *testing.T) {
	c := Configurator{base: &Config{
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		Domain:               "abc.de",
	}}
	one, two, three := c.someValuesFromConfig()
	require.Equal(t, c.base.VerifyServerHostname, one)
	require.Equal(t, c.base.VerifyOutgoing, two)
	require.Equal(t, c.base.Domain, three)
}

func TestConfigurator_VerifyIncomingRPC(t *testing.T) {
	c := Configurator{base: &Config{
		VerifyIncomingRPC: true,
	}}
	verify := c.verifyIncomingRPC()
	require.Equal(t, c.base.VerifyIncomingRPC, verify)
}

func TestConfigurator_VerifyIncomingHTTPS(t *testing.T) {
	c := Configurator{base: &Config{
		VerifyIncomingHTTPS: true,
	}}
	verify := c.verifyIncomingHTTPS()
	require.Equal(t, c.base.VerifyIncomingHTTPS, verify)
}

func TestConfigurator_EnableAgentTLSForChecks(t *testing.T) {
	c := Configurator{base: &Config{
		EnableAgentTLSForChecks: true,
	}}
	enabled := c.enableAgentTLSForChecks()
	require.Equal(t, c.base.EnableAgentTLSForChecks, enabled)
}

func TestConfigurator_IncomingRPCConfig(t *testing.T) {
	c, err := NewConfigurator(Config{
		VerifyIncomingRPC: true,
		CAFile:            "../test/ca/root.cer",
		CertFile:          "../test/key/ourdomain.cer",
		KeyFile:           "../test/key/ourdomain.key",
	}, nil)
	require.NoError(t, err)
	tlsConf := c.IncomingRPCConfig()
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)
	require.NotNil(t, tlsConf.GetConfigForClient)
	tlsConf, err = tlsConf.GetConfigForClient(nil)
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)
}

func TestConfigurator_IncomingHTTPSConfig(t *testing.T) {
	c, err := NewConfigurator(Config{
		VerifyIncomingHTTPS: true,
		CAFile:              "../test/ca/root.cer",
		CertFile:            "../test/key/ourdomain.cer",
		KeyFile:             "../test/key/ourdomain.key",
	}, nil)
	require.NoError(t, err)
	tlsConf := c.IncomingHTTPSConfig()
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)
	require.NotNil(t, tlsConf.GetConfigForClient)
	tlsConf, err = tlsConf.GetConfigForClient(
		&tls.ClientHelloInfo{SupportedProtos: []string{"h2"}},
	)
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)
	require.Equal(t, []string{"h2"}, tlsConf.NextProtos)
}

func TestConfigurator_OutgoingTLSConfigForChecks(t *testing.T) {
	c := Configurator{base: &Config{
		TLSMinVersion:           "tls12",
		EnableAgentTLSForChecks: false,
	}}
	tlsConf := c.OutgoingTLSConfigForCheck(true)
	require.Equal(t, true, tlsConf.InsecureSkipVerify)
	require.Equal(t, uint16(0), tlsConf.MinVersion)

	c.base.EnableAgentTLSForChecks = true
	c.base.ServerName = "servername"
	tlsConf = c.OutgoingTLSConfigForCheck(true)
	require.Equal(t, true, tlsConf.InsecureSkipVerify)
	require.Equal(t, TLSLookup[c.base.TLSMinVersion], tlsConf.MinVersion)
	require.Equal(t, c.base.ServerName, tlsConf.ServerName)
}

func TestConfigurator_OutgoingRPCConfig(t *testing.T) {
	c := Configurator{base: &Config{}}
	require.Nil(t, c.OutgoingRPCConfig())
	c.base.VerifyOutgoing = true
	require.NotNil(t, c.OutgoingRPCConfig())
}

func TestConfigurator_OutgoingRPCWrapper(t *testing.T) {
	c := Configurator{base: &Config{}}
	require.Nil(t, c.OutgoingRPCWrapper())
	c.base.VerifyOutgoing = true
	wrap := c.OutgoingRPCWrapper()
	require.NotNil(t, wrap)
	t.Log("TODO: actually call wrap here eventually")
}

func TestConfigurator_UpdateChecks(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)
	require.NoError(t, c.Update(Config{}))
	require.Error(t, c.Update(Config{VerifyOutgoing: true}))
	require.Error(t, c.Update(Config{VerifyIncoming: true, CAFile: "../test/ca/root.cer"}))
	require.False(t, c.base.VerifyIncoming)
	require.False(t, c.base.VerifyOutgoing)
	require.Equal(t, c.version, 2)
}

func TestConfigurator_UpdateSetsStuff(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)
	require.Nil(t, c.cas)
	require.Nil(t, c.cert)
	require.Equal(t, c.base, &Config{})
	require.Equal(t, 1, c.version)

	require.Error(t, c.Update(Config{VerifyOutgoing: true}))
	require.Equal(t, c.version, 1)

	config := Config{
		CAFile:   "../test/ca/root.cer",
		CertFile: "../test/key/ourdomain.cer",
		KeyFile:  "../test/key/ourdomain.key",
	}
	require.NoError(t, c.Update(config))
	require.NotNil(t, c.cas)
	require.Len(t, c.cas.Subjects(), 1)
	require.NotNil(t, c.cert)
	require.Equal(t, c.base, &config)
	require.Equal(t, 2, c.version)
}
