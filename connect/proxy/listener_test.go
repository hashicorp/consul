package proxy

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	metrics "github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agConnect "github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/lib/freeport"
)

type MockSink struct {
	keys   [][]string
	vals   []float32
	labels [][]metrics.Label
}

func (m *MockSink) SetGaugeWithLabels(key []string, val float32, labels []metrics.Label) {
	m.keys = append(m.keys, key)
	m.vals = append(m.vals, val)
	m.labels = append(m.labels, labels)
}

func (m *MockSink) IncrCounterWithLabels(key []string, val float32, labels []metrics.Label) {
	m.keys = append(m.keys, key)
	m.vals = append(m.vals, val)
	m.labels = append(m.labels, labels)
}

// Empty methods below only included to satisfy MetricSink interface
func (m *MockSink) EmitKey(key []string, val float32) {
}
func (m *MockSink) SetGauge(key []string, val float32) {
}
func (m *MockSink) IncrCounter(key []string, val float32) {
}
func (m *MockSink) AddSample(key []string, val float32) {
}
func (m *MockSink) AddSampleWithLabels(key []string, val float32, labels []metrics.Label) {
}

// Flattens the key for formatting along with its labels, removes spaces
func (m *MockSink) flattenKeyLabels(parts []string, labels []metrics.Label) (string, string) {
	buf := &bytes.Buffer{}
	replacer := strings.NewReplacer(" ", "_")

	if len(parts) > 0 {
		replacer.WriteString(buf, parts[0])
	}
	for _, part := range parts[1:] {
		replacer.WriteString(buf, ".")
		replacer.WriteString(buf, part)
	}

	key := buf.String()

	for _, label := range labels {
		replacer.WriteString(buf, fmt.Sprintf(";%s=%s", label.Name, label.Value))
	}

	return buf.String(), key
}

func testSetupMetrics(t *testing.T) *MockSink {
	// Record for ages (5 mins) so we can be confident that our assertions won't
	// fail on silly long test runs due to dropped data.
	s := MockSink{}
	cfg := metrics.DefaultConfig("consul.proxy.test")
	cfg.EnableHostname = false
	metrics.NewGlobal(cfg, &s)
	return &s
}

func assertTelemetryValue(t *testing.T, sink *MockSink,
	name string, value float32) {
	t.Helper()

	var idx int
	var found bool

	for i := 0; i < len(sink.keys); i++ {
		k, _ := sink.flattenKeyLabels(sink.keys[i], sink.labels[i])

		if k == name {
			idx = i
			found = true
		}
	}

	assert.Equal(t, true, found, "metric not found in sink: %s", name)

	assert.Equalf(t, value, sink.vals[idx],
		"'%s'\n telemetry mismatch - expected: %v, got %v", name, value, sink.vals[idx])
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
	assertTelemetryValue(t, sink, "consul.proxy.test.inbound.conns;dst=db", 1)

	// Close listener to ensure all conns are closed and have reported their
	// metrics
	l.Close()

	// Check all the tx/rx counters got added
	assertTelemetryValue(t, sink, "consul.proxy.test.inbound.tx_bytes;dst=db", 11)
	assertTelemetryValue(t, sink, "consul.proxy.test.inbound.rx_bytes;dst=db", 11)
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
	assertTelemetryValue(t, sink, "consul.proxy.test.upstream.conns;src=web;dst_type=service;dst=db", 1)

	// Close listener to ensure all conns are closed and have reported their
	// metrics
	l.Close()

	// Check all the tx/rx counters got added
	assertTelemetryValue(t, sink, "consul.proxy.test.upstream.tx_bytes;src=web;dst_type=service;dst=db", 11)
	assertTelemetryValue(t, sink, "consul.proxy.test.upstream.rx_bytes;src=web;dst_type=service;dst=db", 11)
}
