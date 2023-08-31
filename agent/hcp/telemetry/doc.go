// Package telemetry implements functionality to collect, aggregate, convert and export
// telemetry data in OpenTelemetry Protocol (OTLP) format.
//
// The entrypoint is the OpenTelemetry (OTEL) go-metrics sink which:
// - Receives metric data.
// - Aggregates metric data using the OTEL Go Metrics SDK.
// - Exports metric data using a configurable OTEL exporter.
//
// The package also provides an OTEL exporter implementation to be used within the sink, which:
// - Transforms metric data from the Metrics SDK OTEL representation to OTLP format.
// - Exports OTLP metric data to an external endpoint using a configurable client.
package telemetry
