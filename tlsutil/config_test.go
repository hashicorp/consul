// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
//go:build !fips
// +build !fips

package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

func TestConfigurator_IncomingConfig_Common(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	testCases := map[string]struct {
		setupFn  func(ProtocolConfig) Config
		configFn func(*Configurator) *tls.Config
	}{
		"Internal RPC": {
			func(lc ProtocolConfig) Config { return Config{InternalRPC: lc} },
			func(c *Configurator) *tls.Config { return c.IncomingRPCConfig() },
		},
		"gRPC": {
			func(lc ProtocolConfig) Config { return Config{GRPC: lc} },
			func(c *Configurator) *tls.Config { return c.IncomingGRPCConfig() },
		},
		"HTTPS": {
			func(lc ProtocolConfig) Config { return Config{HTTPS: lc} },
			func(c *Configurator) *tls.Config { return c.IncomingHTTPSConfig() },
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			t.Run("MinTLSVersion", func(t *testing.T) {
				cfg := ProtocolConfig{
					TLSMinVersion: "TLSv1_3",
					CertFile:      "../test/hostname/Alice.crt",
					KeyFile:       "../test/hostname/Alice.key",
				}
				c := makeConfigurator(t, tc.setupFn(cfg))

				client, errc, _ := startTLSServer(tc.configFn(c))
				if client == nil {
					t.Fatalf("startTLSServer err: %v", <-errc)
				}

				tlsClient := tls.Client(client, &tls.Config{
					InsecureSkipVerify: true,
					MaxVersion:         tls.VersionTLS12,
				})

				err := tlsClient.Handshake()
				require.Error(t, err)
				require.Contains(t, err.Error(), "version not supported")
			})

			t.Run("CipherSuites", func(t *testing.T) {
				cfg := ProtocolConfig{
					CipherSuites: []types.TLSCipherSuite{types.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
					CertFile:     "../test/hostname/Alice.crt",
					KeyFile:      "../test/hostname/Alice.key",
				}
				c := makeConfigurator(t, tc.setupFn(cfg))

				client, errc, _ := startTLSServer(tc.configFn(c))
				if client == nil {
					t.Fatalf("startTLSServer err: %v", <-errc)
				}

				tlsClient := tls.Client(client, &tls.Config{
					InsecureSkipVerify: true,
					MaxVersion:         tls.VersionTLS12, // TLS 1.3 cipher suites are not configurable.
				})
				require.NoError(t, tlsClient.Handshake())

				cipherSuite := tlsClient.ConnectionState().CipherSuite
				require.Equal(t, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, cipherSuite)
			})

			t.Run("manually configured certificate is preferred over AutoTLS", func(t *testing.T) {
				// Manually configure Alice's certifcate.
				cfg := ProtocolConfig{
					CertFile: "../test/hostname/Alice.crt",
					KeyFile:  "../test/hostname/Alice.key",
				}
				c := makeConfigurator(t, tc.setupFn(cfg))

				// Set Bob's certificate via auto TLS.
				bobCert := loadFile(t, "../test/hostname/Bob.crt")
				bobKey := loadFile(t, "../test/hostname/Bob.key")
				require.NoError(t, c.UpdateAutoTLSCert(bobCert, bobKey))

				client, errc, _ := startTLSServer(tc.configFn(c))
				if client == nil {
					t.Fatalf("startTLSServer err: %v", <-errc)
				}

				// Perform a handshake and check the server presented Alice's certificate.
				tlsClient := tls.Client(client, &tls.Config{InsecureSkipVerify: true})
				require.NoError(t, tlsClient.Handshake())

				certificates := tlsClient.ConnectionState().PeerCertificates
				require.NotEmpty(t, certificates)
				require.Equal(t, "Alice", certificates[0].Subject.CommonName)

				// Check the server side of the handshake succeded.
				require.NoError(t, <-errc)
			})

			t.Run("AutoTLS certificate is presented if no certificate was configured manually", func(t *testing.T) {
				// No manually configured certificate.
				c := makeConfigurator(t, Config{})

				// Set Bob's certificate via auto TLS.
				bobCert := loadFile(t, "../test/hostname/Bob.crt")
				bobKey := loadFile(t, "../test/hostname/Bob.key")
				require.NoError(t, c.UpdateAutoTLSCert(bobCert, bobKey))

				client, errc, _ := startTLSServer(tc.configFn(c))
				if client == nil {
					t.Fatalf("startTLSServer err: %v", <-errc)
				}

				// Perform a handshake and check the server presented Bobs's certificate.
				tlsClient := tls.Client(client, &tls.Config{InsecureSkipVerify: true})
				require.NoError(t, tlsClient.Handshake())

				certificates := tlsClient.ConnectionState().PeerCertificates
				require.NotEmpty(t, certificates)
				require.Equal(t, "Bob", certificates[0].Subject.CommonName)

				// Check the server side of the handshake succeded.
				require.NoError(t, <-errc)
			})

			t.Run("VerifyIncoming enabled - successful handshake", func(t *testing.T) {
				cfg := ProtocolConfig{
					CAFile:         "../test/hostname/CertAuth.crt",
					CertFile:       "../test/hostname/Alice.crt",
					KeyFile:        "../test/hostname/Alice.key",
					VerifyIncoming: true,
				}
				c := makeConfigurator(t, tc.setupFn(cfg))

				client, errc, _ := startTLSServer(tc.configFn(c))
				if client == nil {
					t.Fatalf("startTLSServer err: %v", <-errc)
				}

				tlsClient := tls.Client(client, &tls.Config{
					InsecureSkipVerify: true,
					GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
						cert, err := tls.LoadX509KeyPair("../test/hostname/Bob.crt", "../test/hostname/Bob.key")
						return &cert, err
					},
				})
				require.NoError(t, tlsClient.Handshake())
				require.NoError(t, <-errc)
			})

			t.Run("VerifyIncoming enabled - client provides no certificate", func(t *testing.T) {
				cfg := ProtocolConfig{
					CAFile:         "../test/hostname/CertAuth.crt",
					CertFile:       "../test/hostname/Alice.crt",
					KeyFile:        "../test/hostname/Alice.key",
					VerifyIncoming: true,
				}
				c := makeConfigurator(t, tc.setupFn(cfg))

				client, errc, _ := startTLSServer(tc.configFn(c))
				if client == nil {
					t.Fatalf("startTLSServer err: %v", <-errc)
				}

				tlsClient := tls.Client(client, &tls.Config{InsecureSkipVerify: true})
				require.NoError(t, tlsClient.Handshake())

				err := <-errc
				require.Error(t, err)
				require.Contains(t, err.Error(), "client didn't provide a certificate")
			})

			t.Run("VerifyIncoming enabled - client certificate signed by an unknown CA", func(t *testing.T) {
				cfg := ProtocolConfig{
					CAFile:         "../test/ca/root.cer",
					CertFile:       "../test/hostname/Alice.crt",
					KeyFile:        "../test/hostname/Alice.key",
					VerifyIncoming: true,
				}
				c := makeConfigurator(t, tc.setupFn(cfg))

				client, errc, _ := startTLSServer(tc.configFn(c))
				if client == nil {
					t.Fatalf("startTLSServer err: %v", <-errc)
				}

				tlsClient := tls.Client(client, &tls.Config{
					InsecureSkipVerify: true,
					GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
						cert, err := tls.LoadX509KeyPair("../test/hostname/Bob.crt", "../test/hostname/Bob.key")
						return &cert, err
					},
				})
				require.NoError(t, tlsClient.Handshake())

				err := <-errc
				require.Error(t, err)
				require.Contains(t, err.Error(), "signed by unknown authority")
			})
		})
	}
}

