package tracer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testInitSize = 2
	testMaxSize  = 5
)

func TestSpanBufferPushOne(t *testing.T) {
	assert := assert.New(t)

	buffer := newSpanBuffer(newTracerChans(), testInitSize, testMaxSize)
	assert.NotNil(buffer)
	assert.Len(buffer.spans, 0)

	traceID := NextSpanID()
	root := NewSpan("name1", "a-service", "a-resource", traceID, traceID, 0, nil)
	root.buffer = buffer

	buffer.Push(root)
	assert.Len(buffer.spans, 1, "there is one span in the buffer")
	assert.Equal(root, buffer.spans[0], "the span is the one pushed before")

	root.Finish()

	select {
	case trace := <-buffer.channels.trace:
		assert.Len(trace, 1, "there was a trace in the channel")
		assert.Equal(root, trace[0], "the trace in the channel is the one pushed before")
		assert.Equal(0, buffer.Len(), "no more spans in the buffer")
	case err := <-buffer.channels.err:
		assert.Fail("unexpected error:", err.Error())
		t.Logf("buffer: %v", buffer)
	}
}

func TestSpanBufferPushNoFinish(t *testing.T) {
	assert := assert.New(t)

	buffer := newSpanBuffer(newTracerChans(), testInitSize, testMaxSize)
	assert.NotNil(buffer)
	assert.Len(buffer.spans, 0)

	traceID := NextSpanID()
	root := NewSpan("name1", "a-service", "a-resource", traceID, traceID, 0, nil)
	root.buffer = buffer

	buffer.Push(root)
	assert.Len(buffer.spans, 1, "there is one span in the buffer")
	assert.Equal(root, buffer.spans[0], "the span is the one pushed before")

	select {
	case <-buffer.channels.trace:
		assert.Fail("span was not finished, should not be flushed")
		t.Logf("buffer: %v", buffer)
	case err := <-buffer.channels.err:
		assert.Fail("unexpected error:", err.Error())
		t.Logf("buffer: %v", buffer)
	case <-time.After(time.Second / 10):
		t.Logf("expected timeout, nothing should show up in buffer as the trace is not finished")
	}
}

func TestSpanBufferPushSeveral(t *testing.T) {
	assert := assert.New(t)

	buffer := newSpanBuffer(newTracerChans(), testInitSize, testMaxSize)
	assert.NotNil(buffer)
	assert.Len(buffer.spans, 0)

	traceID := NextSpanID()
	root := NewSpan("name1", "a-service", "a-resource", traceID, traceID, 0, nil)
	span2 := NewSpan("name2", "a-service", "a-resource", NextSpanID(), traceID, root.SpanID, nil)
	span3 := NewSpan("name3", "a-service", "a-resource", NextSpanID(), traceID, root.SpanID, nil)
	span3a := NewSpan("name3", "a-service", "a-resource", NextSpanID(), traceID, span3.SpanID, nil)

	spans := []*Span{root, span2, span3, span3a}

	for i, span := range spans {
		span.buffer = buffer
		buffer.Push(span)
		assert.Len(buffer.spans, i+1, "there is one more span in the buffer")
		assert.Equal(span, buffer.spans[i], "the span is the one pushed before")
	}

	for _, span := range spans {
		span.Finish()
	}

	select {
	case trace := <-buffer.channels.trace:
		assert.Len(trace, 4, "there was one trace with the right number of spans in the channel")
		for _, span := range spans {
			assert.Contains(trace, span, "the trace contains the spans")
		}
	case err := <-buffer.channels.err:
		assert.Fail("unexpected error:", err.Error())
	}
}
