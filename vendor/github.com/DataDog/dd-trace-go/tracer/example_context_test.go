package tracer_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/DataDog/dd-trace-go/tracer"
)

func saveFile(ctx context.Context, path string, r io.Reader) error {
	// Start a new span that is the child of the span stored in the context, and
	// attach it to the current context. If the context has no span, it will
	// return an empty root span.
	span, ctx := tracer.NewChildSpanWithContext("filestore.saveFile", ctx)
	defer span.Finish()

	// save the file contents.
	file, err := os.Create(path)
	if err != nil {
		span.SetError(err)
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, r)
	span.SetError(err)
	return err
}

func saveFileHandler(w http.ResponseWriter, r *http.Request) {
	// the name of the operation we're measuring
	name := "http.request"
	service := "example-filestore"
	resource := "/saveFile"

	// This is the main entry point of our application, so we create a root span
	// that includes the service and resource name.
	span := tracer.NewRootSpan(name, service, resource)
	defer span.Finish()

	// Add the span to the request's context so we can pass the tracing information
	// down the stack.
	ctx := span.Context(r.Context())

	// Do the work.
	err := saveFile(ctx, "/tmp/example", r.Body)
	span.SetError(err) // no-op if err == nil

	if err != nil {
		http.Error(w, fmt.Sprintf("error saving file! %s", err), 500)
		return
	}

	w.Write([]byte("saved file!"))
}

// Tracing the hierarchy of spans in a request is a key part of tracing. This, for example,
// let's a developer associate all of the database calls in a web request. As of Go 1.7,
// the standard way of doing this is with the context package. Along with supporting
// deadlines, cancellation signals and more, Contexts are perfect for passing (optional)
// telemetry data through your stack.
//
// Read more about contexts here: https://golang.org/pkg/context/
//
// Here is an example illustrating how to pass tracing data with contexts.
func Example_context() {
	http.HandleFunc("/saveFile", saveFileHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
