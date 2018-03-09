package tracer

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// getTestSpan returns a Span with different fields set
func getTestSpan() *Span {
	return &Span{
		TraceID:  42,
		SpanID:   52,
		ParentID: 42,
		Type:     "web",
		Service:  "high.throughput",
		Name:     "sending.events",
		Resource: "SEND /data",
		Start:    1481215590883401105,
		Duration: 1000000000,
		Meta:     map[string]string{"http.host": "192.168.0.1"},
		Metrics:  map[string]float64{"http.monitor": 41.99},
	}
}

// getTestTrace returns a list of traces that is composed by ``traceN`` number
// of traces, each one composed by ``size`` number of spans.
func getTestTrace(traceN, size int) [][]*Span {
	var traces [][]*Span

	for i := 0; i < traceN; i++ {
		trace := []*Span{}
		for j := 0; j < size; j++ {
			trace = append(trace, getTestSpan())
		}
		traces = append(traces, trace)
	}
	return traces
}

func getTestServices() map[string]Service {
	return map[string]Service{
		"svc1": Service{Name: "scv1", App: "a", AppType: "b"},
		"svc2": Service{Name: "scv2", App: "c", AppType: "d"},
	}
}

type mockDatadogAPIHandler struct {
	t *testing.T
}

func (m mockDatadogAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	assert := assert.New(m.t)

	header := r.Header.Get("X-Datadog-Trace-Count")
	assert.NotEqual("", header, "X-Datadog-Trace-Count header should be here")
	count, err := strconv.Atoi(header)
	assert.Nil(err, "header should be an int")
	assert.NotEqual(0, count, "there should be a non-zero amount of traces")
}

func mockDatadogAPINewServer(t *testing.T) *httptest.Server {
	handler := mockDatadogAPIHandler{t: t}
	server := httptest.NewServer(handler)
	return server
}

func TestTracesAgentIntegration(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		payload [][]*Span
	}{
		{getTestTrace(1, 1)},
		{getTestTrace(10, 1)},
		{getTestTrace(1, 10)},
		{getTestTrace(10, 10)},
	}

	for _, tc := range testCases {
		transport := newHTTPTransport(defaultHostname, defaultPort)
		response, err := transport.SendTraces(tc.payload)
		assert.NoError(err)
		assert.NotNil(response)
		assert.Equal(200, response.StatusCode)
	}
}

func TestAPIDowngrade(t *testing.T) {
	assert := assert.New(t)
	transport := newHTTPTransport(defaultHostname, defaultPort)
	transport.traceURL = "http://localhost:8126/v0.0/traces"

	// if we get a 404 we should downgrade the API
	traces := getTestTrace(2, 2)
	response, err := transport.SendTraces(traces)
	assert.NoError(err)
	assert.NotNil(response)
	assert.Equal(200, response.StatusCode)
}

func TestEncoderDowngrade(t *testing.T) {
	assert := assert.New(t)
	transport := newHTTPTransport(defaultHostname, defaultPort)
	transport.traceURL = "http://localhost:8126/v0.2/traces"

	// if we get a 415 because of a wrong encoder, we should downgrade the encoder
	traces := getTestTrace(2, 2)
	response, err := transport.SendTraces(traces)
	assert.NoError(err)
	assert.NotNil(response)
	assert.Equal(200, response.StatusCode)
}

func TestTransportServices(t *testing.T) {
	assert := assert.New(t)

	transport := newHTTPTransport(defaultHostname, defaultPort)

	response, err := transport.SendServices(getTestServices())
	assert.NoError(err)
	assert.NotNil(response)
	assert.Equal(200, response.StatusCode)
}

func TestTransportServicesDowngrade_0_0(t *testing.T) {
	assert := assert.New(t)

	transport := newHTTPTransport(defaultHostname, defaultPort)
	transport.serviceURL = "http://localhost:8126/v0.0/services"

	response, err := transport.SendServices(getTestServices())
	assert.NoError(err)
	assert.NotNil(response)
	assert.Equal(200, response.StatusCode)
}

func TestTransportServicesDowngrade_0_2(t *testing.T) {
	assert := assert.New(t)

	transport := newHTTPTransport(defaultHostname, defaultPort)
	transport.serviceURL = "http://localhost:8126/v0.2/services"

	response, err := transport.SendServices(getTestServices())
	assert.NoError(err)
	assert.NotNil(response)
	assert.Equal(200, response.StatusCode)
}

func TestTransportEncoderPool(t *testing.T) {
	assert := assert.New(t)
	transport := newHTTPTransport(defaultHostname, defaultPort)

	// MsgpackEncoder is the default encoder of the pool
	encoder := transport.getEncoder()
	assert.Equal("application/msgpack", encoder.ContentType())
}

func TestTransportSwitchEncoder(t *testing.T) {
	assert := assert.New(t)
	transport := newHTTPTransport(defaultHostname, defaultPort)
	transport.changeEncoder(jsonEncoderFactory)

	// MsgpackEncoder is the default encoder of the pool
	encoder := transport.getEncoder()
	assert.Equal("application/json", encoder.ContentType())
}

func TestTraceCountHeader(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		payload [][]*Span
	}{
		{getTestTrace(1, 1)},
		{getTestTrace(10, 1)},
		{getTestTrace(100, 10)},
	}

	receiver := mockDatadogAPINewServer(t)
	parsedURL, err := url.Parse(receiver.URL)
	assert.NoError(err)
	host := parsedURL.Host
	hostItems := strings.Split(host, ":")
	assert.Equal(2, len(hostItems), "port should be given, as it's chosen randomly")
	hostname := hostItems[0]
	port := hostItems[1]
	for _, tc := range testCases {
		transport := newHTTPTransport(hostname, port)
		response, err := transport.SendTraces(tc.payload)
		assert.NoError(err)
		assert.NotNil(response)
		assert.Equal(200, response.StatusCode)
	}

	receiver.Close()
}
