package elastictraced

import (
	"fmt"
	"github.com/DataDog/dd-trace-go/tracer"
	"regexp"
)

var (
	IdRegexp         = regexp.MustCompile("/([0-9]+)([/\\?]|$)")
	IdPlaceholder    = []byte("/?$2")
	IndexRegexp      = regexp.MustCompile("[0-9]{2,}")
	IndexPlaceholder = []byte("?")
)

// Quantize quantizes an Elasticsearch to extract a meaningful resource from the request.
// We quantize based on the method+url with some cleanup applied to the URL.
// URLs with an ID will be generalized as will (potential) timestamped indices.
func Quantize(span *tracer.Span) {
	url := span.GetMeta("elasticsearch.url")
	method := span.GetMeta("elasticsearch.method")

	quantizedURL := IdRegexp.ReplaceAll([]byte(url), IdPlaceholder)
	quantizedURL = IndexRegexp.ReplaceAll(quantizedURL, IndexPlaceholder)
	span.Resource = fmt.Sprintf("%s %s", method, quantizedURL)
}
