package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestReporter_NewReporter_Failures(t *testing.T) {
	exp, err := NewMetricsExporter(&MetricsExporterConfig{
		Client: client.NewMockClient(t),
		Logger: hclog.L(),
	})

	require.NoError(t, err)

	for name, tc := range map[string]struct {
		cfg     *ReporterConfig
		wantErr string
	}{
		"failsWithoutGatherer": {
			cfg: &ReporterConfig{
				Exporter: exp,
				Logger:   hclog.L(),
			},
			wantErr: "metrics exporter, gatherer and logger must be provided",
		},
		"failsWithoutLogger": {
			cfg: &ReporterConfig{
				Exporter: exp,
			},
			wantErr: "metrics exporter, gatherer and logger must be provided",
		},
		"failesWithoutExporter": {
			cfg: &ReporterConfig{
				Gatherer: metrics.NewInmemSink(1*time.Second, time.Minute),
				Logger:   hclog.L(),
			},
			wantErr: "metrics exporter, gatherer and logger must be provided",
		},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := NewReporter(tc.cfg)
			require.Nil(t, r)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}

}

func TestReporter_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := client.NewMockClient(t)

	expCfg := &MetricsExporterConfig{
		Client:  client,
		Logger:  hclog.L(),
		Filters: []string{"raft.*"},
	}
	exp, err := NewMetricsExporter(expCfg)
	require.NoError(t, err)

	flushCh := make(chan struct{}, 1)
	cfg := DefaultConfig()
	cfg.Logger = hclog.L()
	cfg.Exporter = exp
	cfg.Gatherer = metrics.NewInmemSink(1*time.Second, time.Minute)

	r, err := NewReporter(cfg)
	r.testFlushCh = flushCh

	require.NoError(t, err)

	now := time.Now()
	interval := &metrics.IntervalMetrics{
		Interval: now,
		Gauges: map[string]metrics.GaugeValue{
			"consul.raft.peers": {
				Name:  "consul.raft.peers",
				Value: 2.0,
			},
		},
	}

	expectedOTLPMetrics := metricdata.ResourceMetrics{
		Resource: resource.NewWithAttributes(""),
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Scope: instrumentation.Scope{
					Name:    "github.com/hashicorp/consul/agent/hcp/client/telemetry",
					Version: "v1",
				},
				Metrics: r.cfg.Exporter.ConvertToOTLP([]*metrics.IntervalMetrics{interval}),
			},
		},
	}

	client.EXPECT().ExportMetrics(mock.Anything, expectedOTLPMetrics).Once().Return(nil)

	r.batchedMetrics[now] = interval

	go r.Run(ctx)

	select {
	case <-flushCh:
	case <-time.After(15 * time.Second):
		require.Fail(t, "reporter did not flush metrics in expected time")
	}

	cancel()
}
