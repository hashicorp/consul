package opentracing

import (
	"strconv"
	"strings"

	ot "github.com/opentracing/opentracing-go"
)

// Propagator implementations should be able to inject and extract
// SpanContexts into an implementation specific carrier.
type Propagator interface {
	// Inject takes the SpanContext and injects it into the carrier using
	// an implementation specific method.
	Inject(context ot.SpanContext, carrier interface{}) error

	// Extract returns the SpanContext from the given carrier using an
	// implementation specific method.
	Extract(carrier interface{}) (ot.SpanContext, error)
}

const (
	defaultBaggageHeaderPrefix = "ot-baggage-"
	defaultTraceIDHeader       = "x-datadog-trace-id"
	defaultParentIDHeader      = "x-datadog-parent-id"
)

// NewTextMapPropagator returns a new propagator which uses opentracing.TextMap
// to inject and extract values. The parameters specify the prefix that will
// be used to prefix baggage header keys along with the trace and parent header.
// Empty strings may be provided to use the defaults, which are: "ot-baggage-" as
// prefix for baggage headers, "x-datadog-trace-id" and "x-datadog-parent-id" for
// trace and parent ID headers.
func NewTextMapPropagator(baggagePrefix, traceHeader, parentHeader string) *TextMapPropagator {
	if baggagePrefix == "" {
		baggagePrefix = defaultBaggageHeaderPrefix
	}
	if traceHeader == "" {
		traceHeader = defaultTraceIDHeader
	}
	if parentHeader == "" {
		parentHeader = defaultParentIDHeader
	}
	return &TextMapPropagator{baggagePrefix, traceHeader, parentHeader}
}

// TextMapPropagator implements a propagator which uses opentracing.TextMap
// internally.
type TextMapPropagator struct {
	baggagePrefix string
	traceHeader   string
	parentHeader  string
}

// Inject defines the TextMapPropagator to propagate SpanContext data
// out of the current process. The implementation propagates the
// TraceID and the current active SpanID, as well as the Span baggage.
func (p *TextMapPropagator) Inject(context ot.SpanContext, carrier interface{}) error {
	ctx, ok := context.(SpanContext)
	if !ok {
		return ot.ErrInvalidSpanContext
	}
	writer, ok := carrier.(ot.TextMapWriter)
	if !ok {
		return ot.ErrInvalidCarrier
	}

	// propagate the TraceID and the current active SpanID
	writer.Set(p.traceHeader, strconv.FormatUint(ctx.traceID, 10))
	writer.Set(p.parentHeader, strconv.FormatUint(ctx.spanID, 10))

	// propagate OpenTracing baggage
	for k, v := range ctx.baggage {
		writer.Set(p.baggagePrefix+k, v)
	}
	return nil
}

// Extract implements Propagator.
func (p *TextMapPropagator) Extract(carrier interface{}) (ot.SpanContext, error) {
	reader, ok := carrier.(ot.TextMapReader)
	if !ok {
		return nil, ot.ErrInvalidCarrier
	}
	var err error
	var traceID, parentID uint64
	decodedBaggage := make(map[string]string)

	// extract SpanContext fields
	err = reader.ForeachKey(func(k, v string) error {
		switch strings.ToLower(k) {
		case p.traceHeader:
			traceID, err = strconv.ParseUint(v, 10, 64)
			if err != nil {
				return ot.ErrSpanContextCorrupted
			}
		case p.parentHeader:
			parentID, err = strconv.ParseUint(v, 10, 64)
			if err != nil {
				return ot.ErrSpanContextCorrupted
			}
		default:
			lowercaseK := strings.ToLower(k)
			if strings.HasPrefix(lowercaseK, p.baggagePrefix) {
				decodedBaggage[strings.TrimPrefix(lowercaseK, p.baggagePrefix)] = v
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if traceID == 0 || parentID == 0 {
		return nil, ot.ErrSpanContextNotFound
	}

	return SpanContext{
		traceID: traceID,
		spanID:  parentID,
		baggage: decodedBaggage,
	}, nil
}
