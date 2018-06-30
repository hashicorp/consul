package mux_test

import (
	"fmt"
	"net/http"

	muxtrace "github.com/DataDog/dd-trace-go/contrib/gorilla/mux"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/gorilla/mux"
)

// handler is a simple handlerFunc that logs some data from the span
// that is injected into the requests' context.
func handler(w http.ResponseWriter, r *http.Request) {
	span := tracer.SpanFromContextDefault(r.Context())
	fmt.Printf("tracing service:%s resource:%s", span.Service, span.Resource)
	w.Write([]byte("hello world"))
}

func Example() {
	router := mux.NewRouter()
	muxTracer := muxtrace.NewMuxTracer("my-web-app", tracer.DefaultTracer)

	// Add traced routes directly.
	muxTracer.HandleFunc(router, "/users", handler)

	// and subroutes as well.
	subrouter := router.PathPrefix("/user").Subrouter()
	muxTracer.HandleFunc(subrouter, "/view", handler)
	muxTracer.HandleFunc(subrouter, "/create", handler)
}