func TestConfigurator_IncomingGRPCConfig_Peering(t *testing.T) {
	// Manually configure Alice's certificates
	cfg := Config{
		GRPC: ProtocolConfig{
			CertFile: "../test/hostname/Alice.crt",
			KeyFile:  "../test/hostname/Alice.key",
		},
	}
	c := makeConfigurator(t, cfg)

	// Set Bob's certificate via auto TLS.
	bobCert := loadFile(t, "../test/hostname/Bob.crt")
	bobKey := loadFile(t, "../test/hostname/Bob.key")
	require.NoError(t, c.UpdateAutoTLSCert(bobCert, bobKey))

	peeringServerName := "server.dc1.peering.1234"
	c.UpdateAutoTLSPeeringServerName(peeringServerName)

	testutil.RunStep(t, "with peering name", func(t *testing.T) {
		client, errc, _ := startTLSServer(c.IncomingGRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}
		tlsClient := tls.Client(client, &tls.Config{
			// When the peering server name is provided the server should present
			// the certificates configured via AutoTLS (Bob).
			ServerName:         peeringServerName,
			InsecureSkipVerify: true,
		})
		require.NoError(t, tlsClient.Handshake())

		certificates := tlsClient.ConnectionState().PeerCertificates
		require.NotEmpty(t, certificates)
		require.Equal(t, "Bob", certificates[0].Subject.CommonName)

		// Check the server side of the handshake succeded.
		require.NoError(t, <-errc)
	})

	testutil.RunStep(t, "without name", func(t *testing.T) {
		client, errc, _ := startTLSServer(c.IncomingGRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		tlsClient := tls.Client(client, &tls.Config{
			// ServerName:         peeringServerName,
			InsecureSkipVerify: true,
		})
		require.NoError(t, tlsClient.Handshake())

		certificates := tlsClient.ConnectionState().PeerCertificates
		require.NotEmpty(t, certificates)

		// Should default to presenting the manually configured certificates.
		require.Equal(t, "Alice", certificates[0].Subject.CommonName)

		// Check the server side of the handshake succeded.
		require.NoError(t, <-errc)
	})
}
func TestConfigurator_IncomingInsecureRPCConfig(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	cfg := Config{
		InternalRPC: ProtocolConfig{
			CAFile:         "../test/hostname/CertAuth.crt",
			CertFile:       "../test/hostname/Alice.crt",
			KeyFile:        "../test/hostname/Alice.key",
			VerifyIncoming: true,
		},
	}

	c := makeConfigurator(t, cfg)

	client, errc, _ := startTLSServer(c.IncomingInsecureRPCConfig())
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	tlsClient := tls.Client(client, &tls.Config{InsecureSkipVerify: true})
	require.NoError(t, tlsClient.Handshake())

	// Check the server side of the handshake succeded.
	require.NoError(t, <-errc)
}

func TestConfigurator_ALPNRPCConfig(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	t.Run("successful protocol negotiation", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/hostname/CertAuth.crt",
				CertFile: "../test/hostname/Bob.crt",
				KeyFile:  "../test/hostname/Bob.key",
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingALPNRPCConfig([]string{"some-protocol"}))
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/hostname/CertAuth.crt",
				CertFile: "../test/hostname/Alice.crt",
				KeyFile:  "../test/hostname/Alice.key",
			},
			Domain: "consul",
		})
		wrap := clientCfg.OutgoingALPNRPCWrapper()

		tlsClient, err := wrap("dc1", "bob", "some-protocol", client)
		require.NoError(t, err)
		defer tlsClient.Close()

		tlsConn := tlsClient.(*tls.Conn)
		require.NoError(t, tlsConn.Handshake())
		require.Equal(t, "some-protocol", tlsConn.ConnectionState().NegotiatedProtocol)

		// Check the server side of the handshake succeded.
		require.NoError(t, <-errc)
	})

	t.Run("protocol negotiation fails", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/hostname/CertAuth.crt",
				CertFile: "../test/hostname/Bob.crt",
				KeyFile:  "../test/hostname/Bob.key",
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingALPNRPCConfig([]string{"some-protocol"}))
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/hostname/CertAuth.crt",
				CertFile: "../test/hostname/Alice.crt",
				KeyFile:  "../test/hostname/Alice.key",
			},
			Domain: "consul",
		})
		wrap := clientCfg.OutgoingALPNRPCWrapper()

		_, err := wrap("dc1", "bob", "other-protocol", client)
		require.Error(t, err)
		require.Error(t, <-errc)
	})

	t.Run("no node name in SAN", func(t *testing.T) {
		// Note: Alice.crt has server.dc1.consul as its SAN (as apposed to alice.server.dc1.consul).
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/hostname/CertAuth.crt",
				CertFile: "../test/hostname/Alice.crt",
				KeyFile:  "../test/hostname/Alice.key",
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingALPNRPCConfig([]string{"some-protocol"}))
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/hostname/CertAuth.crt",
				CertFile: "../test/hostname/Bob.crt",
				KeyFile:  "../test/hostname/Bob.key",
			},
			Domain: "consul",
		})
		wrap := clientCfg.OutgoingALPNRPCWrapper()

		_, err := wrap("dc1", "alice", "some-protocol", client)
		require.Error(t, err)
		require.Error(t, <-errc)
	})

	t.Run("client certificate is always required", func(t *testing.T) {
		cfg := Config{
			InternalRPC: ProtocolConfig{
				VerifyIncoming: false, // this setting is ignored
				CAFile:         "../test/hostname/CertAuth.crt",
				CertFile:       "../test/hostname/Alice.crt",
				KeyFile:        "../test/hostname/Alice.key",
			},
		}
		c := makeConfigurator(t, cfg)

		client, errc, _ := startTLSServer(c.IncomingALPNRPCConfig([]string{"some-protocol"}))
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		tlsClient := tls.Client(client, &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"some-protocol"},
		})
		require.NoError(t, tlsClient.Handshake())

		err := <-errc
		require.Error(t, err)
		require.Contains(t, err.Error(), "client didn't provide a certificate")
	})

	t.Run("bad DC", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/hostname/CertAuth.crt",
				CertFile: "../test/hostname/Alice.crt",
				KeyFile:  "../test/hostname/Alice.key",
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingALPNRPCConfig([]string{"some-protocol"}))
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/hostname/CertAuth.crt",
				CertFile: "../test/hostname/Bob.crt",
				KeyFile:  "../test/hostname/Bob.key",
			},
			Domain: "consul",
		})
		wrap := clientCfg.OutgoingALPNRPCWrapper()

		_, err := wrap("dc2", "*", "some-protocol", client)
		require.Error(t, err)
		require.Error(t, <-errc)
	})
}

