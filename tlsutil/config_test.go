package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func startRPCTLSServer(config *Config) (net.Conn, chan error) {
	return startTLSServer(config, nil, false)
}

func startALPNRPCTLSServer(config *Config, alpnProtos []string) (net.Conn, chan error) {
	return startTLSServer(config, alpnProtos, true)
}

func startTLSServer(config *Config, alpnProtos []string, doAlpnVariant bool) (net.Conn, chan error) {
	errc := make(chan error, 1)

	c, err := NewConfigurator(*config, nil)
	if err != nil {
		errc <- err
		return nil, errc
	}
	var tlsConfigServer *tls.Config
	if doAlpnVariant {
		tlsConfigServer = c.IncomingALPNRPCConfig(alpnProtos)
	} else {
		tlsConfigServer = c.IncomingRPCConfig()
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
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	config := Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Alice.crt",
		KeyFile:              "../test/hostname/Alice.key",
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		Domain:               "consul",
	}

	client, errc := startRPCTLSServer(&config)
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
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	config := Config{
		VerifyOutgoing: true,
		CAFile:         "../test/hostname/CertAuth.crt",
		CertFile:       "../test/hostname/Alice.crt",
		KeyFile:        "../test/hostname/Alice.key",
		Domain:         "consul",
	}

	client, errc := startRPCTLSServer(&config)
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
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	config := Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Alice.crt",
		KeyFile:              "../test/hostname/Alice.key",
		VerifyServerHostname: true,
		VerifyOutgoing:       true,
		Domain:               "consul",
	}

	client, errc := startRPCTLSServer(&config)
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

	client, errc := startRPCTLSServer(&config)
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

func TestConfigurator_outgoingWrapperALPN_OK(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	config := Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Bob.crt",
		KeyFile:              "../test/hostname/Bob.key",
		VerifyServerHostname: false, // doesn't matter
		VerifyOutgoing:       false, // doesn't matter
		Domain:               "consul",
	}

	client, errc := startALPNRPCTLSServer(&config, []string{"foo", "bar"})
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)
	wrap := c.OutgoingALPNRPCWrapper()
	require.NotNil(t, wrap)

	tlsClient, err := wrap("dc1", "bob", "foo", client)
	require.NoError(t, err)
	defer tlsClient.Close()

	tlsConn := tlsClient.(*tls.Conn)
	cs := tlsConn.ConnectionState()
	require.Equal(t, "foo", cs.NegotiatedProtocol)

	err = <-errc
	require.NoError(t, err)
}

func TestConfigurator_outgoingWrapperALPN_serverHasNoNodeNameInSAN(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	srvConfig := Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Alice.crt",
		KeyFile:              "../test/hostname/Alice.key",
		VerifyServerHostname: false, // doesn't matter
		VerifyOutgoing:       false, // doesn't matter
		Domain:               "consul",
	}

	client, errc := startALPNRPCTLSServer(&srvConfig, []string{"foo", "bar"})
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	config := Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Bob.crt",
		KeyFile:              "../test/hostname/Bob.key",
		VerifyServerHostname: false, // doesn't matter
		VerifyOutgoing:       false, // doesn't matter
		Domain:               "consul",
	}

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)
	wrap := c.OutgoingALPNRPCWrapper()
	require.NotNil(t, wrap)

	_, err = wrap("dc1", "bob", "foo", client)
	require.Error(t, err)
	_, ok := err.(x509.HostnameError)
	require.True(t, ok)
	client.Close()

	<-errc
}

func TestConfigurator_outgoingWrapperALPN_BadDC(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	config := Config{
		CAFile:               "../test/hostname/CertAuth.crt",
		CertFile:             "../test/hostname/Bob.crt",
		KeyFile:              "../test/hostname/Bob.key",
		VerifyServerHostname: false, // doesn't matter
		VerifyOutgoing:       false, // doesn't matter
		Domain:               "consul",
	}

	client, errc := startALPNRPCTLSServer(&config, []string{"foo", "bar"})
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)
	wrap := c.OutgoingALPNRPCWrapper()

	_, err = wrap("dc2", "bob", "foo", client)
	require.Error(t, err)
	_, ok := err.(x509.HostnameError)
	require.True(t, ok)
	client.Close()

	<-errc
}

