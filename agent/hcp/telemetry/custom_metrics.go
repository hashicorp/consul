package telemetry

// Keys for custom Go Metrics metrics emitted only for the OTEL
// export (exporter.go) and transform (transform.go) failures and successes.
// These enable us to monitor OTEL operations.
var (
	internalMetricTransformFailure []string = []string{"hcp", "otel", "transform", "failure"}

	internalMetricExportSuccess []string = []string{"hcp", "otel", "exporter", "export", "sucess"}
	internalMetricExportFailure []string = []string{"hcp", "otel", "exporter", "export", "failure"}

	internalMetricExporterShutdown   []string = []string{"hcp", "otel", "exporter", "shutdown"}
	internalMetricExporterForceFlush []string = []string{"hcp", "otel", "exporter", "force_flush"}
)
