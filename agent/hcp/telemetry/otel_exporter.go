package telemetry

import (
	"context"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// OTELExporter is a custom implementation of a OTEL Metrics SDK metrics.Exporter.
// The exporter is used by a OTEL Metrics SDK PeriodicReader to export aggregated metrics.
// This allows us to use a custom client - HCP authenticated MetricsClient.
type OTELExporter struct {
	client   hcpclient.MetricsClient
	endpoint string
}

// Temporality returns the Cumulative temporality for metrics aggregation.
// Telemetry Gateway stores metrics in Prometheus format, so use Cummulative aggregation as default.
func (e *OTELExporter) Temporality(_ metric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}

// Aggregation returns the Aggregation to use for an instrument kind.
func (e *OTELExporter) Aggregation(kind metric.InstrumentKind) aggregation.Aggregation {
	switch kind {
	case metric.InstrumentKindObservableGauge:
		return aggregation.LastValue{}
	case metric.InstrumentKindHistogram:
		return aggregation.ExplicitBucketHistogram{
			Boundaries: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			NoMinMax:   false,
		}
	}
	// for metric.InstrumentKindCounter and others, default to sum.
	return aggregation.Sum{}
}

// Export serializes and transmits metric data to a receiver.
func (e *OTELExporter) Export(ctx context.Context, metrics *metricdata.ResourceMetrics) error {
	otlpMetrics, merr := transformOTLP(metrics)
	err := e.client.ExportMetrics(ctx, otlpMetrics, e.endpoint)
	return multierror.Append(merr, err)
}

// ForceFlush does nothing, as the MetricsClient client holds no state.
func (e *OTELExporter) ForceFlush(ctx context.Context) error { return ctx.Err() }

// Shutdown does nothing, as the MetricsClient is a HTTP client that requires no graceful shutdown.
func (e *OTELExporter) Shutdown(ctx context.Context) error { return ctx.Err() }
