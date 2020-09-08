package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/grpc/internal/testservice"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func noopRegister(*grpc.Server) {}

func TestHandler_EmitsStats(t *testing.T) {
	sink := patchGlobalMetrics(t)

	addr := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
	handler := NewHandler(addr, noopRegister)

	testservice.RegisterSimpleServer(handler.srv, &simple{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(logError(t, lis.Close))

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
	t.Cleanup(logError(t, conn.Close))

	client := testservice.NewSimpleClient(conn)
	fClient, err := client.Flow(ctx, &testservice.Req{Datacenter: "mine"})
	require.NoError(t, err)

	// Wait for the first event so that we know the stream is sending.
	_, err = fClient.Recv()
	require.NoError(t, err)

	cancel()
	// Wait for the server to stop so that active_streams is predictable.
	retry.RunWith(fastRetry, t, func(r *retry.R) {
		expectedGauge := []metricCall{
			{key: []string{"testing", "grpc", "server", "active_conns"}, val: 1},
			{key: []string{"testing", "grpc", "server", "active_streams"}, val: 1},
			{key: []string{"testing", "grpc", "server", "active_streams"}, val: 0},
		}
		require.Equal(r, expectedGauge, sink.gaugeCalls)
	})

	expectedCounter := []metricCall{
		{key: []string{"testing", "grpc", "server", "request"}, val: 1},
	}
	require.Equal(t, expectedCounter, sink.incrCounterCalls)
}

var fastRetry = &retry.Timer{Timeout: 7 * time.Second, Wait: 2 * time.Millisecond}

func patchGlobalMetrics(t *testing.T) *fakeMetricsSink {
	t.Helper()

	sink := &fakeMetricsSink{}
	cfg := &metrics.Config{
		ServiceName:      "testing",
		TimerGranularity: time.Millisecond, // Timers are in milliseconds
		ProfileInterval:  time.Second,      // Poll runtime every second
		FilterDefault:    true,
	}
	var err error
	defaultMetrics, err = metrics.New(cfg, sink)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err = metrics.NewGlobal(cfg, &metrics.BlackholeSink{})
		require.NoError(t, err, "failed to reset global metrics")
	})
	return sink
}

type fakeMetricsSink struct {
	metrics.BlackholeSink
	gaugeCalls       []metricCall
	incrCounterCalls []metricCall
}

func (f *fakeMetricsSink) SetGaugeWithLabels(key []string, val float32, labels []metrics.Label) {
	f.gaugeCalls = append(f.gaugeCalls, metricCall{key: key, val: val, labels: labels})
}

func (f *fakeMetricsSink) IncrCounterWithLabels(key []string, val float32, labels []metrics.Label) {
	f.incrCounterCalls = append(f.incrCounterCalls, metricCall{key: key, val: val, labels: labels})
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