func TestConfigurator_outgoingWrapperALPN_BadCert(t *testing.T) {
	config := Config{
		CAFile:               "../test/ca/root.cer",
		CertFile:             "../test/key/ourdomain.cer",
		KeyFile:              "../test/key/ourdomain.key",
		VerifyServerHostname: false, // doesn't matter
		VerifyOutgoing:       false, // doesn't matter
		Domain:               "consul",
	}

	client, errc := startALPNRPCTLSServer(&config, []string{"foo", "bar"})
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)
	wrap := c.OutgoingALPNRPCWrapper()

	_, err = wrap("dc1", "bob", "foo", client)
	require.Error(t, err)
	_, ok := err.(x509.HostnameError)
	require.True(t, ok)
	client.Close()

	<-errc
}

func TestConfigurator_wrapTLS_OK(t *testing.T) {
	config := Config{
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
		VerifyOutgoing: true,
	}

	client, errc := startRPCTLSServer(&config)
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

	client, errc := startRPCTLSServer(serverConfig)
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
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	}, ",")
	ciphers := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
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

func TestLoadKeyPair(t *testing.T) {
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
	for i, v := range variants {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			cert, err := loadKeyPair(v.cert, v.key)
			if v.shoulderr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if v.isnil {
				require.Nil(t, cert)
			} else {
				require.NotNil(t, cert)
			}
		})
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
	logger := testutil.Logger(t)
	c, err := NewConfigurator(Config{}, logger)
	require.NoError(t, err)
	require.NotNil(t, c)

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
		{Config{}, false, false},                                              // 1
		{Config{TLSMinVersion: "tls9"}, true, false},                          // 1
		{Config{TLSMinVersion: ""}, false, false},                             // 2
		{Config{VerifyOutgoing: true, CAFile: "", CAPath: ""}, true, false},   // 6
		{Config{VerifyOutgoing: false, CAFile: "", CAPath: ""}, false, false}, // 7
		{Config{VerifyOutgoing: false, CAFile: cafile, CAPath: ""},
			false, false}, // 8
		{Config{VerifyOutgoing: false, CAFile: "", CAPath: capath},
			false, false}, // 9
		{Config{VerifyOutgoing: false, CAFile: cafile, CAPath: capath},
			false, false}, // 10
		{Config{VerifyOutgoing: true, CAFile: cafile, CAPath: ""},
			false, false}, // 11
		{Config{VerifyOutgoing: true, CAFile: "", CAPath: capath},
			false, false}, // 12
		{Config{VerifyOutgoing: true, CAFile: cafile, CAPath: capath},
			false, false}, // 13
		{Config{VerifyIncoming: true, CAFile: "", CAPath: ""}, true, false}, // 14
		{Config{VerifyIncomingRPC: true, CAFile: "", CAPath: ""},
			true, false}, // 15
		{Config{VerifyIncomingHTTPS: true, CAFile: "", CAPath: ""},
			true, false}, // 16
		{Config{VerifyIncoming: true, CAFile: cafile, CAPath: ""}, true, false}, // 17
		{Config{VerifyIncoming: true, CAFile: "", CAPath: capath}, true, false}, // 18
		{Config{VerifyIncoming: true, CAFile: "", CAPath: capath,
			CertFile: certfile, KeyFile: keyfile}, false, false}, // 19
		{Config{CertFile: "bogus", KeyFile: "bogus"}, true, true},                   // 20
		{Config{CAFile: "bogus"}, true, true},                                       // 21
		{Config{CAPath: "bogus"}, true, true},                                       // 22
		{Config{VerifyIncoming: true, CAFile: cafile, AutoTLS: true}, false, false}, // 22
	}
	for _, v := range tlsVersions() {
		variants = append(variants, variant{Config{TLSMinVersion: v}, false, false})
	}

	c := Configurator{}
	for i, v := range variants {
		info := fmt.Sprintf("case %d, config: %+v", i, v.config)
		_, err1 := NewConfigurator(v.config, nil)
		err2 := c.Update(v.config)

		var err3 error
		if !v.excludeCheck {
			cert, err := loadKeyPair(v.config.CertFile, v.config.KeyFile)
			require.NoError(t, err, info)
			pems, err := LoadCAs(v.config.CAFile, v.config.CAPath)
			require.NoError(t, err, info)
			pool, err := newX509CertPool(pems)
			require.NoError(t, err, info)
			err3 = validateConfig(v.config, pool, cert)
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

func TestConfigurator_LoadCAs(t *testing.T) {
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
		pems, err1 := LoadCAs(v.cafile, v.capath)
		pool, err2 := newX509CertPool(pems)
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

	cert, err := c.commonTLSConfig(false).GetClientCertificate(nil)
	require.NoError(t, err)
	require.NotNil(t, cert)
	require.Empty(t, cert.Certificate)

	c1, err := loadKeyPair("../test/key/something_expired.cer", "../test/key/something_expired.key")
	require.NoError(t, err)
	c.manual.cert = c1
	cert, err = c.commonTLSConfig(false).GetClientCertificate(nil)
	require.NoError(t, err)
	require.Equal(t, c.manual.cert, cert)

	c2, err := loadKeyPair("../test/key/ourdomain.cer", "../test/key/ourdomain.key")
	require.NoError(t, err)
	c.autoTLS.cert = c2
	cert, err = c.commonTLSConfig(false).GetClientCertificate(nil)
	require.NoError(t, err)
	require.Equal(t, c.autoTLS.cert, cert)
}

func TestConfigurator_CommonTLSConfigGetCertificate(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)

	cert, err := c.commonTLSConfig(false).GetCertificate(nil)
	require.NoError(t, err)
	require.Nil(t, cert)

	// Setting a certificate as the auto-encrypt cert will return it as the regular server certificate
	c1, err := loadKeyPair("../test/key/something_expired.cer", "../test/key/something_expired.key")
	require.NoError(t, err)
	c.autoTLS.cert = c1
	cert, err = c.commonTLSConfig(false).GetCertificate(nil)
	require.NoError(t, err)
	require.Equal(t, c.autoTLS.cert, cert)

	// Setting a different certificate as a manual cert will override the auto-encrypt cert and instead return the manual cert
	c2, err := loadKeyPair("../test/key/ourdomain.cer", "../test/key/ourdomain.key")
	require.NoError(t, err)
	c.manual.cert = c2
	cert, err = c.commonTLSConfig(false).GetCertificate(nil)
	require.NoError(t, err)
	require.Equal(t, c.manual.cert, cert)
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
	require.Equal(t, c.commonTLSConfig(false).MinVersion, tlsLookup["tls10"])

	for _, version := range tlsVersions() {
		require.NoError(t, c.Update(Config{TLSMinVersion: version}))
		require.Equal(t, c.commonTLSConfig(false).MinVersion,
			tlsLookup[version])
	}

	require.Error(t, c.Update(Config{TLSMinVersion: "tlsBOGUS"}))
}

func TestConfigurator_CommonTLSConfigVerifyIncoming(t *testing.T) {
	c := Configurator{base: &Config{}}
	type variant struct {
		verify   bool
		expected tls.ClientAuthType
	}
	variants := []variant{
		{true, tls.RequireAndVerifyClientCert},
		{false, tls.NoClientCert},
	}
	for _, v := range variants {
		require.Equal(t, v.expected, c.commonTLSConfig(v.verify).ClientAuth)
	}
}

func TestConfigurator_OutgoingRPCTLSDisabled(t *testing.T) {
	c := Configurator{base: &Config{}}
	type variant struct {
		verify         bool
		autoEncryptTLS bool
		pool           *x509.CertPool
		expected       bool
	}
	variants := []variant{
		{false, false, nil, false},
		{true, false, nil, true},
		{false, true, nil, true},
		{true, true, nil, true},

		{true, false, &x509.CertPool{}, true},
		{false, true, &x509.CertPool{}, true},
		{true, true, &x509.CertPool{}, true},
	}
	for i, v := range variants {
		info := fmt.Sprintf("case %d", i)
		c.caPool = v.pool
		c.base.VerifyOutgoing = v.verify
		c.base.AutoTLS = v.autoEncryptTLS
		require.Equal(t, v.expected, c.outgoingRPCTLSEnabled(), info)
	}
}

func TestConfigurator_MutualTLSCapable(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	t.Run("no ca", func(t *testing.T) {
		config := Config{
			Domain: "consul",
		}
		c, err := NewConfigurator(config, nil)
		require.NoError(t, err)

		require.False(t, c.MutualTLSCapable())
	})

	t.Run("ca and no keys", func(t *testing.T) {
		config := Config{
			CAFile: "../test/hostname/CertAuth.crt",
			Domain: "consul",
		}
		c, err := NewConfigurator(config, nil)
		require.NoError(t, err)

		require.False(t, c.MutualTLSCapable())
	})

	t.Run("ca and manual key", func(t *testing.T) {
		config := Config{
			CAFile:   "../test/hostname/CertAuth.crt",
			CertFile: "../test/hostname/Bob.crt",
			KeyFile:  "../test/hostname/Bob.key",
			Domain:   "consul",
		}
		c, err := NewConfigurator(config, nil)
		require.NoError(t, err)

		require.True(t, c.MutualTLSCapable())
	})

	loadFile := func(t *testing.T, path string) string {
		data, err := ioutil.ReadFile(path)
		require.NoError(t, err)
		return string(data)
	}

	t.Run("autoencrypt ca and no autoencrypt keys", func(t *testing.T) {
		config := Config{
			Domain: "consul",
		}
		c, err := NewConfigurator(config, nil)
		require.NoError(t, err)

		caPEM := loadFile(t, "../test/hostname/CertAuth.crt")
		require.NoError(t, c.UpdateAutoTLSCA([]string{caPEM}))

		require.False(t, c.MutualTLSCapable())
	})

	t.Run("autoencrypt ca and autoencrypt key", func(t *testing.T) {
		config := Config{
			Domain: "consul",
		}
		c, err := NewConfigurator(config, nil)
		require.NoError(t, err)

		caPEM := loadFile(t, "../test/hostname/CertAuth.crt")
		certPEM := loadFile(t, "../test/hostname/Bob.crt")
		keyPEM := loadFile(t, "../test/hostname/Bob.key")
		require.NoError(t, c.UpdateAutoTLSCA([]string{caPEM}))
		require.NoError(t, c.UpdateAutoTLSCert(certPEM, keyPEM))

		require.True(t, c.MutualTLSCapable())
	})
}

func TestConfigurator_UpdateAutoTLSCA_DoesNotPanic(t *testing.T) {
	config := Config{
		Domain: "consul",
	}
	c, err := NewConfigurator(config, hclog.New(nil))
	require.NoError(t, err)

	err = c.UpdateAutoTLSCA([]string{"invalid pem"})
	require.Error(t, err)
}

func TestConfigurator_VerifyIncomingRPC(t *testing.T) {
	c := Configurator{base: &Config{
		VerifyIncomingRPC: true,
	}}
	verify := c.VerifyIncomingRPC()
	require.Equal(t, c.base.VerifyIncomingRPC, verify)
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
	require.Empty(t, tlsConf.NextProtos)
	require.Empty(t, tlsConf.ServerName)

	require.NotNil(t, tlsConf.GetConfigForClient)
	tlsConf, err = tlsConf.GetConfigForClient(nil)
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)
	require.Empty(t, tlsConf.NextProtos)
	require.Empty(t, tlsConf.ServerName)
}

