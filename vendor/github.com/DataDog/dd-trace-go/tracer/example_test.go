package tracer_test

import (
	"net/http"

	"github.com/DataDog/dd-trace-go/tracer"
)

func Example() {
	span := tracer.NewRootSpan("http.client.request", "example.com", "/user/{id}")
	defer span.Finish()

	url := "http://example.com/user/123"

	resp, err := http.Get(url)
	if err != nil {
		span.SetError(err)
		return
	}

	span.SetMeta("http.status", resp.Status)
	span.SetMeta("http.url", url)
}
