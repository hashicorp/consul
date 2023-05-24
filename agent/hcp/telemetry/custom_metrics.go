package telemetry

// Keys for custom Go Metrics metrics emitted only for the OTEL
// export (exporter.go) and transform (transform.go) failures and successes.
// These enable us to monitor OTEL operations.
var (
	transformFailureMetric []string = []string{"hcp", "otel", "transform", "failure"}

	exportSuccessMetric []string = []string{"hcp", "otel", "exporter", "export", "sucess"}
	exportFailureMetric []string = []string{"hcp", "otel", "exporter", "export", "failure"}

	exporterShutdownMetric   []string = []string{"hcp", "otel", "exporter", "shutdown"}
	exporterForceFlushMetric []string = []string{"hcp", "otel", "exporter", "force_flush"}
)