func TestConfigurator_IncomingALPNRPCConfig(t *testing.T) {
	c, err := NewConfigurator(Config{
		VerifyIncomingRPC: false, // ignored, assumed true
		CAFile:            "../test/ca/root.cer",
		CertFile:          "../test/key/ourdomain.cer",
		KeyFile:           "../test/key/ourdomain.key",
	}, nil)
	require.NoError(t, err)
	tlsConf := c.IncomingALPNRPCConfig([]string{"foo/1", "bar/2"})
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)
	require.False(t, tlsConf.InsecureSkipVerify)
	require.Equal(t, []string{"foo/1", "bar/2"}, tlsConf.NextProtos)
	require.Empty(t, tlsConf.ServerName)

	require.NotNil(t, tlsConf.GetConfigForClient)
	tlsConf, err = tlsConf.GetConfigForClient(nil)
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)
	require.False(t, tlsConf.InsecureSkipVerify)
	require.Equal(t, []string{"foo/1", "bar/2"}, tlsConf.NextProtos)
	require.Empty(t, tlsConf.ServerName)
}

func TestConfigurator_IncomingHTTPSConfig(t *testing.T) {

	// compare tls.Config.GetConfigForClient by nil/not-nil, since Go can not compare
	// functions any other way.
	cmpClientFunc := cmp.Comparer(func(x, y func(*tls.ClientHelloInfo) (*tls.Config, error)) bool {
		return (x == nil && y == nil) || (x != nil && y != nil)
	})

	t.Run("default", func(t *testing.T) {
		c, err := NewConfigurator(Config{}, nil)
		require.NoError(t, err)

		cfg := c.IncomingHTTPSConfig()

		expected := &tls.Config{
			NextProtos:         []string{"h2", "http/1.1"},
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS10,
			GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
				return nil, nil
			},
		}
		assertDeepEqual(t, expected, cfg, cmpTLSConfig, cmpClientFunc)
	})

	t.Run("verify incoming", func(t *testing.T) {
		c := Configurator{base: &Config{VerifyIncoming: true}}

		cfg := c.IncomingHTTPSConfig()

		expected := &tls.Config{
			NextProtos:         []string{"h2", "http/1.1"},
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS10,
			GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
				return nil, nil
			},
			ClientAuth: tls.RequireAndVerifyClientCert,
		}
		assertDeepEqual(t, expected, cfg, cmpTLSConfig, cmpClientFunc)
	})

}