func TestConfigurator_OutgoingRPC_ServerMode(t *testing.T) {
	type testCase struct {
		clientConfig Config
		expectName   string
	}

	run := func(t *testing.T, tc testCase) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:         "../test/hostname/CertAuth.crt",
				CertFile:       "../test/hostname/Alice.crt",
				KeyFile:        "../test/hostname/Alice.key",
				VerifyIncoming: true,
			},
			ServerMode: true,
		})

		serverConn, errc, certc := startTLSServer(serverCfg.IncomingRPCConfig())
		if serverConn == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, tc.clientConfig)

		bettyCert := loadFile(t, "../test/hostname/Betty.crt")
		bettyKey := loadFile(t, "../test/hostname/Betty.key")
		require.NoError(t, clientCfg.UpdateAutoTLSCert(bettyCert, bettyKey))

		wrap := clientCfg.OutgoingRPCWrapper()
		require.NotNil(t, wrap)

		tlsClient, err := wrap("dc1", serverConn)
		require.NoError(t, err)
		defer tlsClient.Close()

		err = tlsClient.(*tls.Conn).Handshake()
		require.NoError(t, err)

		err = <-errc
		require.NoError(t, err)

		clientCerts := <-certc
		require.NotEmpty(t, clientCerts)

		require.Equal(t, tc.expectName, clientCerts[0].Subject.CommonName)

		// Check the server side of the handshake succeeded.
		require.NoError(t, <-errc)
	}

	tt := map[string]testCase{
		"server with manual cert": {
			clientConfig: Config{
				InternalRPC: ProtocolConfig{
					VerifyOutgoing: true,
					CAFile:         "../test/hostname/CertAuth.crt",
					CertFile:       "../test/hostname/Bob.crt",
					KeyFile:        "../test/hostname/Bob.key",
				},
				ServerMode: true,
			},
			// Even though an AutoTLS cert is configured, the server will prefer the manually configured cert.
			expectName: "Bob",
		},
		"client with manual cert": {
			clientConfig: Config{
				InternalRPC: ProtocolConfig{
					VerifyOutgoing: true,
					CAFile:         "../test/hostname/CertAuth.crt",
					CertFile:       "../test/hostname/Bob.crt",
					KeyFile:        "../test/hostname/Bob.key",
				},
				ServerMode: false,
			},
			expectName: "Betty",
		},
		"client with auto-TLS": {
			clientConfig: Config{
				ServerMode: false,
				AutoTLS:    true,
			},
			expectName: "Betty",
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestConfigurator_OutgoingInternalRPCWrapper(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	t.Run("AutoTLS", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:         "../test/hostname/CertAuth.crt",
				CertFile:       "../test/hostname/Alice.crt",
				KeyFile:        "../test/hostname/Alice.key",
				VerifyIncoming: true,
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			AutoTLS: true,
		})
		bobCert := loadFile(t, "../test/hostname/Bob.crt")
		bobKey := loadFile(t, "../test/hostname/Bob.key")
		require.NoError(t, clientCfg.UpdateAutoTLSCert(bobCert, bobKey))

		wrap := clientCfg.OutgoingRPCWrapper()
		require.NotNil(t, wrap)

		tlsClient, err := wrap("dc1", client)
		require.NoError(t, err)
		defer tlsClient.Close()

		err = tlsClient.(*tls.Conn).Handshake()
		require.NoError(t, err)

		err = <-errc
		require.NoError(t, err)
	})

	t.Run("VerifyOutgoing and a manually configured certificate", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:         "../test/hostname/CertAuth.crt",
				CertFile:       "../test/hostname/Alice.crt",
				KeyFile:        "../test/hostname/Alice.key",
				VerifyIncoming: true,
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				VerifyOutgoing: true,
				CAFile:         "../test/hostname/CertAuth.crt",
				CertFile:       "../test/hostname/Bob.crt",
				KeyFile:        "../test/hostname/Bob.key",
			},
		})

		wrap := clientCfg.OutgoingRPCWrapper()
		require.NotNil(t, wrap)

		tlsClient, err := wrap("dc1", client)
		require.NoError(t, err)
		defer tlsClient.Close()

		err = tlsClient.(*tls.Conn).Handshake()
		require.NoError(t, err)

		err = <-errc
		require.NoError(t, err)
	})

	t.Run("outgoing TLS not enabled", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:         "../test/hostname/CertAuth.crt",
				CertFile:       "../test/hostname/Alice.crt",
				KeyFile:        "../test/hostname/Alice.key",
				VerifyIncoming: true,
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{})

		wrap := clientCfg.OutgoingRPCWrapper()
		require.NotNil(t, wrap)

		client, err := wrap("dc1", client)
		require.NoError(t, err)
		defer client.Close()

		_, isTLS := client.(*tls.Conn)
		require.False(t, isTLS)
	})

	t.Run("VerifyServerHostname = true", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/client_certs/rootca.crt",
				CertFile: "../test/client_certs/client.crt",
				KeyFile:  "../test/client_certs/client.key",
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				VerifyOutgoing:       true,
				VerifyServerHostname: true,
				CAFile:               "../test/client_certs/rootca.crt",
				CertFile:             "../test/client_certs/client.crt",
				KeyFile:              "../test/client_certs/client.key",
			},
			Domain: "consul",
		})

		wrap := clientCfg.OutgoingRPCWrapper()
		require.NotNil(t, wrap)

		tlsClient, err := wrap("dc1", client)
		require.NoError(t, err)
		defer tlsClient.Close()

		err = tlsClient.(*tls.Conn).Handshake()
		require.Error(t, err)
		require.Regexp(t, `certificate is valid for ([a-z].+) not server.dc1.consul`, err.Error())
	})

	t.Run("VerifyServerHostname = true and incorrect DC name", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/client_certs/rootca.crt",
				CertFile: "../test/client_certs/client.crt",
				KeyFile:  "../test/client_certs/client.key",
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				VerifyServerHostname: true,
				VerifyOutgoing:       true,
				CAFile:               "../test/client_certs/rootca.crt",
				CertFile:             "../test/client_certs/client.crt",
				KeyFile:              "../test/client_certs/client.key",
			},
			Domain: "consul",
		})

		wrap := clientCfg.OutgoingRPCWrapper()
		require.NotNil(t, wrap)

		tlsClient, err := wrap("dc2", client)
		require.NoError(t, err)
		defer tlsClient.Close()

		err = tlsClient.(*tls.Conn).Handshake()
		require.Error(t, err)
		require.Regexp(t, `certificate is valid for ([a-z].+) not server.dc2.consul`, err.Error())
	})

	t.Run("VerifyServerHostname = false", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/client_certs/rootca.crt",
				CertFile: "../test/client_certs/client.crt",
				KeyFile:  "../test/client_certs/client.key",
			},
		})

		client, errc, _ := startTLSServer(serverCfg.IncomingRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				VerifyServerHostname: false,
				VerifyOutgoing:       true,
				CAFile:               "../test/client_certs/rootca.crt",
				CertFile:             "../test/client_certs/client.crt",
				KeyFile:              "../test/client_certs/client.key",
			},
			Domain: "other",
		})

		wrap := clientCfg.OutgoingRPCWrapper()
		require.NotNil(t, wrap)

		tlsClient, err := wrap("dc1", client)
		require.NoError(t, err)
		defer tlsClient.Close()

		err = tlsClient.(*tls.Conn).Handshake()
		require.NoError(t, err)

		// Check the server side of the handshake succeded.
		require.NoError(t, <-errc)
	})

	t.Run("AutoTLS certificate preferred over manually configured certificate", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:         "../test/hostname/CertAuth.crt",
				CertFile:       "../test/hostname/Alice.crt",
				KeyFile:        "../test/hostname/Alice.key",
				VerifyIncoming: true,
			},
		})

		client, errc, certc := startTLSServer(serverCfg.IncomingRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				VerifyServerHostname: true,
				VerifyOutgoing:       true,
				CAFile:               "../test/hostname/CertAuth.crt",
				CertFile:             "../test/hostname/Bob.crt",
				KeyFile:              "../test/hostname/Bob.key",
			},
			Domain: "consul",
		})

		bettyCert := loadFile(t, "../test/hostname/Betty.crt")
		bettyKey := loadFile(t, "../test/hostname/Betty.key")
		require.NoError(t, clientCfg.UpdateAutoTLSCert(bettyCert, bettyKey))

		wrap := clientCfg.OutgoingRPCWrapper()
		require.NotNil(t, wrap)

		tlsClient, err := wrap("dc1", client)
		require.NoError(t, err)
		defer tlsClient.Close()

		err = tlsClient.(*tls.Conn).Handshake()
		require.NoError(t, err)

		err = <-errc
		require.NoError(t, err)

		clientCerts := <-certc
		require.NotEmpty(t, clientCerts)
		require.Equal(t, "Betty", clientCerts[0].Subject.CommonName)
	})

	t.Run("manually configured certificate is presented if there's no AutoTLS certificate", func(t *testing.T) {
		serverCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				CAFile:         "../test/hostname/CertAuth.crt",
				CertFile:       "../test/hostname/Alice.crt",
				KeyFile:        "../test/hostname/Alice.key",
				VerifyIncoming: true,
			},
		})

		client, errc, certc := startTLSServer(serverCfg.IncomingRPCConfig())
		if client == nil {
			t.Fatalf("startTLSServer err: %v", <-errc)
		}

		clientCfg := makeConfigurator(t, Config{
			InternalRPC: ProtocolConfig{
				VerifyServerHostname: true,
				VerifyOutgoing:       true,
				CAFile:               "../test/hostname/CertAuth.crt",
				CertFile:             "../test/hostname/Bob.crt",
				KeyFile:              "../test/hostname/Bob.key",
			},
			Domain: "consul",
		})

		wrap := clientCfg.OutgoingRPCWrapper()
		require.NotNil(t, wrap)

		tlsClient, err := wrap("dc1", client)
		require.NoError(t, err)
		defer tlsClient.Close()

		err = tlsClient.(*tls.Conn).Handshake()
		require.NoError(t, err)

		err = <-errc
		require.NoError(t, err)

		clientCerts := <-certc
		require.NotEmpty(t, clientCerts)
		require.Equal(t, "Bob", clientCerts[0].Subject.CommonName)
	})
}

