package elastictrace_test

import (
	"context"
	elastictrace "github.com/DataDog/dd-trace-go/contrib/olivere/elastic"
	"github.com/DataDog/dd-trace-go/tracer"
	elasticv3 "gopkg.in/olivere/elastic.v3"
	elasticv5 "gopkg.in/olivere/elastic.v5"
)

// To start tracing elastic.v5 requests, create a new TracedHTTPClient that you will
// use when initializing the elastic.Client.
func Example_v5() {
	tc := elastictrace.NewTracedHTTPClient("my-elasticsearch-service", tracer.DefaultTracer)
	client, _ := elasticv5.NewClient(
		elasticv5.SetURL("http://127.0.0.1:9200"),
		elasticv5.SetHttpClient(tc),
	)

	// Spans are emitted for all
	client.Index().
		Index("twitter").Type("tweet").Index("1").
		BodyString(`{"user": "test", "message": "hello"}`).
		Do(context.Background())

	// Use a context to pass information down the call chain
	root := tracer.NewRootSpan("parent.request", "web", "/tweet/1")
	ctx := root.Context(context.Background())
	client.Get().
		Index("twitter").Type("tweet").Index("1").
		Do(ctx)
	root.Finish()
}

// To trace elastic.v3 you create a TracedHTTPClient in the same way but all requests must use
// the DoC() call to pass the request context.
func Example_v3() {
	tc := elastictrace.NewTracedHTTPClient("my-elasticsearch-service", tracer.DefaultTracer)
	client, _ := elasticv3.NewClient(
		elasticv3.SetURL("http://127.0.0.1:9200"),
		elasticv3.SetHttpClient(tc),
	)

	// Spans are emitted for all
	client.Index().
		Index("twitter").Type("tweet").Index("1").
		BodyString(`{"user": "test", "message": "hello"}`).
		DoC(context.Background())

	// Use a context to pass information down the call chain
	root := tracer.NewRootSpan("parent.request", "web", "/tweet/1")
	ctx := root.Context(context.Background())
	client.Get().
		Index("twitter").Type("tweet").Index("1").
		DoC(ctx)
	root.Finish()
}
