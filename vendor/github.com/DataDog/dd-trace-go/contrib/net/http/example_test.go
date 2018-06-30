package http_test

import (
	"net/http"

	httptrace "github.com/DataDog/dd-trace-go/contrib/net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello World!\n"))
}

func Example() {
	mux := httptrace.NewServeMux("web-service", nil)
	mux.HandleFunc("/", handler)
	http.ListenAndServe(":8080", mux)
}
