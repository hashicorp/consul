package telemetry

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

type mockMetricsClient struct {
	exportErr error
}

func (m *mockMetricsClient) ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error {
	return m.exportErr
}

<<<<<<< HEAD
type mockEndpointProvider struct {
	url *url.URL
}

func (m *mockEndpointProvider) GetEndpoint() (*url.URL, bool) {
	return m.url, m.url != nil
}
=======
type mockEndpointProvider struct{}

func (m *mockEndpointProvider) GetEndpoint() *url.URL { return &url.URL{} }
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh

func TestTemporality(t *testing.T) {
	t.Parallel()
	exp := &OTELExporter{}
	require.Equal(t, metricdata.CumulativeTemporality, exp.Temporality(metric.InstrumentKindCounter))
}

func TestAggregation(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		kind   metric.InstrumentKind
		expAgg aggregation.Aggregation
	}{
		"gauge": {
			kind:   metric.InstrumentKindObservableGauge,
			expAgg: aggregation.LastValue{},
		},
		"counter": {
			kind:   metric.InstrumentKindCounter,
			expAgg: aggregation.Sum{},
		},
		"histogram": {
			kind:   metric.InstrumentKindHistogram,
			expAgg: aggregation.ExplicitBucketHistogram{Boundaries: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000}, NoMinMax: false},
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			exp := &OTELExporter{}
			require.Equal(t, test.expAgg, exp.Aggregation(test.kind))
		})
	}
}

func TestExport(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		wantErr string
		metrics *metricdata.ResourceMetrics
		client  MetricsClient
	}{
		"earlyReturnWithoutScopeMetrics": {
			client:  &mockMetricsClient{},
			metrics: mutateMetrics(nil),
		},
		"earlyReturnWithoutMetrics": {
			client: &mockMetricsClient{},
			metrics: mutateMetrics([]metricdata.ScopeMetrics{
				{Metrics: []metricdata.Metrics{}},
			},
			),
		},
		"errorWithExportFailure": {
			client: &mockMetricsClient{
				exportErr: fmt.Errorf("failed to export metrics."),
			},
			metrics: mutateMetrics([]metricdata.ScopeMetrics{
				{
					Metrics: []metricdata.Metrics{
						{
							Name: "consul.raft.commitTime",
							Data: metricdata.Gauge[float64]{},
						},
					},
				},
			},
			),
			wantErr: "failed to export metrics",
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
<<<<<<< HEAD
			exp := NewOTELExporter(test.client, &mockEndpointProvider{
				url: &url.URL{},
			})
=======
			exp := NewOTELExporter(test.client, &mockEndpointProvider{})
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh

			err := exp.Export(context.Background(), test.metrics)
			if test.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

// TestExport_CustomMetrics tests that a custom metric (hcp.otel.exporter.*) is emitted
// for exporter operations. This test cannot be run in parallel as the metrics.NewGlobal()
// sets a shared global sink.
func TestExport_CustomMetrics(t *testing.T) {
	for name, tc := range map[string]struct {
		client    MetricsClient
		metricKey []string
		operation string
	}{
		"exportSuccessEmitsCustomMetric": {
			client:    &mockMetricsClient{},
			metricKey: internalMetricExportSuccess,
			operation: "export",
		},
		"exportFailureEmitsCustomMetric": {
			client: &mockMetricsClient{
				exportErr: fmt.Errorf("client err"),
			},
			metricKey: internalMetricExportFailure,
			operation: "export",
		},
		"shutdownEmitsCustomMetric": {
			metricKey: internalMetricExporterShutdown,
			operation: "shutdown",
		},
		"forceFlushEmitsCustomMetric": {
			metricKey: internalMetricExporterForceFlush,
			operation: "flush",
		},
	} {
		t.Run(name, func(t *testing.T) {
			// Init global sink.
			serviceName := "test.transform"
			cfg := metrics.DefaultConfig(serviceName)
			cfg.EnableHostname = false

			sink := metrics.NewInmemSink(10*time.Second, 10*time.Second)
			metrics.NewGlobal(cfg, sink)

			// Perform operation that emits metric.
<<<<<<< HEAD
			exp := NewOTELExporter(tc.client, &mockEndpointProvider{
				url: &url.URL{},
			})
=======
			exp := NewOTELExporter(tc.client, &mockEndpointProvider{})
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh

			ctx := context.Background()
			switch tc.operation {
			case "flush":
				exp.ForceFlush(ctx)
			case "shutdown":
				exp.Shutdown(ctx)
			default:
				exp.Export(ctx, inputResourceMetrics)
			}

			// Collect sink metrics.
			intervals := sink.Data()
			require.Len(t, intervals, 1)
			key := serviceName + "." + strings.Join(tc.metricKey, ".")
			sv := intervals[0].Counters[key]

			// Verify count for transform failure metric.
			require.NotNil(t, sv)
			require.NotNil(t, sv.AggregateSample)
			require.Equal(t, 1, sv.AggregateSample.Count)
		})
	}
}

func TestForceFlush(t *testing.T) {
	t.Parallel()
	exp := &OTELExporter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := exp.ForceFlush(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestShutdown(t *testing.T) {
	t.Parallel()
	exp := &OTELExporter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := exp.Shutdown(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func mutateMetrics(m []metricdata.ScopeMetrics) *metricdata.ResourceMetrics {
	return &metricdata.ResourceMetrics{
		Resource:     resource.Empty(),
		ScopeMetrics: m,
	}
}
