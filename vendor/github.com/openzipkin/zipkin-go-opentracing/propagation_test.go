package zipkintracer_test

import (
	"bytes"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	opentracing "github.com/opentracing/opentracing-go"

	zipkintracer "github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/flag"
	"github.com/openzipkin/zipkin-go-opentracing/types"
)

type verbatimCarrier struct {
	zipkintracer.SpanContext
	b map[string]string
}

var _ zipkintracer.DelegatingCarrier = &verbatimCarrier{}

func (vc *verbatimCarrier) SetBaggageItem(k, v string) {
	vc.b[k] = v
}

func (vc *verbatimCarrier) GetBaggage(f func(string, string)) {
	for k, v := range vc.b {
		f(k, v)
	}
}

func (vc *verbatimCarrier) SetState(tID types.TraceID, sID uint64, pID *uint64, sampled bool, flags flag.Flags) {
	vc.SpanContext = zipkintracer.SpanContext{
		TraceID:      tID,
		SpanID:       sID,
		ParentSpanID: pID,
		Sampled:      sampled,
		Flags:        flags,
	}
}

func (vc *verbatimCarrier) State() (traceID types.TraceID, spanID uint64, parentSpanID *uint64, sampled bool, flags flag.Flags) {
	return vc.SpanContext.TraceID, vc.SpanContext.SpanID, vc.SpanContext.ParentSpanID, vc.SpanContext.Sampled, vc.SpanContext.Flags
}

func TestSpanPropagator(t *testing.T) {
	const op = "test"
	recorder := zipkintracer.NewInMemoryRecorder()
	tracer, err := zipkintracer.NewTracer(
		recorder,
		zipkintracer.ClientServerSameSpan(true),
		zipkintracer.DebugMode(true),
		zipkintracer.TraceID128Bit(true),
	)
	if err != nil {
		t.Fatalf("Unable to create Tracer: %+v", err)
	}

	// create root span so propagation test will include parentSpanID
	ps := tracer.StartSpan("root")
	defer ps.Finish()

	// client side span with parent span 'ps'
	sp := tracer.StartSpan(op, opentracing.ChildOf(ps.Context()))
	sp.SetBaggageItem("foo", "bar")
	tmc := opentracing.HTTPHeadersCarrier(http.Header{})
	tests := []struct {
		typ, carrier interface{}
	}{
		{zipkintracer.Delegator, zipkintracer.DelegatingCarrier(&verbatimCarrier{b: map[string]string{}})},
		{opentracing.Binary, &bytes.Buffer{}},
		{opentracing.HTTPHeaders, tmc},
		{opentracing.TextMap, tmc},
	}

	for i, test := range tests {
		if err := tracer.Inject(sp.Context(), test.typ, test.carrier); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		injectedContext, err := tracer.Extract(test.typ, test.carrier)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		child := tracer.StartSpan(
			op,
			opentracing.ChildOf(injectedContext))
		child.Finish()
	}
	sp.Finish()

	spans := recorder.GetSpans()
	if a, e := len(spans), len(tests)+1; a != e {
		t.Fatalf("expected %d spans, got %d", e, a)
	}

	// The last span is the original one.
	exp, spans := spans[len(spans)-1], spans[:len(spans)-1]
	exp.Duration = time.Duration(123)
	exp.Start = time.Time{}.Add(1)

	for i, sp := range spans {
		if a, e := *sp.Context.ParentSpanID, exp.Context.SpanID; a != e {
			t.Fatalf("%d: ParentSpanID %d does not match expectation %d", i, a, e)
		} else {
			// Prepare for comparison.
			sp.Context.Flags &= flag.Debug  // other flags then Debug should be discarded in comparison
			exp.Context.Flags &= flag.Debug // other flags then Debug should be discarded in comparison
			sp.Context.SpanID, sp.Context.ParentSpanID = exp.Context.SpanID, exp.Context.ParentSpanID
			sp.Duration, sp.Start = exp.Duration, exp.Start
		}
		if a, e := sp.Context.TraceID, exp.Context.TraceID; a != e {
			t.Fatalf("%d: TraceID changed from %d to %d", i, e, a)
		}
		if exp.Context.ParentSpanID == nil {
			t.Fatalf("%d: Expected a ParentSpanID, got nil", i)
		}
		if p, c := sp.Context.ParentSpanID, exp.Context.ParentSpanID; p != c {
			t.Fatalf("%d: ParentSpanID changed from %d to %d", i, p, c)
		}
		if !reflect.DeepEqual(exp, sp) {
			t.Fatalf("%d: wanted %+v, got %+v", i, spew.Sdump(exp), spew.Sdump(sp))
		}
	}
}

func TestInvalidCarrier(t *testing.T) {
	recorder := zipkintracer.NewInMemoryRecorder()
	tracer, err := zipkintracer.NewTracer(
		recorder,
		zipkintracer.ClientServerSameSpan(true),
		zipkintracer.DebugMode(true),
		zipkintracer.TraceID128Bit(true),
	)
	if err != nil {
		t.Fatalf("Unable to create Tracer: %+v", err)
	}

	if _, err = tracer.Extract(zipkintracer.Delegator, "invalid carrier"); err == nil {
		t.Fatalf("Expected: %s, got nil", opentracing.ErrInvalidCarrier)
	}
}

func TestB3Hex(t *testing.T) {
	recorder := zipkintracer.NewInMemoryRecorder()
	tracer, err := zipkintracer.NewTracer(
		recorder,
		zipkintracer.TraceID128Bit(true),
	)
	if err != nil {
		t.Fatalf("Unable to create Tracer: %+v", err)
	}

	for i := 0; i < 1000; i++ {
		headers := http.Header{}
		tmc := opentracing.HTTPHeadersCarrier(headers)
		span := tracer.StartSpan("dummy")
		if err := tracer.Inject(span.Context(), opentracing.TextMap, tmc); err != nil {
			t.Fatalf("Expected nil, got error %+v", err)
		}
		if want1, want2, have := 32, 16, len(headers["X-B3-Traceid"][0]); want1 != have && want2 != have {
			t.Errorf("X-B3-TraceId hex length expected %d or %d, got %d", want1, want2, have)
		}
		if want, have := 16, len(headers["X-B3-Spanid"][0]); want != have {
			t.Errorf("X-B3-SpanId hex length expected %d, got %d", want, have)
		}
		span.Finish()
	}
}
