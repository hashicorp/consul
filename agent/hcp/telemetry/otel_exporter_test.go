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

const (
	testExportEndpoint = "https://test.com/v1/metrics"
)

type mockMetricsClient struct {
	exportErr error
}

func (m *mockMetricsClient) ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error {
	return m.exportErr
}

type mockEndpointProvider struct {
	endpoint *url.URL
	disabled bool
}

func (m *mockEndpointProvider) GetEndpoint() *url.URL { return m.endpoint }
func (m *mockEndpointProvider) IsDisabled() bool      { return m.disabled }

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
		wantErr  string
		metrics  *metricdata.ResourceMetrics
		client   MetricsClient
		provider EndpointProvider
	}{
		"earlyReturnDisabledProvider": {
			client: &mockMetricsClient{},
			provider: &mockEndpointProvider{
				disabled: true,
			},
		},
		"earlyReturnWithoutEndpoint": {
			client:   &mockMetricsClient{},
			provider: &mockEndpointProvider{},
		},
		"earlyReturnWithoutScopeMetrics": {
			client:   &mockMetricsClient{},
			metrics:  mutateMetrics(nil),
			provider: &mockEndpointProvider{},
		},
		"earlyReturnWithoutMetrics": {
			client: &mockMetricsClient{},
			metrics: mutateMetrics([]metricdata.ScopeMetrics{
				{Metrics: []metricdata.Metrics{}},
			},
			),
			provider: &mockEndpointProvider{},
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
			provider: &mockEndpointProvider{
				endpoint: &url.URL{},
			},
			wantErr: "failed to export metrics",
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			provider := test.provider
			if provider == nil {
				u, err := url.Parse(testExportEndpoint)
				require.NoError(t, err)
				provider = &mockEndpointProvider{
					endpoint: u,
				}
			}

			exp := NewOTELExporter(test.client, provider)

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
			u, err := url.Parse(testExportEndpoint)
			require.NoError(t, err)

			exp := NewOTELExporter(tc.client, &mockEndpointProvider{
				endpoint: u,
			})

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