var cmpTLSConfig = cmp.Options{
	cmpopts.IgnoreFields(tls.Config{}, "GetCertificate", "GetClientCertificate"),
	cmpopts.IgnoreUnexported(tls.Config{}),
}

func TestConfigurator_OutgoingTLSConfigForCheck(t *testing.T) {
	type testCase struct {
		name       string
		conf       func() (*Configurator, error)
		skipVerify bool
		serverName string
		expected   *tls.Config
	}

	run := func(t *testing.T, tc testCase) {
		configurator, err := tc.conf()
		require.NoError(t, err)
		c := configurator.OutgoingTLSConfigForCheck(tc.skipVerify, tc.serverName)
		assertDeepEqual(t, tc.expected, c, cmpTLSConfig)
	}

	testCases := []testCase{
		{
			name: "default tls",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{}, nil)
			},
			expected: &tls.Config{},
		},
		{
			name: "default tls, skip verify, no server name",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{
					TLSMinVersion:           "tls12",
					EnableAgentTLSForChecks: false,
				}, nil)
			},
			skipVerify: true,
			expected:   &tls.Config{InsecureSkipVerify: true},
		},
		{
			name: "default tls, skip verify, default server name",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{
					TLSMinVersion:           "tls12",
					EnableAgentTLSForChecks: false,
					ServerName:              "servername",
					NodeName:                "nodename",
				}, nil)
			},
			skipVerify: true,
			expected:   &tls.Config{InsecureSkipVerify: true},
		},
		{
			name: "default tls, skip verify, check server name",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{
					TLSMinVersion:           "tls12",
					EnableAgentTLSForChecks: false,
					ServerName:              "servername",
				}, nil)
			},
			skipVerify: true,
			serverName: "check-server-name",
			expected: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         "check-server-name",
			},
		},
		{
			name: "agent tls, default server name",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{
					TLSMinVersion:           "tls12",
					EnableAgentTLSForChecks: true,
					NodeName:                "nodename",
					ServerName:              "servername",
				}, nil)
			},
			expected: &tls.Config{
				MinVersion: tls.VersionTLS12,
				ServerName: "servername",
			},
		},
		{
			name: "agent tls, skip verify, node name for server name",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{
					TLSMinVersion:           "tls12",
					EnableAgentTLSForChecks: true,
					NodeName:                "nodename",
				}, nil)
			},
			skipVerify: true,
			expected: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
				ServerName:         "nodename",
			},
		},
		{
			name: "agent tls, skip verify, with server name override",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{
					TLSMinVersion:           "tls12",
					EnableAgentTLSForChecks: true,
					ServerName:              "servername",
				}, nil)
			},
			skipVerify: true,
			serverName: "override",
			expected: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
				ServerName:         "override",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func assertDeepEqual(t *testing.T, x, y interface{}, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(x, y, opts...); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
}

