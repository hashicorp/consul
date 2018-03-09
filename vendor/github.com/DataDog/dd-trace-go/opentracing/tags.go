package opentracing

const (
	// SpanType defines the Span type (web, db, cache)
	SpanType = "span.type"
	// ServiceName defines the Service name for this Span
	ServiceName = "service.name"
	// ResourceName defines the Resource name for the Span
	ResourceName = "resource.name"
	// Error defines an error.
	Error = "error.error"
)
