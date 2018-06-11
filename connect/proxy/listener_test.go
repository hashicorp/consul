package proxy

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agConnect "github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/lib/freeport"
)

func testSetupMetrics(t *testing.T) *metrics.InmemSink {
	// Record for ages (5 mins) so we can be confident that our assertions won't
	// fail on silly long test runs due to dropped data.
	s := metrics.NewInmemSink(10*time.Second, 300*time.Second)
	cfg := metrics.DefaultConfig("consul.proxy.test")
	cfg.EnableHostname = false
	metrics.NewGlobal(cfg, s)
	return s
}

func assertCurrentGaugeValue(t *testing.T, sink *metrics.InmemSink,
	name string, value float32) {
	t.Helper()

	data := sink.Data()

	// Current interval is the last one
	currentInterval := data[len(data)-1]
	currentInterval.RLock()
	defer currentInterval.RUnlock()

	assert.Equalf(t, value, currentInterval.Gauges[name].Value,
		"gauge value mismatch. Current Interval:\n%v", currentInterval)
}

func assertAllTimeCounterValue(t *testing.T, sink *metrics.InmemSink,
	name string, value float64) {
	t.Helper()

	data := sink.Data()

	var got float64
	for _, intv := range data {
		intv.RLock()
		// Note that InMemSink uses SampledValue and treats the _Sum_ not the Count
		// as the entire value.
		if sample, ok := intv.Counters[name]; ok {
			got += sample.Sum
		}
		intv.RUnlock()
	}

	if !assert.Equal(t, value, got) {
		// no nice way to dump this - this is copied from private method in
		// InMemSink used for dumping to stdout on SIGUSR1.
		buf := bytes.NewBuffer(nil)
		for _, intv := range data {
			intv.RLock()
			for _, val := range intv.Gauges {
				fmt.Fprintf(buf, "[%v][G] '%s': %0.3f\n", intv.Interval, name, val.Value)
			}
			for name, vals := range intv.Points {
				for _, val := range vals {
					fmt.Fprintf(buf, "[%v][P] '%s': %0.3f\n", intv.Interval, name, val)
				}
			}
			for _, agg := range intv.Counters {
				fmt.Fprintf(buf, "[%v][C] '%s': %s\n", intv.Interval, name, agg.AggregateSample)
			}
			for _, agg := range intv.Samples {
				fmt.Fprintf(buf, "[%v][S] '%s': %s\n", intv.Interval, name, agg.AggregateSample)
			}
			intv.RUnlock()
		}
		t.Log(buf.String())
	}
}

func TestPublicListener(t *testing.T) {
	// Can't enable t.Parallel since we rely on the global metrics instance.

	ca := agConnect.TestCA(t, nil)
	ports := freeport.GetT(t, 1)

	testApp := NewTestTCPServer(t)
	defer testApp.Close()

	cfg := PublicListenerConfig{
		BindAddress:           "127.0.0.1",
		BindPort:              ports[0],
		LocalServiceAddress:   testApp.Addr().String(),
		HandshakeTimeoutMs:    100,
		LocalConnectTimeoutMs: 100,
	}

	// Setup metrics to test they are recorded
	sink := testSetupMetrics(t)

	svc := connect.TestService(t, "db", ca)
	l := NewPublicListener(svc, cfg, log.New(os.Stderr, "", log.LstdFlags))

	// Run proxy
	go func() {
		err := l.Serve()
		require.NoError(t, err)
	}()
	defer l.Close()
	l.Wait()

	// Proxy and backend are running, play the part of a TLS client using same
	// cert for now.
	conn, err := svc.Dial(context.Background(), &connect.StaticResolver{
		Addr:    TestLocalAddr(ports[0]),
		CertURI: agConnect.TestSpiffeIDService(t, "db"),
	})
	require.NoError(t, err)

	TestEchoConn(t, conn, "")

	// Check active conn is tracked in gauges
	assertCurrentGaugeValue(t, sink, "consul.proxy.test.inbound.conns;dst=db", 1)

	// Close listener to ensure all conns are closed and have reported their
	// metrics
	l.Close()

	// Check all the tx/rx counters got added
	assertAllTimeCounterValue(t, sink, "consul.proxy.test.inbound.tx_bytes;dst=db", 11)
	assertAllTimeCounterValue(t, sink, "consul.proxy.test.inbound.rx_bytes;dst=db", 11)
}

func TestUpstreamListener(t *testing.T) {
	// Can't enable t.Parallel since we rely on the global metrics instance.

	ca := agConnect.TestCA(t, nil)
	ports := freeport.GetT(t, 1)

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
		ConnectTimeoutMs:     100,
		LocalBindAddress:     "localhost",
		LocalBindPort:        ports[0],
		resolver: &connect.StaticResolver{
			Addr:    testSvr.Addr,
			CertURI: agConnect.TestSpiffeIDService(t, "db"),
		},
	}

	// Setup metrics to test they are recorded
	sink := testSetupMetrics(t)

	svc := connect.TestService(t, "web", ca)

	l := NewUpstreamListener(svc, cfg, log.New(os.Stderr, "", log.LstdFlags))

	// Run proxy
	go func() {
		err := l.Serve()
		require.NoError(t, err)
	}()
	defer l.Close()
	l.Wait()

	// Proxy and fake remote service are running, play the part of the app
	// connecting to a remote connect service over TCP.
	conn, err := net.Dial("tcp",
		fmt.Sprintf("%s:%d", cfg.LocalBindAddress, cfg.LocalBindPort))
	require.NoError(t, err)

	TestEchoConn(t, conn, "")

	// Check active conn is tracked in gauges
	assertCurrentGaugeValue(t, sink, "consul.proxy.test.upstream.conns;src=web;dst_type=service;dst=db", 1)

	// Close listener to ensure all conns are closed and have reported their
	// metrics
	l.Close()

	// Check all the tx/rx counters got added
	assertAllTimeCounterValue(t, sink, "consul.proxy.test.upstream.tx_bytes;src=web;dst_type=service;dst=db", 11)
	assertAllTimeCounterValue(t, sink, "consul.proxy.test.upstream.rx_bytes;src=web;dst_type=service;dst=db", 11)
}