func TestConfigurator_OutgoingRPCConfig(t *testing.T) {
	c := &Configurator{base: &Config{}}
	require.Nil(t, c.OutgoingRPCConfig())

	c, err := NewConfigurator(Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
	}, nil)
	require.NoError(t, err)

	tlsConf := c.OutgoingRPCConfig()
	require.NotNil(t, tlsConf)
	require.Equal(t, tls.NoClientCert, tlsConf.ClientAuth)
	require.True(t, tlsConf.InsecureSkipVerify)
	require.Empty(t, tlsConf.NextProtos)
	require.Empty(t, tlsConf.ServerName)
}

func TestConfigurator_OutgoingALPNRPCConfig(t *testing.T) {
	c := &Configurator{base: &Config{}}
	require.Nil(t, c.outgoingALPNRPCConfig())

	c, err := NewConfigurator(Config{
		VerifyOutgoing: false, // ignored, assumed true
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}, nil)
	require.NoError(t, err)

	tlsConf := c.outgoingALPNRPCConfig()
	require.NotNil(t, tlsConf)
	require.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)
	require.False(t, tlsConf.InsecureSkipVerify)
	require.Empty(t, tlsConf.NextProtos)
	require.Empty(t, tlsConf.ServerName)
}

func TestConfigurator_OutgoingRPCWrapper(t *testing.T) {
	c := &Configurator{base: &Config{}}
	wrapper := c.OutgoingRPCWrapper()
	require.NotNil(t, wrapper)
	conn := &net.TCPConn{}
	cWrap, err := wrapper("", conn)
	require.NoError(t, err)
	require.Equal(t, conn, cWrap)

	c, err = NewConfigurator(Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
	}, nil)
	require.NoError(t, err)

	wrapper = c.OutgoingRPCWrapper()
	require.NotNil(t, wrapper)
	cWrap, err = wrapper("", conn)
	require.EqualError(t, err, "invalid argument")
	require.NotEqual(t, conn, cWrap)
}

