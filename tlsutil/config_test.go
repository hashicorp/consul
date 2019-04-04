package tlsutil

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/require"
)

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

func TestConfigurator_outgoingWrapper_noverify_OK(t *testing.T) {
	config := Config{
		CAFile:   "../test/hostname/CertAuth.crt",
		CertFile: "../test/hostname/Alice.crt",
		KeyFile:  "../test/hostname/Alice.key",
		Domain:   "consul",
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
	require.NoError(t, err)
	if got, want := v, ciphers; !reflect.DeepEqual(got, want) {
		t.Fatalf("got ciphers %#v want %#v", got, want)
	}

	_, err = ParseCiphers("TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,cipherX")
	require.Error(t, err)

	v, err = ParseCiphers("")
	require.NoError(t, err)
	require.Equal(t, []uint16{}, v)
}

func TestConfigurator_loadKeyPair(t *testing.T) {
	type variant struct {
		cert, key string
		shoulderr bool
		isnil     bool
	}
	variants := []variant{
		{"", "", false, true},
		{"bogus", "", false, true},
		{"", "bogus", false, true},
		{"../test/key/ourdomain.cer", "", false, true},
		{"", "../test/key/ourdomain.key", false, true},
		{"bogus", "bogus", true, true},
		{"../test/key/ourdomain.cer", "../test/key/ourdomain.key",
			false, false},
	}
	for _, v := range variants {
		cert1, err1 := loadKeyPair(v.cert, v.key)
		config := &Config{CertFile: v.cert, KeyFile: v.key}
		cert2, err2 := config.KeyPair()
		if v.shoulderr {
			require.Error(t, err1)
			require.Error(t, err2)
		} else {
			require.NoError(t, err1)
			require.NoError(t, err2)
		}
		if v.isnil {
			require.Nil(t, cert1)
			require.Nil(t, cert2)
		} else {
			require.NotNil(t, cert1)
			require.NotNil(t, cert2)
		}
	}
}

func TestConfig_SpecifyDC(t *testing.T) {
	require.Nil(t, SpecificDC("", nil))
	dcwrap := func(dc string, conn net.Conn) (net.Conn, error) { return nil, nil }
	wrap := SpecificDC("", dcwrap)
	require.NotNil(t, wrap)
	conn, err := wrap(nil)
	require.NoError(t, err)
	require.Nil(t, conn)
}

func TestConfigurator_NewConfigurator(t *testing.T) {
	buf := bytes.Buffer{}
	logger := log.New(&buf, "logger: ", log.Lshortfile)
	c, err := NewConfigurator(Config{}, logger)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, logger, c.logger)

	c, err = NewConfigurator(Config{VerifyOutgoing: true}, nil)
	require.Error(t, err)
	require.Nil(t, c)
}

func TestConfigurator_ErrorPropagation(t *testing.T) {
	type variant struct {
		config       Config
		shouldErr    bool
		excludeCheck bool
	}
	cafile := "../test/ca/root.cer"
	capath := "../test/ca_path"
	certfile := "../test/key/ourdomain.cer"
	keyfile := "../test/key/ourdomain.key"
	variants := []variant{
		{Config{}, false, false},
		{Config{TLSMinVersion: "tls9"}, true, false},
		{Config{TLSMinVersion: ""}, false, false},
		{Config{TLSMinVersion: "tls10"}, false, false},
		{Config{TLSMinVersion: "tls11"}, false, false},
		{Config{TLSMinVersion: "tls12"}, false, false},
		{Config{VerifyOutgoing: true, CAFile: "", CAPath: ""}, true, false},
		{Config{VerifyOutgoing: false, CAFile: "", CAPath: ""}, false, false},
		{Config{VerifyOutgoing: false, CAFile: cafile, CAPath: ""},
			false, false},
		{Config{VerifyOutgoing: false, CAFile: "", CAPath: capath},
			false, false},
		{Config{VerifyOutgoing: false, CAFile: cafile, CAPath: capath},
			false, false},
		{Config{VerifyOutgoing: true, CAFile: cafile, CAPath: ""},
			false, false},
		{Config{VerifyOutgoing: true, CAFile: "", CAPath: capath},
			false, false},
		{Config{VerifyOutgoing: true, CAFile: cafile, CAPath: capath},
			false, false},
		{Config{VerifyIncoming: true, CAFile: "", CAPath: ""}, true, false},
		{Config{VerifyIncomingRPC: true, CAFile: "", CAPath: ""},
			true, false},
		{Config{VerifyIncomingHTTPS: true, CAFile: "", CAPath: ""},
			true, false},
		{Config{VerifyIncoming: true, CAFile: cafile, CAPath: ""}, true, false},
		{Config{VerifyIncoming: true, CAFile: "", CAPath: capath}, true, false},
		{Config{VerifyIncoming: true, CAFile: "", CAPath: capath,
			CertFile: certfile, KeyFile: keyfile}, false, false},
		{Config{CertFile: "bogus", KeyFile: "bogus"}, true, true},
		{Config{CAFile: "bogus"}, true, true},
		{Config{CAPath: "bogus"}, true, true},
	}

	c := &Configurator{}
	for i, v := range variants {
		info := fmt.Sprintf("case %d", i)
		_, err1 := NewConfigurator(v.config, nil)
		err2 := c.Update(v.config)

		var err3 error
		if !v.excludeCheck {
			cert, err := v.config.KeyPair()
			require.NoError(t, err, info)
			pems, err := loadCAs(v.config.CAFile, v.config.CAPath)
			require.NoError(t, err, info)
			pool, err := combinedPool(pems, nil)
			require.NoError(t, err, info)
			err3 = c.check(v.config, pool, cert)
		}
		if v.shouldErr {
			require.Error(t, err1, info)
			require.Error(t, err2, info)
			if !v.excludeCheck {
				require.Error(t, err3, info)
			}
		} else {
			require.NoError(t, err1, info)
			require.NoError(t, err2, info)
			if !v.excludeCheck {
				require.NoError(t, err3, info)
			}
		}
	}
}

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

