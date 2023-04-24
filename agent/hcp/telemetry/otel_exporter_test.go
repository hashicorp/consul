package telemetry

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func TestTemporality(t *testing.T) {
	exp := &OTELExporter{}
	require.Equal(t, metricdata.CumulativeTemporality, exp.Temporality(metric.InstrumentKindCounter))
}

func TestAggregation(t *testing.T) {
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
		t.Run(name, func(t *testing.T) {
			exp := &OTELExporter{}
			require.Equal(t, test.expAgg, exp.Aggregation(test.kind))
		})
	}
}

type mockErrMetricsClient struct{}

func (m *mockErrMetricsClient) ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error {
	return fmt.Errorf("failed to export metrics")
}

type mockMetricsClient struct{}

func (m *mockMetricsClient) ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error {
	return nil
}

func TestExport(t *testing.T) {
	for name, test := range map[string]struct {
		wantErr string
		metrics *metricdata.ResourceMetrics
		client  client.MetricsClient
	}{
		"errorWithExportFailure": {
			client: &mockErrMetricsClient{},
			metrics: &metricdata.ResourceMetrics{
				Resource: resource.Empty(),
			},
			wantErr: "failed to export metrics",
		},
		"errorWithTransformFailure": {
			wantErr: "unknown aggregation: metricdata.Gauge[int64]",
			client:  &mockMetricsClient{},
			metrics: &metricdata.ResourceMetrics{
				Resource: resource.Empty(),
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Metrics: []metricdata.Metrics{
							{
								// unsupported, only float64 supported
								Data: metricdata.Gauge[int64]{},
							},
						},
					},
				},
			},
		},
		"multierrorTransformExportFailure": {
			wantErr: "2 errors occurred:\n\t* unknown aggregation: metricdata.Gauge[int64]\n\t* failed to export metrics",
			client:  &mockErrMetricsClient{},
			metrics: &metricdata.ResourceMetrics{
				Resource: resource.Empty(),
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Metrics: []metricdata.Metrics{
							{
								// unsupported, only float64 supported
								Data: metricdata.Gauge[int64]{},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			exp := &OTELExporter{
				client: test.client,
			}

			err := exp.Export(context.Background(), test.metrics)
			require.Error(t, err)
			require.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestForceFlush(t *testing.T) {
	exp := &OTELExporter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.Error(t, exp.ForceFlush(ctx))
}

func TestShutdown(t *testing.T) {
	exp := &OTELExporter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.Error(t, exp.Shutdown(ctx))
}
