package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/grpc/internal/testservice"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func TestHandler_EmitsStats(t *testing.T) {
	sink := patchGlobalMetrics(t)

	addr := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
	handler := NewHandler(addr)

	testservice.RegisterSimpleServer(handler.srv, &simple{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()

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
	defer conn.Close()

	client := testservice.NewSimpleClient(conn)
	fClient, err := client.Flow(ctx, &testservice.Req{Datacenter: "mine"})
	require.NoError(t, err)

	// Wait for the first event so that we know the stream is sending.
	_, err = fClient.Recv()
	require.NoError(t, err)

	expectedCounter := []metricCall{
		{key: []string{"testing", "grpc", "server", "request"}, val: 1},
	}
	require.Equal(t, expectedCounter, sink.incrCounterCalls)
	expectedGauge := []metricCall{
		{key: []string{"testing", "grpc", "server", "active_conns"}, val: 1},
		{key: []string{"testing", "grpc", "server", "active_streams"}, val: 1},
		// TODO: why is the count reset to 0 before the client receives the second message?
		{key: []string{"testing", "grpc", "server", "active_streams"}, val: 0},
	}
	require.Equal(t, expectedGauge, sink.gaugeCalls)
}

type simple struct {
	name string
}

func (s *simple) Flow(_ *testservice.Req, flow testservice.Simple_FlowServer) error {
	if err := flow.Send(&testservice.Resp{ServerName: "one"}); err != nil {
		return err
	}
	if err := flow.Send(&testservice.Resp{ServerName: "two"}); err != nil {
		return err
	}
	return nil
}

func (s *simple) Something(_ context.Context, _ *testservice.Req) (*testservice.Resp, error) {
	return &testservice.Resp{ServerName: "the-fake-service-name"}, nil
}

func patchGlobalMetrics(t *testing.T) *fakeMetricsSink {
	t.Helper()

	sink := &fakeMetricsSink{}
	cfg := &metrics.Config{
		ServiceName:      "testing",
		TimerGranularity: time.Millisecond, // Timers are in milliseconds
		ProfileInterval:  time.Second,      // Poll runtime every second
		FilterDefault:    true,
	}
	_, err := metrics.NewGlobal(cfg, sink)
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
