package zipkintracer

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/openzipkin/zipkin-go-opentracing/thrift/gen-go/zipkincore"
)

const (
	interval    = 10 * time.Millisecond
	serverSleep = 100 * time.Millisecond
)

func TestHttpCollector(t *testing.T) {
	t.Parallel()

	port := 10000
	server := newHTTPServer(t, port)
	c, err := NewHTTPCollector(fmt.Sprintf("http://localhost:%d/api/v1/spans", port))
	if err != nil {
		t.Fatal(err)
	}

	var (
		serviceName  = "service"
		methodName   = "method"
		traceID      = int64(123)
		spanID       = int64(456)
		parentSpanID = int64(0)
		value        = "foo"
	)

	span := makeNewSpan("1.2.3.4:1234", serviceName, methodName, traceID, spanID, parentSpanID, true)
	annotate(span, time.Now(), value, nil)
	if err := c.Collect(span); err != nil {
		t.Errorf("error during collection: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("error during collection: %v", err)
	}
	if want, have := 1, len(server.spans()); want != have {
		t.Fatal("never received a span")
	}

	gotSpan := server.spans()[0]
	if want, have := methodName, gotSpan.GetName(); want != have {
		t.Errorf("want %q, have %q", want, have)
	}
	if want, have := traceID, gotSpan.TraceID; want != have {
		t.Errorf("want %d, have %d", want, have)
	}
	if want, have := spanID, gotSpan.ID; want != have {
		t.Errorf("want %d, have %d", want, have)
	}
	if want, have := parentSpanID, *gotSpan.ParentID; want != have {
		t.Errorf("want %d, have %d", want, have)
	}

	if want, have := 1, len(gotSpan.GetAnnotations()); want != have {
		t.Fatalf("want %d, have %d", want, have)
	}

	gotAnnotation := gotSpan.GetAnnotations()[0]
	if want, have := value, gotAnnotation.GetValue(); want != have {
		t.Errorf("want %q, have %q", want, have)
	}

}

func TestHttpCollector_Batch(t *testing.T) {
	t.Parallel()

	port := 10001
	server := newHTTPServer(t, port)

	var (
		batchSize   = 5
		spanTimeout = 100 * time.Millisecond
	)

	c, err := NewHTTPCollector(fmt.Sprintf("http://localhost:%d/api/v1/spans", port),
		HTTPBatchSize(batchSize),
		HTTPBatchInterval(time.Duration(2*batchSize)*spanTimeout), // Make sure timeout won't cause this test to pass
	)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < batchSize-1; i++ {
		if err := c.Collect(&zipkincore.Span{}); err != nil {
			t.Errorf("error during collection: %v", err)
		}
	}

	err = consistently(func() bool { return len(server.spans()) == 0 }, spanTimeout)
	if err != nil {
		t.Fatal("Client sent spans before batch size")
	}

	if err := c.Collect(&zipkincore.Span{}); err != nil {
		t.Errorf("error during collection: %v", err)
	}

	err = eventually(func() bool { return len(server.spans()) != batchSize }, time.Duration(batchSize)*time.Millisecond)
	if err != nil {
		t.Fatal("Client did not send spans when batch size reached")
	}
}

func TestHttpCollector_BatchInterval(t *testing.T) {
	t.Parallel()

	port := 10002
	server := newHTTPServer(t, port)

	var (
		batchSize     = 5
		batchInterval = 100 * time.Millisecond
	)

	start := time.Now()
	c, err := NewHTTPCollector(fmt.Sprintf("http://localhost:%d/api/v1/spans", port),
		HTTPBatchSize(batchSize), // Make sure batch won't make this test pass
		HTTPBatchInterval(batchInterval),
	)
	if err != nil {
		t.Fatal(err)
	}

	// send less spans than batchSize in the background
	lessThanBatchSize := batchSize - 1
	go func() {
		for i := 0; i < lessThanBatchSize; i++ {
			if err := c.Collect(&zipkincore.Span{}); err != nil {
				t.Errorf("error during collection: %v", err)
			}
		}
	}()

	beforeInterval := batchInterval - (2 * interval) - time.Now().Sub(start)
	err = consistently(func() bool { return len(server.spans()) == 0 }, beforeInterval)
	if err != nil {
		t.Fatal("Client sent spans before timeout")
	}

	afterInterval := batchInterval * 2
	err = eventually(func() bool { return len(server.spans()) == lessThanBatchSize }, afterInterval)
	if err != nil {
		t.Fatal("Client did not send spans after timeout")
	}
}

// TestHttpCollector_NonBlockCollect tests that the Collect
// function is non-blocking, even when the server is slow.
// Use of the /api/v1/sleep endpoint registered in the server.
func TestHttpCollector_NonBlockCollect(t *testing.T) {
	t.Parallel()

	port := 10003
	newHTTPServer(t, port)

	c, err := NewHTTPCollector(fmt.Sprintf("http://localhost:%d/api/v1/sleep", port))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := c.Collect(&zipkincore.Span{}); err != nil {
		t.Errorf("error during collection: %v", err)
	}

	if time.Now().Sub(start) >= serverSleep {
		t.Fatal("Collect is blocking")
	}

}

func TestHttpCollector_MaxBatchSize(t *testing.T) {
	t.Parallel()

	port := 10004
	server := newHTTPServer(t, port)

	var (
		maxBacklog = 5
		batchSize  = maxBacklog * 2 // make backsize bigger than backlog enable testing backlog disposal
	)

	c, err := NewHTTPCollector(fmt.Sprintf("http://localhost:%d/api/v1/spans", port),
		HTTPMaxBacklog(maxBacklog),
		HTTPBatchSize(batchSize),
	)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < batchSize; i++ {
		c.Collect(makeNewSpan("", "", "", 0, int64(i), 0, false))
	}
	c.Close()

	for i, s := range server.spans() {
		if want, have := int64(i+maxBacklog), s.ID; want != have {
			t.Errorf("Span ID is wrong. want %d, have %d", want, have)
		}
	}

}

func TestHTTPCollector_RequestCallback(t *testing.T) {
	t.Parallel()

	var (
		err      error
		port     = 10005
		server   = newHTTPServer(t, port)
		hdrKey   = "test-key"
		hdrValue = "test-value"
	)

	c, err := NewHTTPCollector(
		fmt.Sprintf("http://localhost:%d/api/v1/spans", port),
		HTTPRequestCallback(func(r *http.Request) {
			r.Header.Add(hdrKey, hdrValue)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Collect(&zipkincore.Span{}); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}

	if want, have := 1, len(server.spans()); want != have {
		t.Fatal("never received a span")
	}

	headers := server.headers()
	if len(headers) == 0 {
		t.Fatalf("Collect request was not handled")
	}
	testHeader := headers.Get(hdrKey)
	if !strings.EqualFold(testHeader, hdrValue) {
		t.Errorf("Custom header not received. want %s, have %s", testHeader, hdrValue)
	}
	server.clearHeaders()
}

type httpServer struct {
	t            *testing.T
	zipkinSpans  []*zipkincore.Span
	zipkinHeader http.Header
	mutex        sync.RWMutex
}

func (s *httpServer) spans() []*zipkincore.Span {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.zipkinSpans
}

func (s *httpServer) clearSpans() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.zipkinSpans = s.zipkinSpans[:0]
}

func (s *httpServer) headers() http.Header {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.zipkinHeader
}

func (s *httpServer) clearHeaders() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.zipkinHeader = make(http.Header, 0)
}

func newHTTPServer(t *testing.T, port int) *httpServer {
	server := &httpServer{
		t:           t,
		zipkinSpans: make([]*zipkincore.Span, 0),
		mutex:       sync.RWMutex{},
	}

	handler := http.NewServeMux()

	handler.HandleFunc("/api/v1/spans", func(w http.ResponseWriter, r *http.Request) {
		contextType := r.Header.Get("Content-Type")
		if contextType != "application/x-thrift" {
			t.Fatalf(
				"except Content-Type should be application/x-thrift, but is %s",
				contextType)
		}

		// clone headers from request
		headers := make(http.Header, len(r.Header))
		for k, vv := range r.Header {
			vv2 := make([]string, len(vv))
			copy(vv2, vv)
			headers[k] = vv2
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		buffer := thrift.NewTMemoryBuffer()
		if _, err = buffer.Write(body); err != nil {
			t.Error(err)
			return
		}
		transport := thrift.NewTBinaryProtocolTransport(buffer)
		_, size, err := transport.ReadListBegin()
		if err != nil {
			t.Error(err)
			return
		}
		var spans []*zipkincore.Span
		for i := 0; i < size; i++ {
			zs := &zipkincore.Span{}
			if err = zs.Read(transport); err != nil {
				t.Error(err)
				return
			}
			spans = append(spans, zs)
		}
		err = transport.ReadListEnd()
		if err != nil {
			t.Error(err)
			return
		}
		server.mutex.Lock()
		defer server.mutex.Unlock()
		server.zipkinSpans = append(server.zipkinSpans, spans...)
		server.zipkinHeader = headers
	})

	handler.HandleFunc("/api/v1/sleep", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(serverSleep)
	})

	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", port), handler)
	}()

	return server
}

func consistently(assertion func() bool, atList time.Duration) error {
	deadline := time.Now().Add(atList)
	for time.Now().Before(deadline) {
		if !assertion() {
			return fmt.Errorf("failed")
		}
		time.Sleep(interval)
	}
	return nil
}

func eventually(assertion func() bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if assertion() {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("failed")
}
