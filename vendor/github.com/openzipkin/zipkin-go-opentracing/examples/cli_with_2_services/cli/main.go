// +build go1.7

package main

import (
	"context"
	"fmt"
	"os"

	opentracing "github.com/opentracing/opentracing-go"

	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/examples/cli_with_2_services/svc1"
)

const (
	// Our service name.
	serviceName = "cli"

	// Host + port of our service.
	hostPort = "0.0.0.0:0"

	// Endpoint to send Zipkin spans to.
	zipkinHTTPEndpoint = "http://localhost:9411/api/v1/spans"

	// Debug mode.
	debug = false

	// Base endpoint of our SVC1 service.
	svc1Endpoint = "http://localhost:61001"

	// same span can be set to true for RPC style spans (Zipkin V1) vs Node style (OpenTracing)
	sameSpan = true

	// make Tracer generate 128 bit traceID's for root spans.
	traceID128Bit = true
)

//ci
func main() {
	// Create our HTTP collector.
	collector, err := zipkin.NewHTTPCollector(zipkinHTTPEndpoint)
	if err != nil {
		fmt.Printf("unable to create Zipkin HTTP collector: %+v\n", err)
		os.Exit(-1)
	}

	// Create our recorder.
	recorder := zipkin.NewRecorder(collector, debug, hostPort, serviceName)

	// Create our tracer.
	tracer, err := zipkin.NewTracer(
		recorder,
		zipkin.ClientServerSameSpan(sameSpan),
		zipkin.TraceID128Bit(traceID128Bit),
	)
	if err != nil {
		fmt.Printf("unable to create Zipkin tracer: %+v\n", err)
		os.Exit(-1)
	}

	// Explicitly set our tracer to be the default tracer.
	opentracing.InitGlobalTracer(tracer)

	// Create Client to svc1 Service
	client := svc1.NewHTTPClient(tracer, svc1Endpoint)

	// Create Root Span for duration of the interaction with svc1
	span := opentracing.StartSpan("Run")

	// Put root span in context so it will be used in our calls to the client.
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	// Call the Concat Method
	span.LogEvent("Call Concat")
	res1, err := client.Concat(ctx, "Hello", " World!")
	fmt.Printf("Concat: %s Err: %+v\n", res1, err)

	// Call the Sum Method
	span.LogEvent("Call Sum")
	res2, err := client.Sum(ctx, 10, 20)
	fmt.Printf("Sum: %d Err: %+v\n", res2, err)

	// Finish our CLI span
	span.Finish()

	// Close collector to ensure spans are sent before exiting.
	collector.Close()
}
