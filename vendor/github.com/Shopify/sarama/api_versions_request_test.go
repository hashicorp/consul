package sarama

import "testing"

var (
	apiVersionRequest = []byte{}
)

func TestApiVersionsRequest(t *testing.T) {
	var request *ApiVersionsRequest

	request = new(ApiVersionsRequest)
	testRequest(t, "basic", request, apiVersionRequest)
}
