package tracertest

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/stretchr/testify/assert"
)

// CopySpan returns a new span with the same fields of the copied one.
// This function is necessary because the usual assignment copies the mutex address
// and then the use of the copied span can conflict with the original one when concurent calls.
func CopySpan(span *tracer.Span, trc *tracer.Tracer) *tracer.Span {
	newSpan := tracer.NewSpan(span.Name, span.Service, span.Resource, span.SpanID, span.TraceID, span.ParentID, trc)
	newSpan.Type = ext.SQLType
	newSpan.Meta = span.Meta
	return newSpan
}

// Test strict equality between the most important fields of the two spans
func CompareSpan(t *testing.T, expectedSpan, actualSpan *tracer.Span, debug ...bool) {
	if len(debug) > 0 && debug[0] {
		fmt.Printf("-> ExpectedSpan: \n%s\n\n", expectedSpan)
	}
	assert := assert.New(t)
	assert.Equal(expectedSpan.Name, actualSpan.Name)
	assert.Equal(expectedSpan.Service, actualSpan.Service)
	assert.Equal(expectedSpan.Resource, actualSpan.Resource)
	assert.Equal(expectedSpan.Type, actualSpan.Type)
	assert.True(reflect.DeepEqual(expectedSpan.Meta, actualSpan.Meta), fmt.Sprintf("%v != %v", expectedSpan.Meta, actualSpan.Meta))
}

// Return a Tracer with a DummyTransport
func GetTestTracer() (*tracer.Tracer, *DummyTransport) {
	transport := &DummyTransport{}
	tracer := tracer.NewTracerTransport(transport)
	return tracer, transport
}

// dummyTransport is a transport that just buffers spans and encoding
type DummyTransport struct {
	traces   [][]*tracer.Span
	services map[string]tracer.Service
}

func (t *DummyTransport) SendTraces(traces [][]*tracer.Span) (*http.Response, error) {
	t.traces = append(t.traces, traces...)
	return nil, nil
}

func (t *DummyTransport) SendServices(services map[string]tracer.Service) (*http.Response, error) {
	t.services = services
	return nil, nil
}

func (t *DummyTransport) Traces() [][]*tracer.Span {
	traces := t.traces
	t.traces = nil
	return traces
}

func (t *DummyTransport) SetHeader(key, value string) {}
