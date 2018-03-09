package opentracing

import (
	"errors"
	"io"
	"time"

	ddtrace "github.com/DataDog/dd-trace-go/tracer"
	ot "github.com/opentracing/opentracing-go"
)

// Tracer is a simple, thin interface for Span creation and SpanContext
// propagation. In the current state, this Tracer is a compatibility layer
// that wraps the Datadog Tracer implementation.
type Tracer struct {
	// impl is the Datadog Tracer implementation.
	impl *ddtrace.Tracer

	// config holds the Configuration used to create the Tracer.
	config *Configuration
}

// StartSpan creates, starts, and returns a new Span with the given `operationName`
// A Span with no SpanReference options (e.g., opentracing.ChildOf() or
// opentracing.FollowsFrom()) becomes the root of its own trace.
func (t *Tracer) StartSpan(operationName string, options ...ot.StartSpanOption) ot.Span {
	sso := ot.StartSpanOptions{}
	for _, o := range options {
		o.Apply(&sso)
	}

	return t.startSpanWithOptions(operationName, sso)
}

func (t *Tracer) startSpanWithOptions(operationName string, options ot.StartSpanOptions) ot.Span {
	if options.StartTime.IsZero() {
		options.StartTime = time.Now().UTC()
	}

	var context SpanContext
	var hasParent bool
	var parent *Span
	var span *ddtrace.Span

	for _, ref := range options.References {
		ctx, ok := ref.ReferencedContext.(SpanContext)
		if !ok {
			// ignore the SpanContext since it's not valid
			continue
		}

		// if we have parenting define it
		if ref.Type == ot.ChildOfRef {
			hasParent = true
			context = ctx
			parent = ctx.span
		}
	}

	if parent == nil {
		// create a root Span with the default service name and resource
		span = t.impl.NewRootSpan(operationName, t.config.ServiceName, operationName)

		if hasParent {
			// the Context doesn't have a Span reference because it
			// has been propagated from another process, so we set these
			// values manually
			span.TraceID = context.traceID
			span.ParentID = context.spanID
			t.impl.Sample(span)
		}
	} else {
		// create a child Span that inherits from a parent
		span = t.impl.NewChildSpan(operationName, parent.Span)
	}

	// create an OpenTracing compatible Span; the SpanContext has a
	// back-reference that is used for parent-child hierarchy
	otSpan := &Span{
		Span: span,
		context: SpanContext{
			traceID:  span.TraceID,
			spanID:   span.SpanID,
			parentID: span.ParentID,
			sampled:  span.Sampled,
		},
		tracer: t,
	}
	otSpan.context.span = otSpan

	// set start time
	otSpan.Span.Start = options.StartTime.UnixNano()

	if parent != nil {
		// propagate baggage items
		if l := len(parent.context.baggage); l > 0 {
			otSpan.context.baggage = make(map[string]string, len(parent.context.baggage))
			for k, v := range parent.context.baggage {
				otSpan.context.baggage[k] = v
			}
		}
	}

	// add tags from options
	for k, v := range options.Tags {
		otSpan.SetTag(k, v)
	}

	// add global tags
	for k, v := range t.config.GlobalTags {
		otSpan.SetTag(k, v)
	}

	return otSpan
}

// Inject takes the `sm` SpanContext instance and injects it for
// propagation within `carrier`. The actual type of `carrier` depends on
// the value of `format`. Currently supported Injectors are:
// * `TextMap`
// * `HTTPHeaders`
func (t *Tracer) Inject(ctx ot.SpanContext, format interface{}, carrier interface{}) error {
	switch format {
	case ot.TextMap, ot.HTTPHeaders:
		return t.config.TextMapPropagator.Inject(ctx, carrier)
	}
	return ot.ErrUnsupportedFormat
}

// Extract returns a SpanContext instance given `format` and `carrier`.
func (t *Tracer) Extract(format interface{}, carrier interface{}) (ot.SpanContext, error) {
	switch format {
	case ot.TextMap, ot.HTTPHeaders:
		return t.config.TextMapPropagator.Extract(carrier)
	}
	return nil, ot.ErrUnsupportedFormat
}

// Close method implements `io.Closer` interface to graceful shutdown the Datadog
// Tracer. Note that this is a blocking operation that waits for the flushing Go
// routine.
func (t *Tracer) Close() error {
	t.impl.Stop()
	return nil
}

// NewTracer uses a Configuration object to initialize a Datadog Tracer.
// The initialization returns a `io.Closer` that can be used to graceful
// shutdown the tracer. If the configuration object defines a disabled
// Tracer, a no-op implementation is returned.
func NewTracer(config *Configuration) (ot.Tracer, io.Closer, error) {
	if config.ServiceName == "" {
		// abort initialization if a `ServiceName` is not defined
		return nil, nil, errors.New("A Datadog Tracer requires a valid `ServiceName` set")
	}

	if config.Enabled == false {
		// return a no-op implementation so Datadog provides the minimum overhead
		return &ot.NoopTracer{}, &noopCloser{}, nil
	}

	// configure a Datadog Tracer
	transport := ddtrace.NewTransport(config.AgentHostname, config.AgentPort)
	tracer := &Tracer{
		impl:   ddtrace.NewTracerTransport(transport),
		config: config,
	}
	tracer.impl.SetDebugLogging(config.Debug)
	tracer.impl.SetSampleRate(config.SampleRate)

	// set the new Datadog Tracer as a `DefaultTracer` so it can be
	// used in integrations. NOTE: this is a temporary implementation
	// that can be removed once all integrations have been migrated
	// to the OpenTracing API.
	ddtrace.DefaultTracer = tracer.impl

	return tracer, tracer, nil
}
