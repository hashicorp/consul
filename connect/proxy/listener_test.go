// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxy

import (
	"context"
	"net"
	"testing"

	"github.com/hashicorp/consul/connect"

	"github.com/stretchr/testify/require"

	agConnect "github.com/hashicorp/consul/agent/connect"
	agMetrics "github.com/hashicorp/consul/agent/metrics"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestPublicListener(t *testing.T) {
	// Can't enable t.Parallel since we rely on the global metrics instance.

	ca := agConnect.TestCA(t, nil)
	testApp := NewTestTCPServer(t)
	defer testApp.Close()

	port := freeport.GetOne(t)
	cfg := PublicListenerConfig{
		BindAddress:           "127.0.0.1",
		BindPort:              port,
		LocalServiceAddress:   testApp.Addr().String(),
		HandshakeTimeoutMs:    100,
		LocalConnectTimeoutMs: 100,
	}

	// Setup metrics to test they are recorded
	sink := agMetrics.TestSetupMetrics(t, "consul.proxy.test")

	svc := connect.TestService(t, "db", ca)
	l := NewPublicListener(svc, cfg, testutil.Logger(t))

	// Run proxy
	go func() {
		if err := l.Serve(); err != nil {
			t.Errorf("failed to listen: %v", err.Error())
		}
	}()
	defer l.Close()
	l.Wait()

	// Proxy and backend are running, play the part of a TLS client using same
	// cert for now.
	conn, err := svc.Dial(context.Background(), &connect.StaticResolver{
		Addr:    TestLocalAddr(port),
		CertURI: agConnect.TestSpiffeIDService(t, "db"),
	})
	require.NoError(t, err)

	TestEchoConn(t, conn, "")

	// Check active conn is tracked in gauges
	agMetrics.AssertGauge(t, sink, "consul.proxy.test.inbound.conns;dst=db", 1)

	// Close listener to ensure all conns are closed and have reported their metrics
	l.Close()

	// Check all the tx/rx counters got added
	agMetrics.AssertCounter(t, sink, "consul.proxy.test.inbound.tx_bytes;dst=db", 11)
	agMetrics.AssertCounter(t, sink, "consul.proxy.test.inbound.rx_bytes;dst=db", 11)
}

func TestUpstreamListener(t *testing.T) {
	// Can't enable t.Parallel since we rely on the global metrics instance.

	ca := agConnect.TestCA(t, nil)
	// Run a test server that we can dial.
	testSvr := connect.NewTestServer(t, "db", ca)
	go func() {
		err := testSvr.Serve()
		require.NoError(t, err)
	}()
	defer testSvr.Close()
	<-testSvr.Listening

	cfg := UpstreamConfig{
		DestinationType:      "service",
		DestinationNamespace: "default",
		DestinationName:      "db",
		Config:               map[string]interface{}{"connect_timeout_ms": 100},
		LocalBindAddress:     "localhost",
		LocalBindPort:        freeport.GetOne(t),
	}

	// Setup metrics to test they are recorded
	sink := agMetrics.TestSetupMetrics(t, "consul.proxy.test")

	svc := connect.TestService(t, "web", ca)

	// Setup with a statuc resolver instead
	rf := TestStaticUpstreamResolverFunc(&connect.StaticResolver{
		Addr:    testSvr.Addr,
		CertURI: agConnect.TestSpiffeIDService(t, "db"),
	})

	logger := testutil.Logger(t)
	l := newUpstreamListenerWithResolver(svc, cfg, rf, logger)

	// Run proxy
	go func() {
		if err := l.Serve(); err != nil {
			t.Errorf("failed to listen: %v", err.Error())
		}
	}()
	defer l.Close()
	l.Wait()

	// Proxy and fake remote service are running, play the part of the app
	// connecting to a remote connect service over TCP.
	conn, err := net.Dial("tcp",
		ipaddr.FormatAddressPort(cfg.LocalBindAddress, cfg.LocalBindPort))
	require.NoError(t, err)

	TestEchoConn(t, conn, "")

	// Check active conn is tracked in gauges
	agMetrics.AssertGauge(t, sink, "consul.proxy.test.upstream.conns;src=web;dst_type=service;dst=db", 1)

	// Close listener to ensure all conns are closed and have reported their metrics
	l.Close()

	// Check all the tx/rx counters got added
	agMetrics.AssertCounter(t, sink, "consul.proxy.test.upstream.tx_bytes;src=web;dst_type=service;dst=db", 11)
	agMetrics.AssertCounter(t, sink, "consul.proxy.test.upstream.rx_bytes;src=web;dst_type=service;dst=db", 11)
}