func TestConfigurator_outgoingWrapperALPN_serverHasNoNodeNameInSAN(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	srvConfig := Config{
		InternalRPC: ProtocolConfig{
			CAFile:               "../test/hostname/CertAuth.crt",
			CertFile:             "../test/hostname/Alice.crt",
			KeyFile:              "../test/hostname/Alice.key",
			VerifyOutgoing:       false, // doesn't matter
			VerifyServerHostname: false, // doesn't matter
		},
		Domain: "consul",
	}

	client, errc := startALPNRPCTLSServer(t, &srvConfig, []string{"foo", "bar"})
	if client == nil {
		t.Fatalf("startTLSServer err: %v", <-errc)
	}

	config := Config{
		InternalRPC: ProtocolConfig{
			CAFile:               "../test/hostname/CertAuth.crt",
			CertFile:             "../test/hostname/Bob.crt",
			KeyFile:              "../test/hostname/Bob.key",
			VerifyOutgoing:       false, // doesn't matter
			VerifyServerHostname: false, // doesn't matter
		},
		Domain: "consul",
	}

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)
	wrap := c.OutgoingALPNRPCWrapper()
	require.NotNil(t, wrap)

	_, err = wrap("dc1", "bob", "foo", client)
	require.Error(t, err)
	_, ok := err.(*tls.CertificateVerificationError)
	require.True(t, ok)
	client.Close()

	<-errc
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

