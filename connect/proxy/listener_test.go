package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/consul/connect"

	metrics "github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agConnect "github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
)

func testSetupMetrics(t *testing.T) *metrics.InmemSink {
	// Record for ages (5 mins) so we can be confident that our assertions won't
	// fail on silly long test runs due to dropped data.
	s := metrics.NewInmemSink(10*time.Second, 300*time.Second)
	cfg := metrics.DefaultConfig("consul.proxy.test")
	cfg.EnableHostname = false
	cfg.EnableRuntimeMetrics = false
	metrics.NewGlobal(cfg, s)
	return s
}

func assertCurrentGaugeValue(t *testing.T, sink *metrics.InmemSink,
	name string, value float32) {
	t.Helper()

	data := sink.Data()

	// Loop backward through intervals until there is a non-empty one
	// Addresses flakiness around recording to one interval but accessing during the next
	var got float32
	for i := len(data) - 1; i >= 0; i-- {
		currentInterval := data[i]

		currentInterval.RLock()
		if len(currentInterval.Gauges) > 0 {
			got = currentInterval.Gauges[name].Value
			currentInterval.RUnlock()
			break
		}
		currentInterval.RUnlock()
	}

	if !assert.Equal(t, value, got) {
		buf := bytes.NewBuffer(nil)
		for _, intv := range data {
			intv.RLock()
			for name, val := range intv.Gauges {
				fmt.Fprintf(buf, "[%v][G] '%s': %0.3f\n", intv.Interval, name, val.Value)
			}
			intv.RUnlock()
		}
		t.Log(buf.String())
	}
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
			for name, val := range intv.Gauges {
				fmt.Fprintf(buf, "[%v][G] '%s': %0.3f\n", intv.Interval, name, val.Value)
			}
			for name, vals := range intv.Points {
				for _, val := range vals {
					fmt.Fprintf(buf, "[%v][P] '%s': %0.3f\n", intv.Interval, name, val)
				}
			}
			for name, agg := range intv.Counters {
				fmt.Fprintf(buf, "[%v][C] '%s': %s\n", intv.Interval, name, agg.AggregateSample)
			}
			for name, agg := range intv.Samples {
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
	sink := testSetupMetrics(t)

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
	assertCurrentGaugeValue(t, sink, "consul.proxy.test.inbound.conns;dst=db", 1)

	// Close listener to ensure all conns are closed and have reported their metrics
	l.Close()

	// Check all the tx/rx counters got added
	assertAllTimeCounterValue(t, sink, "consul.proxy.test.inbound.tx_bytes;dst=db", 11)
	assertAllTimeCounterValue(t, sink, "consul.proxy.test.inbound.rx_bytes;dst=db", 11)
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
	sink := testSetupMetrics(t)

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
	assertCurrentGaugeValue(t, sink, "consul.proxy.test.upstream.conns;src=web;dst_type=service;dst=db", 1)

	// Close listener to ensure all conns are closed and have reported their metrics
	l.Close()

	// Check all the tx/rx counters got added
	assertAllTimeCounterValue(t, sink, "consul.proxy.test.upstream.tx_bytes;src=web;dst_type=service;dst=db", 11)
	assertAllTimeCounterValue(t, sink, "consul.proxy.test.upstream.rx_bytes;src=web;dst_type=service;dst=db", 11)
}
