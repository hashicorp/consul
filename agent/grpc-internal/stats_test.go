package internal

import (
	"context"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/grpc-internal/internal/testservice"
	"github.com/hashicorp/consul/proto/prototest"
)

func noopRegister(*grpc.Server) {}

func TestHandler_EmitsStats(t *testing.T) {
	sink, reset := patchGlobalMetrics(t)

	addr := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
	handler := NewHandler(hclog.Default(), addr, noopRegister)
	reset()

	testservice.RegisterSimpleServer(handler.srv, &simple{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return handler.srv.Serve(lis)
	})
	t.Cleanup(func() {
		if err := handler.Shutdown(); err != nil {
			t.Logf("grpc server shutdown: %v", err)
		}
		if err := g.Wait(); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	})

	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	client := testservice.NewSimpleClient(conn)
	fClient, err := client.Flow(ctx, &testservice.Req{Datacenter: "mine"})
	require.NoError(t, err)

	// Wait for the first event so that we know the stream is sending.
	_, err = fClient.Recv()
	require.NoError(t, err)

	cancel()
	conn.Close()
	handler.srv.GracefulStop()
	// Wait for the server to stop so that active_streams is predictable.
	require.NoError(t, g.Wait())

	// Occasionally the active_stream=0 metric may be emitted before the
	// active_conns=0 metric. The order of those metrics is not really important
	// so we sort the calls to match the expected.
	sort.Slice(sink.gaugeCalls, func(i, j int) bool {
		if i < 2 || j < 2 {
			return i < j
		}
		if len(sink.gaugeCalls[i].key) < 4 || len(sink.gaugeCalls[j].key) < 4 {
			return i < j
		}
		return sink.gaugeCalls[i].key[3] < sink.gaugeCalls[j].key[3]
	})

	cmpMetricCalls := cmp.AllowUnexported(metricCall{})
	expectedGauge := []metricCall{
		{key: []string{"testing", "grpc", "server", "connections"}, val: 1},
		{key: []string{"testing", "grpc", "server", "streams"}, val: 1},
		{key: []string{"testing", "grpc", "server", "connections"}, val: 0},
		{key: []string{"testing", "grpc", "server", "streams"}, val: 0},
	}
	prototest.AssertDeepEqual(t, expectedGauge, sink.gaugeCalls, cmpMetricCalls)

	expectedCounter := []metricCall{
		{key: []string{"testing", "grpc", "server", "connection", "count"}, val: 1},
		{key: []string{"testing", "grpc", "server", "request", "count"}, val: 1},
		{key: []string{"testing", "grpc", "server", "stream", "count"}, val: 1},
	}
	prototest.AssertDeepEqual(t, expectedCounter, sink.incrCounterCalls, cmpMetricCalls)
}

func patchGlobalMetrics(t *testing.T) (*fakeMetricsSink, func()) {
	t.Helper()

	sink := &fakeMetricsSink{}
	cfg := &metrics.Config{
		ServiceName:      "testing",
		TimerGranularity: time.Millisecond, // Timers are in milliseconds
		ProfileInterval:  time.Second,      // Poll runtime every second
		FilterDefault:    true,
	}
	var err error
	defaultMetrics = func() *metrics.Metrics {
		m, _ := metrics.New(cfg, sink)
		return m
	}
	require.NoError(t, err)
	reset := func() {
		t.Helper()
		defaultMetrics = metrics.Default
		require.NoError(t, err, "failed to reset global metrics")
	}
	return sink, reset
}

type fakeMetricsSink struct {
	lock sync.Mutex
	metrics.BlackholeSink
	gaugeCalls       []metricCall
	incrCounterCalls []metricCall
}

func (f *fakeMetricsSink) SetGaugeWithLabels(key []string, val float32, labels []metrics.Label) {
	f.lock.Lock()
	f.gaugeCalls = append(f.gaugeCalls, metricCall{key: key, val: val, labels: labels})
	f.lock.Unlock()
}

func (f *fakeMetricsSink) IncrCounterWithLabels(key []string, val float32, labels []metrics.Label) {
	f.lock.Lock()
	f.incrCounterCalls = append(f.incrCounterCalls, metricCall{key: key, val: val, labels: labels})
	f.lock.Unlock()
}

type metricCall struct {
	key    []string
	val    float32
	labels []metrics.Label
}

func logError(t *testing.T, f func() error) func() {
	return func() {
		if err := f(); err != nil {
			t.Logf(err.Error())
		}
	}
}
