package telemetry

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"

	"github.com/hashicorp/consul/agent/hcp/client"
)

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

type mockMetricsClient struct {
	exportErr error
}

func (m *mockMetricsClient) ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error {
	return m.exportErr
}

func TestExport(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		wantErr string
		metrics *metricdata.ResourceMetrics
		client  client.MetricsClient
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
			exp := &OTELExporter{
				client: test.client,
			}

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
