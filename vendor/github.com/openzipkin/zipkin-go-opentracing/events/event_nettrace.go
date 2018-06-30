package events

import (
	"bytes"
	"fmt"

	"golang.org/x/net/trace"

	"github.com/openzipkin/zipkin-go-opentracing"
)

// NetTraceIntegrator can be passed into a zipkintracer as NewSpanEventListener
// and causes all traces to be registered with the net/trace endpoint.
var NetTraceIntegrator = func() func(zipkintracer.SpanEvent) {
	var tr trace.Trace
	return func(e zipkintracer.SpanEvent) {
		switch t := e.(type) {
		case zipkintracer.EventCreate:
			tr = trace.New("tracing", t.OperationName)
			tr.SetMaxEvents(1000)
		case zipkintracer.EventFinish:
			tr.Finish()
		case zipkintracer.EventTag:
			tr.LazyPrintf("%s:%v", t.Key, t.Value)
		case zipkintracer.EventLogFields:
			var buf bytes.Buffer
			for i, f := range t.Fields {
				if i > 0 {
					buf.WriteByte(' ')
				}
				fmt.Fprintf(&buf, "%s:%v", f.Key(), f.Value())
			}

			tr.LazyPrintf("%s", buf.String())
		case zipkintracer.EventLog:
			if t.Payload != nil {
				tr.LazyPrintf("%s (payload %v)", t.Event, t.Payload)
			} else {
				tr.LazyPrintf("%s", t.Event)
			}
		}
	}
}
