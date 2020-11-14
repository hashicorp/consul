package pool

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/consul/tlsutil"
	"github.com/stretchr/testify/require"
)

func TestPeekForTLS_not_TLS(t *testing.T) {
	type testcase struct {
		name     string
		connData []byte
	}

	var cases []testcase
	for _, rpcType := range []RPCType{
		RPCConsul,
		RPCRaft,
		RPCMultiplex,
		RPCTLS,
		RPCMultiplexV2,
		RPCSnapshot,
		RPCGossip,
		RPCTLSInsecure,
	} {
		cases = append(cases, testcase{
			name:     fmt.Sprintf("tcp rpc type byte %d", rpcType),
			connData: []byte{byte(rpcType), 'h', 'e', 'l', 'l', 'o'},
		})
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dead := time.Now().Add(1 * time.Second)
			serverConn, clientConn, err := deadlineNetPipe(dead)
			require.NoError(t, err)
			go func() {
				_, _ = clientConn.Write(tc.connData)
				_ = clientConn.Close()
			}()
			defer serverConn.Close()

			wrapped, isTLS, err := PeekForTLS(serverConn)
			require.NoError(t, err)
			require.False(t, isTLS)

			all, err := ioutil.ReadAll(wrapped)
			require.NoError(t, err)
			require.Equal(t, tc.connData, all)
		})
	}
}

func TestPeekForTLS_actual_TLS(t *testing.T) {
	type testcase struct {
		name     string
		connData []byte
	}

	var cases []testcase
	for _, rpcType := range []RPCType{
		RPCConsul,
		RPCRaft,
		RPCMultiplex,
		RPCTLS,
		RPCMultiplexV2,
		RPCSnapshot,
		RPCGossip,
		RPCTLSInsecure,
	} {
		cases = append(cases, testcase{
			name:     fmt.Sprintf("tcp rpc type byte %d", rpcType),
			connData: []byte{byte(rpcType), 'h', 'e', 'l', 'l', 'o'},
		})
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testPeekForTLS_withTLS(t, tc.connData)
		})
	}
}

func testPeekForTLS_withTLS(t *testing.T, connData []byte) {
	t.Helper()

	cert, caPEM, err := generateTestCert("server.dc1.consul")
	require.NoError(t, err)

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))

	dead := time.Now().Add(1 * time.Second)
	serverConn, clientConn, err := deadlineNetPipe(dead)
	require.NoError(t, err)

	var (
		clientErrCh      = make(chan error, 1)
		serverErrCh      = make(chan error, 1)
		serverGotPayload []byte
	)
	go func(conn net.Conn) { // Client
		config := &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    roots,
			ServerName: "server.dc1.consul",
			NextProtos: []string{"foo/bar"},
		}

		tlsConn := tls.Client(conn, config)
		defer tlsConn.Close()

		if err := tlsConn.Handshake(); err != nil {
			clientErrCh <- err
			return
		}

		_, err = tlsConn.Write(connData)
		clientErrCh <- err
	}(clientConn)

	go func(conn net.Conn) { // Server
		defer conn.Close()

		wrapped, isTLS, err := PeekForTLS(conn)
		if err != nil {
			serverErrCh <- err
			return
		} else if !isTLS {
			serverErrCh <- errors.New("expected to have peeked TLS but did not")
			return
		}

		config := &tls.Config{
			MinVersion:   tls.VersionTLS12,
			RootCAs:      roots,
			Certificates: []tls.Certificate{cert},
			ServerName:   "server.dc1.consul",
			NextProtos:   []string{"foo/bar"},
		}

		tlsConn := tls.Server(wrapped, config)
		defer tlsConn.Close()

		if err := tlsConn.Handshake(); err != nil {
			serverErrCh <- err
			return
		}

		all, err := ioutil.ReadAll(tlsConn)
		if err != nil {
			serverErrCh <- err
			return
		}

		serverGotPayload = all
		serverErrCh <- nil
	}(serverConn)

	require.NoError(t, <-clientErrCh)
	require.NoError(t, <-serverErrCh)

	require.Equal(t, connData, serverGotPayload)
}

func deadlineNetPipe(deadline time.Time) (net.Conn, net.Conn, error) {
	server, client := net.Pipe()

	if err := server.SetDeadline(deadline); err != nil {
		server.Close()
		client.Close()
		return nil, nil, err
	}
	if err := client.SetDeadline(deadline); err != nil {
		server.Close()
		client.Close()
		return nil, nil, err
	}

	return server, client, nil
}

func generateTestCert(serverName string) (cert tls.Certificate, caPEM []byte, err error) {
	// generate CA
	serial, err := tlsutil.GenerateSerialNumber()
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	signer, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	ca, err := tlsutil.GenerateCA(signer, serial, 365, nil)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	// generate leaf
	serial, err = tlsutil.GenerateSerialNumber()
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	certificate, privateKey, err := tlsutil.GenerateCert(
		signer,
		ca,
		serial,
		"Test Cert Name",
		365,
		[]string{serverName},
		nil,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	cert, err = tls.X509KeyPair([]byte(certificate), []byte(privateKey))
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	return cert, []byte(ca), nil
}