func TestConfigurator_Validation(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	const (
		caFile   = "../test/ca/root.cer"
		caPath   = "../test/ca_path"
		certFile = "../test/key/ourdomain.cer"
		keyFile  = "../test/key/ourdomain.key"
	)

	t.Run("empty config", func(t *testing.T) {
		_, err := NewConfigurator(Config{}, nil)
		require.NoError(t, err)
		require.NoError(t, new(Configurator).Update(Config{}))
	})

	t.Run("common fields", func(t *testing.T) {
		type testCase struct {
			config  ProtocolConfig
			isValid bool
		}

		testCases := map[string]testCase{
			"invalid CAFile": {
				ProtocolConfig{CAFile: "bogus"},
				false,
			},
			"invalid CAPath": {
				ProtocolConfig{CAPath: "bogus"},
				false,
			},
			"invalid CertFile": {
				ProtocolConfig{
					CertFile: "bogus",
					KeyFile:  keyFile,
				},
				false,
			},
			"invalid KeyFile": {
				ProtocolConfig{
					CertFile: certFile,
					KeyFile:  "bogus",
				},
				false,
			},
			"VerifyIncoming set but no CA": {
				ProtocolConfig{
					VerifyIncoming: true,
					CAFile:         "",
					CAPath:         "",
					CertFile:       certFile,
					KeyFile:        keyFile,
				},
				false,
			},
			"VerifyIncoming set but no CertFile": {
				ProtocolConfig{
					VerifyIncoming: true,
					CAFile:         caFile,
					CertFile:       "",
					KeyFile:        keyFile,
				},
				false,
			},
			"VerifyIncoming set but no KeyFile": {
				ProtocolConfig{
					VerifyIncoming: true,
					CAFile:         caFile,
					CertFile:       certFile,
					KeyFile:        "",
				},
				false,
			},
			"VerifyIncoming + CAFile": {
				ProtocolConfig{
					VerifyIncoming: true,
					CAFile:         caFile,
					CertFile:       certFile,
					KeyFile:        keyFile,
				},
				true,
			},
			"VerifyIncoming + CAPath": {
				ProtocolConfig{
					VerifyIncoming: true,
					CAPath:         caPath,
					CertFile:       certFile,
					KeyFile:        keyFile,
				},
				true,
			},
			"VerifyIncoming + invalid CAFile": {
				ProtocolConfig{
					VerifyIncoming: true,
					CAFile:         "bogus",
					CertFile:       certFile,
					KeyFile:        keyFile,
				},
				false,
			},
			"VerifyIncoming + invalid CAPath": {
				ProtocolConfig{
					VerifyIncoming: true,
					CAPath:         "bogus",
					CertFile:       certFile,
					KeyFile:        keyFile,
				},
				false,
			},
			"VerifyOutgoing + CAFile": {
				ProtocolConfig{VerifyOutgoing: true, CAFile: caFile},
				true,
			},
			"VerifyOutgoing + CAPath": {
				ProtocolConfig{VerifyOutgoing: true, CAPath: caPath},
				true,
			},
			"VerifyOutgoing + CAFile + CAPath": {
				ProtocolConfig{
					VerifyOutgoing: true,
					CAFile:         caFile,
					CAPath:         caPath,
				},
				true,
			},
			"VerifyOutgoing but no CA": {
				ProtocolConfig{
					VerifyOutgoing: true,
					CAFile:         "",
					CAPath:         "",
				},
				false,
			},
		}

		for desc, tc := range testCases {
			for _, p := range []string{"internal", "grpc", "https"} {
				info := fmt.Sprintf("%s => %s", p, desc)

				var cfg Config
				switch p {
				case "internal":
					cfg.InternalRPC = tc.config
				case "grpc":
					cfg.GRPC = tc.config
				case "https":
					cfg.HTTPS = tc.config
				default:
					t.Fatalf("unknown protocol: %s", p)
				}

				_, err1 := NewConfigurator(cfg, nil)
				err2 := new(Configurator).Update(cfg)

				if tc.isValid {
					require.NoError(t, err1, info)
					require.NoError(t, err2, info)
				} else {
					require.Error(t, err1, info)
					require.Error(t, err2, info)
				}
			}
		}
	})

	t.Run("VerifyIncoming + AutoTLS", func(t *testing.T) {
		cfg := Config{
			InternalRPC: ProtocolConfig{
				VerifyIncoming: true,
				CAFile:         caFile,
			},
			GRPC: ProtocolConfig{
				VerifyIncoming: true,
				CAFile:         caFile,
			},
			HTTPS: ProtocolConfig{
				VerifyIncoming: true,
				CAFile:         caFile,
			},
			AutoTLS: true,
		}

		_, err := NewConfigurator(cfg, nil)
		require.NoError(t, err)
		require.NoError(t, new(Configurator).Update(cfg))
	})
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
		tlsConf := c.internalRPCTLSConfig(false)
		require.Empty(t, tlsConf.ServerName)
	}
}

