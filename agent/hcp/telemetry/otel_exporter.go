package telemetry

import (
	"context"
	"net/url"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
)

// OTELExporter is a custom implementation of a OTEL Metrics SDK metrics.Exporter.
// The exporter is used by a OTEL Metrics SDK PeriodicReader to export aggregated metrics.
// This allows us to use a custom client - HCP authenticated MetricsClient.
type OTELExporter struct {
	client   hcpclient.MetricsClient
	endpoint *url.URL
}

// NewOTELExporter returns a configured OTELExporter
func NewOTELExporter(client hcpclient.MetricsClient, endpoint *url.URL) *OTELExporter {
	return &OTELExporter{
		client:   client,
		endpoint: endpoint,
	}
}

// Temporality returns the Cumulative temporality for metrics aggregation.
// Telemetry Gateway stores metrics in Prometheus format, so use Cummulative aggregation as default.
func (e *OTELExporter) Temporality(_ metric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}

// Aggregation returns the Aggregation to use for an instrument kind.
// The default implementation provided by the OTEL Metrics SDK library DefaultAggregationSelector panics.
// This custom version replicates that logic, but removes the panic.
func (e *OTELExporter) Aggregation(kind metric.InstrumentKind) aggregation.Aggregation {
	switch kind {
	case metric.InstrumentKindObservableGauge:
		return aggregation.LastValue{}
	case metric.InstrumentKindHistogram:
		return aggregation.ExplicitBucketHistogram{
			Boundaries: []float64{0, 5, 10, 15, 20, 40, 60, 80, 100, 125, 150, 175, 200, 300, 500, 750, 1000, 2500, 5000, 7500, 10000},
			NoMinMax:   false,
		}
	}
	// for metric.InstrumentKindCounter and others, default to sum.
	return aggregation.Sum{}
}

// Export serializes and transmits metric data to a receiver.
func (e *OTELExporter) Export(ctx context.Context, metrics *metricdata.ResourceMetrics) error {
	otlpMetrics := transformOTLP(metrics)
	if isEmpty(otlpMetrics) {
		return nil
	}
	return e.client.ExportMetrics(ctx, otlpMetrics, e.endpoint.String())
}

// ForceFlush is a no-op, as the MetricsClient client holds no state.
func (e *OTELExporter) ForceFlush(ctx context.Context) error {
	// TODO: Emit metric when this operation occurs.
	return ctx.Err()
}

// Shutdown is a no-op, as the MetricsClient is a HTTP client that requires no graceful shutdown.
func (e *OTELExporter) Shutdown(ctx context.Context) error {
	// TODO: Emit metric when this operation occurs.
	return ctx.Err()
}
