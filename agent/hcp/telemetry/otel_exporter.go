package telemetry

import (
	"context"
	"fmt"
	"net/url"

	goMetrics "github.com/armon/go-metrics"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// MetricsClient exports Consul metrics in OTLP format to the desired endpoint.
type MetricsClient interface {
	ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error
}

// EndpointProvider provides the endpoint where metrics are exported to by the OTELExporter.
// EndpointProvider exposes the GetEndpoint() interface method to fetch the endpoint.
// This abstraction layer offers flexibility, in particular for dynamic configuration or changes to the endpoint.
type EndpointProvider interface {
<<<<<<< HEAD
	GetEndpoint() (*url.URL, bool)
=======
	GetEndpoint() *url.URL
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
}

// OTELExporter is a custom implementation of a OTEL Metrics SDK metrics.Exporter.
// The exporter is used by a OTEL Metrics SDK PeriodicReader to export aggregated metrics.
// This allows us to use a custom client - HCP authenticated MetricsClient.
type OTELExporter struct {
	client           MetricsClient
	endpointProvider EndpointProvider
}

// NewOTELExporter returns a configured OTELExporter.
func NewOTELExporter(client MetricsClient, endpointProvider EndpointProvider) *OTELExporter {
	return &OTELExporter{
		client:           client,
		endpointProvider: endpointProvider,
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
			Boundaries: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
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

	endpoint, ok := e.endpointProvider.GetEndpoint()
	if !ok {
		// We skip exporting metrics if we do not have an endpoint to send this to. This can
		// happen if we fail to download Agent Telemetry Config from HCP on start up.
		goMetrics.IncrCounter(internalMetricExportSkip, 1)
		return nil
	}

	err := e.client.ExportMetrics(ctx, otlpMetrics, endpoint.String())
	if err != nil {
		goMetrics.IncrCounter(internalMetricExportFailure, 1)
		return fmt.Errorf("failed to export metrics: %w", err)
	}

	goMetrics.IncrCounter(internalMetricExportSuccess, 1)
	return nil
}

// ForceFlush is a no-op, as the MetricsClient client holds no state.
func (e *OTELExporter) ForceFlush(ctx context.Context) error {
	goMetrics.IncrCounter(internalMetricExporterForceFlush, 1)
	return ctx.Err()
}

// Shutdown is a no-op, as the MetricsClient is a HTTP client that requires no graceful shutdown.
func (e *OTELExporter) Shutdown(ctx context.Context) error {
	goMetrics.IncrCounter(internalMetricExporterShutdown, 1)
	return ctx.Err()
}