func TestConfigurator_OutgoingALPNRPCWrapper(t *testing.T) {
	c := &Configurator{base: &Config{}}
	wrapper := c.OutgoingRPCWrapper()
	require.NotNil(t, wrapper)
	conn := &net.TCPConn{}
	cWrap, err := wrapper("", conn)
	require.NoError(t, err)
	require.Equal(t, conn, cWrap)

	c, err = NewConfigurator(Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
	}, nil)
	require.NoError(t, err)

	wrapper = c.OutgoingRPCWrapper()
	require.NotNil(t, wrapper)
	cWrap, err = wrapper("", conn)
	require.EqualError(t, err, "invalid argument")
	require.NotEqual(t, conn, cWrap)
}

func TestConfigurator_UpdateChecks(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)
	require.NoError(t, c.Update(Config{}))
	require.Error(t, c.Update(Config{VerifyOutgoing: true}))
	require.Error(t, c.Update(Config{VerifyIncoming: true, CAFile: "../test/ca/root.cer"}))
	require.False(t, c.base.VerifyIncoming)
	require.False(t, c.base.VerifyOutgoing)
	require.Equal(t, uint64(2), c.version)
}

func TestConfigurator_UpdateSetsStuff(t *testing.T) {
	c, err := NewConfigurator(Config{}, nil)
	require.NoError(t, err)
	require.Nil(t, c.caPool)
	require.Nil(t, c.manual.cert)
	require.Equal(t, c.base, &Config{})
	require.Equal(t, uint64(1), c.version)

	require.Error(t, c.Update(Config{VerifyOutgoing: true}))
	require.Equal(t, uint64(1), c.version)

	config := Config{
		CAFile:   "../test/ca/root.cer",
		CertFile: "../test/key/ourdomain.cer",
		KeyFile:  "../test/key/ourdomain.key",
	}
	require.NoError(t, c.Update(config))
	require.NotNil(t, c.caPool)
	require.Len(t, c.caPool.Subjects(), 1)
	require.NotNil(t, c.manual.cert)
	require.Equal(t, c.base, &config)
	require.Equal(t, uint64(2), c.version)
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

func TestConfigurator_VerifyOutgoing(t *testing.T) {
	c := Configurator{base: &Config{}}
	type variant struct {
		verify         bool
		autoEncryptTLS bool
		pool           *x509.CertPool
		expected       bool
	}
	variants := []variant{
		{false, false, nil, false},
		{true, false, nil, true},
		{false, true, nil, false},
		{true, true, nil, true},

		{false, false, &x509.CertPool{}, false},
		{true, false, &x509.CertPool{}, true},
		{false, true, &x509.CertPool{}, true},
		{true, true, &x509.CertPool{}, true},
	}
	for i, v := range variants {
		info := fmt.Sprintf("case %d", i)
		c.caPool = v.pool
		c.base.VerifyOutgoing = v.verify
		c.base.AutoTLS = v.autoEncryptTLS
		require.Equal(t, v.expected, c.verifyOutgoing(), info)
	}
}

func TestConfigurator_Domain(t *testing.T) {
	c := Configurator{base: &Config{Domain: "something"}}
	require.Equal(t, "something", c.domain())
}

func TestConfigurator_VerifyServerHostname(t *testing.T) {
	c := Configurator{base: &Config{}}
	require.False(t, c.VerifyServerHostname())

	c.base.VerifyServerHostname = true
	c.autoTLS.verifyServerHostname = false
	require.True(t, c.VerifyServerHostname())

	c.base.VerifyServerHostname = false
	c.autoTLS.verifyServerHostname = true
	require.True(t, c.VerifyServerHostname())

	c.base.VerifyServerHostname = true
	c.autoTLS.verifyServerHostname = true
	require.True(t, c.VerifyServerHostname())
}

func TestConfigurator_AutoEncryptCert(t *testing.T) {
	c := Configurator{base: &Config{}}
	require.Nil(t, c.AutoEncryptCert())

	cert, err := loadKeyPair("../test/key/something_expired.cer", "../test/key/something_expired.key")
	require.NoError(t, err)
	c.autoTLS.cert = cert
	require.Equal(t, int64(1561561551), c.AutoEncryptCert().NotAfter.Unix())

	cert, err = loadKeyPair("../test/key/ourdomain.cer", "../test/key/ourdomain.key")
	require.NoError(t, err)
	c.autoTLS.cert = cert
	require.Equal(t, int64(4679716209), c.AutoEncryptCert().NotAfter.Unix())
}

func TestConfigurator_AuthorizeServerConn(t *testing.T) {
	caPEM, caPK, err := GenerateCA(CAOpts{Days: 5, Domain: "consul"})
	require.NoError(t, err)

	dir := testutil.TempDir(t, "ca")
	caPath := filepath.Join(dir, "ca.pem")
	err = ioutil.WriteFile(caPath, []byte(caPEM), 0600)
	require.NoError(t, err)

	// Cert and key are not used, but required to get past validateConfig
	signer, err := ParseSigner(caPK)
	require.NoError(t, err)
	pub, pk, err := GenerateCert(CertOpts{
		Signer: signer,
		CA:     caPEM,
	})
	require.NoError(t, err)
	certFile := filepath.Join("cert.pem")
	err = ioutil.WriteFile(certFile, []byte(pub), 0600)
	require.NoError(t, err)
	keyFile := filepath.Join("cert.key")
	err = ioutil.WriteFile(keyFile, []byte(pk), 0600)
	require.NoError(t, err)

	cfg := Config{
		VerifyServerHostname: true,
		VerifyIncomingRPC:    true,
		Domain:               "consul",
		CAFile:               caPath,
		CertFile:             certFile,
		KeyFile:              keyFile,
	}
	c, err := NewConfigurator(cfg, hclog.New(nil))
	require.NoError(t, err)

	t.Run("wrong DNSName", func(t *testing.T) {
		signer, err := ParseSigner(caPK)
		require.NoError(t, err)

		pem, _, err := GenerateCert(CertOpts{
			Signer:      signer,
			CA:          caPEM,
			Name:        "server.dc1.consul",
			Days:        5,
			DNSNames:    []string{"this-name-is-wrong", "localhost"},
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
		require.NoError(t, err)

		s := fakeTLSConn{
			state: tls.ConnectionState{
				VerifiedChains:   [][]*x509.Certificate{certChain(t, pem, caPEM)},
				PeerCertificates: certChain(t, pem, caPEM),
			},
		}
		err = c.AuthorizeServerConn("dc1", s)
		testutil.RequireErrorContains(t, err, "is valid for this-name-is-wrong, localhost, not server.dc1.consul")
	})

	t.Run("wrong CA", func(t *testing.T) {
		caPEM, caPK, err := GenerateCA(CAOpts{Days: 5, Domain: "consul"})
		require.NoError(t, err)

		dir := testutil.TempDir(t, "other")
		caPath := filepath.Join(dir, "ca.pem")
		err = ioutil.WriteFile(caPath, []byte(caPEM), 0600)
		require.NoError(t, err)

		signer, err := ParseSigner(caPK)
		require.NoError(t, err)

		pem, _, err := GenerateCert(CertOpts{
			Signer:      signer,
			CA:          caPEM,
			Name:        "server.dc1.consul",
			Days:        5,
			DNSNames:    []string{"server.dc1.consul", "localhost"},
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
		require.NoError(t, err)

		s := fakeTLSConn{
			state: tls.ConnectionState{
				VerifiedChains:   [][]*x509.Certificate{certChain(t, pem, caPEM)},
				PeerCertificates: certChain(t, pem, caPEM),
			},
		}
		err = c.AuthorizeServerConn("dc1", s)
		testutil.RequireErrorContains(t, err, "signed by unknown authority")
	})

	t.Run("missing ext key usage", func(t *testing.T) {
		signer, err := ParseSigner(caPK)
		require.NoError(t, err)

		pem, _, err := GenerateCert(CertOpts{
			Signer:      signer,
			CA:          caPEM,
			Name:        "server.dc1.consul",
			Days:        5,
			DNSNames:    []string{"server.dc1.consul", "localhost"},
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
		})
		require.NoError(t, err)

		s := fakeTLSConn{
			state: tls.ConnectionState{
				VerifiedChains:   [][]*x509.Certificate{certChain(t, pem, caPEM)},
				PeerCertificates: certChain(t, pem, caPEM),
			},
		}
		err = c.AuthorizeServerConn("dc1", s)
		testutil.RequireErrorContains(t, err, "certificate specifies an incompatible key usage")
	})

	t.Run("disabled by verify_incoming_rpc", func(t *testing.T) {
		cfg := Config{
			VerifyServerHostname: true,
			VerifyIncomingRPC:    false,
			Domain:               "consul",
			CAFile:               caPath,
		}
		c, err := NewConfigurator(cfg, hclog.New(nil))
		require.NoError(t, err)

		s := fakeTLSConn{}
		err = c.AuthorizeServerConn("dc1", s)
		require.NoError(t, err)
	})

}

type fakeTLSConn struct {
	state tls.ConnectionState
}

func (f fakeTLSConn) ConnectionState() tls.ConnectionState {
	return f.state
}

func certChain(t *testing.T, certs ...string) []*x509.Certificate {
	t.Helper()

	result := make([]*x509.Certificate, 0, len(certs))

	for i, c := range certs {
		cert, err := parseCert(c)
		require.NoError(t, err, "cert %d", i)
		result = append(result, cert)
	}
	return result
}

func TestConfig_tlsVersions(t *testing.T) {
	require.Equal(t, []string{"tls10", "tls11", "tls12", "tls13"}, tlsVersions())
	expected := "tls10, tls11, tls12, tls13"
	require.Equal(t, expected, strings.Join(tlsVersions(), ", "))
}
