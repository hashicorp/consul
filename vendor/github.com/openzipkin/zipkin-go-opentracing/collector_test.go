package zipkintracer

import (
	"fmt"
	"testing"
	"time"

	"github.com/openzipkin/zipkin-go-opentracing/thrift/gen-go/zipkincore"
)

var s = makeNewSpan("203.0.113.10:1234", "service1", "avg", 123, 456, 0, true)

func TestNopCollector(t *testing.T) {
	c := NopCollector{}
	if err := c.Collect(s); err != nil {
		t.Error(err)
	}
	if err := c.Close(); err != nil {
		t.Error(err)
	}
}

type stubCollector struct {
	errid     int
	collected bool
	closed    bool
}

func (c *stubCollector) Collect(*zipkincore.Span) error {
	c.collected = true
	if c.errid != 0 {
		return fmt.Errorf("error %d", c.errid)
	}
	return nil
}

func (c *stubCollector) Close() error {
	c.closed = true
	if c.errid != 0 {
		return fmt.Errorf("error %d", c.errid)
	}
	return nil
}

func TestMultiCollector(t *testing.T) {
	cs := MultiCollector{
		&stubCollector{errid: 1},
		&stubCollector{},
		&stubCollector{errid: 2},
	}
	err := cs.Collect(s)
	if err == nil {
		t.Fatal("wanted error, got none")
	}
	if want, have := "error 1; error 2", err.Error(); want != have {
		t.Errorf("want %q, have %q", want, have)
	}
	collectionError := err.(CollectionError).GetErrors()
	if want, have := 3, len(collectionError); want != have {
		t.Fatalf("want %d, have %d", want, have)
	}
	if want, have := cs[0].Collect(s).Error(), collectionError[0].Error(); want != have {
		t.Errorf("want %q, have %q", want, have)
	}
	if want, have := cs[1].Collect(s), collectionError[1]; want != have {
		t.Errorf("want %q, have %q", want, have)
	}
	if want, have := cs[2].Collect(s).Error(), collectionError[2].Error(); want != have {
		t.Errorf("want %q, have %q", want, have)
	}

	for _, c := range cs {
		if !c.(*stubCollector).collected {
			t.Error("collect not called")
		}
	}
}

func TestMultiCollectorClose(t *testing.T) {
	cs := MultiCollector{
		&stubCollector{errid: 1},
		&stubCollector{},
		&stubCollector{errid: 2},
	}
	err := cs.Close()
	if err == nil {
		t.Fatal("wanted error, got none")
	}
	if want, have := "error 1; error 2", err.Error(); want != have {
		t.Errorf("want %q, have %q", want, have)
	}

	for _, c := range cs {
		if !c.(*stubCollector).closed {
			t.Error("close not called")
		}
	}
}

func makeNewSpan(hostPort, serviceName, methodName string, traceID, spanID, parentSpanID int64, debug bool) *zipkincore.Span {
	timestamp := time.Now().UnixNano() / 1e3
	return &zipkincore.Span{
		TraceID:   traceID,
		Name:      methodName,
		ID:        spanID,
		ParentID:  &parentSpanID,
		Debug:     debug,
		Timestamp: &timestamp,
	}
}