func TestConfigurator_LoadCAs(t *testing.T) {
	type variant struct {
		cafile, capath string
		shouldErr      bool
		isNil          bool
		count          int
		expectedCaPool *x509.CertPool
	}
	variants := []variant{
		{"", "", false, true, 0, nil},
		{"bogus", "", true, true, 0, nil},
		{"", "bogus", true, true, 0, nil},
		{"", "../test/bin", true, true, 0, nil},
		{"../test/ca/root.cer", "", false, false, 1, getExpectedCaPoolByFile(t)},
		{"", "../test/ca_path", false, false, 2, getExpectedCaPoolByDir(t)},
		{"../test/ca/root.cer", "../test/ca_path", false, false, 1, getExpectedCaPoolByFile(t)},
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
			assertDeepEqual(t, v.expectedCaPool, pool, cmpCertPool)
			require.Len(t, pems, v.count, info)
		}
	}
}

func TestConfigurator_InternalRPCMutualTLSCapable(t *testing.T) {
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
			InternalRPC: ProtocolConfig{
				CAFile: "../test/hostname/CertAuth.crt",
			},
			Domain: "consul",
		}
		c, err := NewConfigurator(config, nil)
		require.NoError(t, err)

		require.False(t, c.MutualTLSCapable())
	})

	t.Run("ca and manual key", func(t *testing.T) {
		config := Config{
			InternalRPC: ProtocolConfig{
				CAFile:   "../test/hostname/CertAuth.crt",
				CertFile: "../test/hostname/Bob.crt",
				KeyFile:  "../test/hostname/Bob.key",
			},
			Domain: "consul",
		}
		c, err := NewConfigurator(config, nil)
		require.NoError(t, err)

		require.True(t, c.MutualTLSCapable())
	})

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
	c := Configurator{base: &Config{}}
	c.base.InternalRPC.VerifyIncoming = true
	require.True(t, c.VerifyIncomingRPC())
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

		if diff := cmp.Diff(tc.expected, c, cmp.Options{
			cmpopts.IgnoreFields(tls.Config{}, "GetCertificate", "GetClientCertificate"),
			cmpopts.IgnoreUnexported(tls.Config{}),
		}); diff != "" {
			t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
		}
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
					InternalRPC: ProtocolConfig{
						TLSMinVersion: types.TLSv1_2,
					},
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
					InternalRPC: ProtocolConfig{
						TLSMinVersion: types.TLSv1_2,
					},
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
					InternalRPC: ProtocolConfig{
						TLSMinVersion: types.TLSv1_2,
					},
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
			name: "agent tls, default consul server name, no override",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{
					InternalRPC: ProtocolConfig{
						TLSMinVersion: types.TLSv1_2,
					},
					EnableAgentTLSForChecks: true,
					NodeName:                "nodename",
					ServerName:              "servername",
				}, nil)
			},
			expected: &tls.Config{
				MinVersion: tls.VersionTLS12,
				ServerName: "",
			},
		},
		{
			name: "agent tls, skip verify, consul node name for server name, no override",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{
					InternalRPC: ProtocolConfig{
						TLSMinVersion: types.TLSv1_2,
					},
					EnableAgentTLSForChecks: true,
					NodeName:                "nodename",
				}, nil)
			},
			skipVerify: true,
			expected: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
				ServerName:         "",
			},
		},
		{
			name: "agent tls, skip verify, with server name override",
			conf: func() (*Configurator, error) {
				return NewConfigurator(Config{
					InternalRPC: ProtocolConfig{
						TLSMinVersion: types.TLSv1_2,
					},
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

func TestConfigurator_InternalRPCVerifyServerHostname(t *testing.T) {
	c := Configurator{base: &Config{}}
	require.False(t, c.VerifyServerHostname())

	c.base.InternalRPC.VerifyServerHostname = true
	c.autoTLS.verifyServerHostname = false
	require.True(t, c.VerifyServerHostname())

	c.base.InternalRPC.VerifyServerHostname = false
	c.autoTLS.verifyServerHostname = true
	require.True(t, c.VerifyServerHostname())

	c.base.InternalRPC.VerifyServerHostname = true
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
	require.Equal(t, int64(4820915609), c.AutoEncryptCert().NotAfter.Unix())
}

func TestConfigurator_AuthorizeInternalRPCServerConn(t *testing.T) {
	caPEM, caPK, err := GenerateCA(CAOpts{Days: 5, Domain: "consul"})
	require.NoError(t, err)

	dir := testutil.TempDir(t, "ca")
	caPath := filepath.Join(dir, "ca.pem")
	err = os.WriteFile(caPath, []byte(caPEM), 0600)
	require.NoError(t, err)

	// Cert and key are not used, but required to get past validation.
	signer, err := ParseSigner(caPK)
	require.NoError(t, err)
	pub, pk, err := GenerateCert(CertOpts{
		Signer: signer,
		CA:     caPEM,
	})
	require.NoError(t, err)
	certFile := filepath.Join(dir, "cert.pem")
	err = os.WriteFile(certFile, []byte(pub), 0600)
	require.NoError(t, err)
	keyFile := filepath.Join(dir, "cert.key")
	err = os.WriteFile(keyFile, []byte(pk), 0600)
	require.NoError(t, err)

	cfg := Config{
		InternalRPC: ProtocolConfig{
			VerifyServerHostname: true,
			VerifyIncoming:       true,
			CAFile:               caPath,
			CertFile:             certFile,
			KeyFile:              keyFile,
		},
		Domain: "consul",
	}
	c := makeConfigurator(t, cfg)

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
		err = os.WriteFile(caPath, []byte(caPEM), 0600)
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
			InternalRPC: ProtocolConfig{
				VerifyServerHostname: true,
				VerifyIncoming:       false,
				CAFile:               caPath,
			},
			Domain: "consul",
		}
		c, err := NewConfigurator(cfg, hclog.New(nil))
		require.NoError(t, err)

		s := fakeTLSConn{}
		err = c.AuthorizeServerConn("dc1", s)
		require.NoError(t, err)
	})
}

func TestConfigurator_GRPCServerUseTLS(t *testing.T) {
	t.Run("certificate manually configured", func(t *testing.T) {
		c := makeConfigurator(t, Config{
			GRPC: ProtocolConfig{
				CertFile: "../test/hostname/Alice.crt",
				KeyFile:  "../test/hostname/Alice.key",
			},
		})
		require.True(t, c.GRPCServerUseTLS())
	})

	t.Run("no certificate", func(t *testing.T) {
		c := makeConfigurator(t, Config{})
		require.False(t, c.GRPCServerUseTLS())
	})

	t.Run("AutoTLS (default)", func(t *testing.T) {
		c := makeConfigurator(t, Config{})

		bobCert := loadFile(t, "../test/hostname/Bob.crt")
		bobKey := loadFile(t, "../test/hostname/Bob.key")
		require.NoError(t, c.UpdateAutoTLSCert(bobCert, bobKey))
		require.False(t, c.GRPCServerUseTLS())
	})

	t.Run("AutoTLS w/ UseAutoCert Disabled", func(t *testing.T) {
		c := makeConfigurator(t, Config{
			GRPC: ProtocolConfig{
				UseAutoCert: false,
			},
		})

		bobCert := loadFile(t, "../test/hostname/Bob.crt")
		bobKey := loadFile(t, "../test/hostname/Bob.key")
		require.NoError(t, c.UpdateAutoTLSCert(bobCert, bobKey))
		require.False(t, c.GRPCServerUseTLS())
	})

	t.Run("AutoTLS w/ UseAutoCert Enabled", func(t *testing.T) {
		c := makeConfigurator(t, Config{
			GRPC: ProtocolConfig{
				UseAutoCert: true,
			},
		})

		bobCert := loadFile(t, "../test/hostname/Bob.crt")
		bobKey := loadFile(t, "../test/hostname/Bob.key")
		require.NoError(t, c.UpdateAutoTLSCert(bobCert, bobKey))
		require.True(t, c.GRPCServerUseTLS())
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

func startRPCTLSServer(t *testing.T, c *Configurator) (net.Conn, <-chan error) {
	client, errc, _ := startTLSServer(c.IncomingRPCConfig())
	return client, errc
}

func startALPNRPCTLSServer(t *testing.T, config *Config, alpnProtos []string) (net.Conn, <-chan error) {
	cfg := makeConfigurator(t, *config).IncomingALPNRPCConfig(alpnProtos)
	client, errc, _ := startTLSServer(cfg)
	return client, errc
}

func makeConfigurator(t *testing.T, config Config) *Configurator {
	t.Helper()

	c, err := NewConfigurator(config, nil)
	require.NoError(t, err)

	return c
}

func startTLSServer(tlsConfigServer *tls.Config) (net.Conn, <-chan error, <-chan []*x509.Certificate) {
	errc := make(chan error, 1)
	certc := make(chan []*x509.Certificate, 1)

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
		certc <- tlsServer.ConnectionState().PeerCertificates
		close(errc)

		// Because net.Pipe() is unbuffered, if both sides
		// Close() simultaneously, we will deadlock as they
		// both send an alert and then block. So we make the
		// server read any data from the client until error or
		// EOF, which will allow the client to Close(), and
		// *then* we Close() the server.
		io.Copy(io.Discard, tlsServer)
		tlsServer.Close()
	}()
	return clientConn, errc, certc
}

func loadFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func getExpectedCaPoolByFile(t *testing.T) *x509.CertPool {
	pool := x509.NewCertPool()
	data, err := os.ReadFile("../test/ca/root.cer")
	if err != nil {
		t.Fatal("could not open test file ../test/ca/root.cer for reading")
	}
	if !pool.AppendCertsFromPEM(data) {
		t.Fatal("could not add test ca ../test/ca/root.cer to pool")
	}
	return pool
}

func getExpectedCaPoolByDir(t *testing.T) *x509.CertPool {
	pool := x509.NewCertPool()
	entries, err := os.ReadDir("../test/ca_path")
	if err != nil {
		t.Fatal("could not open test dir ../test/ca_path for reading")
	}

	for _, entry := range entries {
		filename := path.Join("../test/ca_path", entry.Name())

		data, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("could not open test file %s for reading", filename)
		}

		if !pool.AppendCertsFromPEM(data) {
			t.Fatalf("could not add test ca %s to pool", filename)
		}
	}

	return pool
}

// lazyCerts has a func field which can't be compared.
var cmpCertPool = cmp.Options{
	cmpopts.IgnoreFields(x509.CertPool{}, "lazyCerts"),
	cmp.AllowUnexported(x509.CertPool{}),
}

func assertDeepEqual(t *testing.T, x, y interface{}, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(x, y, opts...); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
}