func TestConfigurator_loadCAs(t *testing.T) {
	type variant struct {
		cafile, capath string
		shouldErr      bool
		isNil          bool
		count          int
	}
	variants := []variant{
		{"", "", false, true, 0},
		{"bogus", "", true, true, 0},
		{"", "bogus", true, true, 0},
		{"", "../test/bin", true, true, 0},
		{"../test/ca/root.cer", "", false, false, 1},
		{"", "../test/ca_path", false, false, 2},
		{"../test/ca/root.cer", "../test/ca_path", false, false, 1},
	}
	for i, v := range variants {
		pems, err1 := loadCAs(v.cafile, v.capath)
		pool, err2 := combinedPool(pems, nil)
		info := fmt.Sprintf("case %d", i)
		if v.shouldErr {
			if err1 == nil && err2 == nil {
				t.Fatal("An error is expected but got nil.")
			}
		} else {
			require.NoError(t, err1, info)
			require.NoError(t, err2, info)
		}
		if v.isNil {
			require.Nil(t, pool, info)
		} else {
			require.NotEmpty(t, pems, info)
			require.NotNil(t, pool, info)
			require.Len(t, pool.Subjects(), v.count, info)
			require.Len(t, pems, v.count, info)
		}
	}
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

	conf := Config{CipherSuites: []uint16{
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305}}
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

	c.caPool = &x509.CertPool{}
	require.Equal(t, c.caPool, c.commonTLSConfig(false).ClientCAs)
	require.Equal(t, c.caPool, c.commonTLSConfig(false).RootCAs)
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
	type variant struct {
		verify     bool
		additional bool
		expected   tls.ClientAuthType
	}
	variants := []variant{
		{false, false, tls.NoClientCert},
		{true, false, tls.RequireAndVerifyClientCert},
		{false, true, tls.RequireAndVerifyClientCert},
		{true, true, tls.RequireAndVerifyClientCert},
	}
	for _, v := range variants {
		c.base.VerifyIncoming = v.verify
		require.Equal(t, v.expected,
			c.commonTLSConfig(v.additional).ClientAuth)
	}
}

func TestConfigurator_OutgoingRPCTLSDisabled(t *testing.T) {
	c := Configurator{base: &Config{}}
	type variant struct {
		verify      bool
		file        string
		path        string
		autoEncrypt bool
		expected    bool
	}
	cafile := "../test/ca/root.cer"
	capath := "../test/ca_path"
	variants := []variant{
		{false, "", "", false, true},
		{false, cafile, "", false, false},
		{false, "", capath, false, false},
		{false, cafile, capath, false, false},
		{false, "", "", true, true},
		{false, cafile, "", true, false},
		{false, "", capath, true, false},
		{false, cafile, capath, true, false},
		{true, "", "", false, false},
		{true, cafile, "", false, false},
		{true, "", capath, false, false},
		{true, cafile, capath, false, false},
		{true, "", "", true, false},
		{true, cafile, "", true, false},
		{true, "", capath, true, false},
		{true, cafile, capath, true, false},
	}
	for i, v := range variants {
		info := fmt.Sprintf("case %d", i)
		pems, err := loadCAs(v.file, v.path)
		require.NoError(t, err, info)
		pool, err := combinedPool(pems, nil)
		require.NoError(t, err, info)
		c.caPool = pool
		c.base.VerifyOutgoing = v.verify
		require.Equal(t, v.expected, c.outgoingRPCTLSDisabled(), info)
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
	c := Configurator{base: &Config{}}
	require.Equal(t, []string{"h2", "http/1.1"}, c.IncomingHTTPSConfig().NextProtos)
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
	require.Error(t, c.Update(Config{VerifyIncoming: true,
		CAFile: "../test/ca/root.cer"}))
	require.False(t, c.base.VerifyIncoming)
	require.False(t, c.base.VerifyOutgoing)
	require.Equal(t, c.version, 2)
}

func TestConfigurator_UpdateSetsStuff(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)
	require.Nil(t, c.caPool)
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
	require.NotNil(t, c.caPool)
	require.Len(t, c.caPool.Subjects(), 1)
	require.NotNil(t, c.cert)
	require.Equal(t, c.base, &config)
	require.Equal(t, 2, c.version)
}

func TestConfigurator_ServerNameOrNodeName(t *testing.T) {
	c := Configurator{base: &Config{}}
	type variant struct {
		server, node, expected string
	}
	variants := []variant{
		{"", "", ""},
		{"a", "", "a"},
		{"", "b", "b"},
		{"a", "b", "a"},
	}
	for _, v := range variants {
		c.base.ServerName = v.server
		c.base.NodeName = v.node
		require.Equal(t, v.expected, c.serverNameOrNodeName())
	}
}
