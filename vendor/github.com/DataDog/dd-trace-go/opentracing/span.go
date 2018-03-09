package opentracing

import (
	"fmt"
	"time"

	ddtrace "github.com/DataDog/dd-trace-go/tracer"
	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

// Span represents an active, un-finished span in the OpenTracing system.
// Spans are created by the Tracer interface.
type Span struct {
	*ddtrace.Span
	context SpanContext
	tracer  *Tracer
}

// Tracer provides access to the `Tracer`` that created this Span.
func (s *Span) Tracer() ot.Tracer {
	return s.tracer
}

// Context yields the SpanContext for this Span. Note that the return
// value of Context() is still valid after a call to Span.Finish(), as is
// a call to Span.Context() after a call to Span.Finish().
func (s *Span) Context() ot.SpanContext {
	return s.context
}

// SetBaggageItem sets a key:value pair on this Span and its SpanContext
// that also propagates to descendants of this Span.
func (s *Span) SetBaggageItem(key, val string) ot.Span {
	s.Span.Lock()
	defer s.Span.Unlock()

	s.context = s.context.WithBaggageItem(key, val)
	return s
}

// BaggageItem gets the value for a baggage item given its key. Returns the empty string
// if the value isn't found in this Span.
func (s *Span) BaggageItem(key string) string {
	s.Span.Lock()
	defer s.Span.Unlock()

	return s.context.baggage[key]
}

// SetTag adds a tag to the span, overwriting pre-existing values for
// the given `key`.
func (s *Span) SetTag(key string, value interface{}) ot.Span {
	switch key {
	case ServiceName:
		s.Span.Lock()
		defer s.Span.Unlock()
		s.Span.Service = fmt.Sprint(value)
	case ResourceName:
		s.Span.Lock()
		defer s.Span.Unlock()
		s.Span.Resource = fmt.Sprint(value)
	case SpanType:
		s.Span.Lock()
		defer s.Span.Unlock()
		s.Span.Type = fmt.Sprint(value)
	case Error:
		switch v := value.(type) {
		case nil:
			// no error
		case error:
			s.Span.SetError(v)
		default:
			s.Span.SetError(fmt.Errorf("%v", v))
		}
	default:
		// NOTE: locking is not required because the `SetMeta` is
		// already thread-safe
		s.Span.SetMeta(key, fmt.Sprint(value))
	}
	return s
}

// FinishWithOptions is like Finish() but with explicit control over
// timestamps and log data.
func (s *Span) FinishWithOptions(options ot.FinishOptions) {
	if options.FinishTime.IsZero() {
		options.FinishTime = time.Now().UTC()
	}

	s.Span.FinishWithTime(options.FinishTime.UnixNano())
}

// SetOperationName sets or changes the operation name.
func (s *Span) SetOperationName(operationName string) ot.Span {
	s.Span.Lock()
	defer s.Span.Unlock()

	s.Span.Name = operationName
	return s
}

// LogFields is an efficient and type-checked way to record key:value
// logging data about a Span, though the programming interface is a little
// more verbose than LogKV().
func (s *Span) LogFields(fields ...log.Field) {
	// TODO: implementation missing
}

// LogKV is a concise, readable way to record key:value logging data about
// a Span, though unfortunately this also makes it less efficient and less
// type-safe than LogFields().
func (s *Span) LogKV(keyVals ...interface{}) {
	// TODO: implementation missing
}

// LogEvent is deprecated: use LogFields or LogKV
func (s *Span) LogEvent(event string) {
	// TODO: implementation missing
}

// LogEventWithPayload deprecated: use LogFields or LogKV
func (s *Span) LogEventWithPayload(event string, payload interface{}) {
	// TODO: implementation missing
}

// Log is deprecated: use LogFields or LogKV
func (s *Span) Log(data ot.LogData) {
	// TODO: implementation missing
}

// NewSpan is the OpenTracing Span constructor
func NewSpan(operationName string) *Span {
	span := &ddtrace.Span{
		Name: operationName,
	}

	otSpan := &Span{
		Span: span,
		context: SpanContext{
			traceID:  span.TraceID,
			spanID:   span.SpanID,
			parentID: span.ParentID,
			sampled:  span.Sampled,
		},
	}

	// SpanContext is propagated and used to create children
	otSpan.context.span = otSpan
	return otSpan
}
