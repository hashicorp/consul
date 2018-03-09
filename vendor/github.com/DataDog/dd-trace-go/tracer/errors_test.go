package tracer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorSpanBufFull(t *testing.T) {
	assert := assert.New(t)

	err := &errorSpanBufFull{Len: 42}
	assert.Equal("span buffer is full (length: 42)", err.Error())
	assert.Equal("ErrorSpanBufFull", errorKey(err))
}

func TestErrorTraceChanFull(t *testing.T) {
	assert := assert.New(t)

	err := &errorTraceChanFull{Len: 42}
	assert.Equal("trace channel is full (length: 42)", err.Error())
	assert.Equal("ErrorTraceChanFull", errorKey(err))
}

func TestErrorServiceChanFull(t *testing.T) {
	assert := assert.New(t)

	err := &errorServiceChanFull{Len: 42}
	assert.Equal("service channel is full (length: 42)", err.Error())
	assert.Equal("ErrorServiceChanFull", errorKey(err))
}

func TestErrorTraceIDMismatch(t *testing.T) {
	assert := assert.New(t)

	err := &errorTraceIDMismatch{Expected: 42, Actual: 65535}
	assert.Equal("trace ID mismatch (expected: 2a actual: ffff)", err.Error())
	assert.Equal("ErrorTraceIDMismatch", errorKey(err))
}

func TestErrorNoSpanBuf(t *testing.T) {
	assert := assert.New(t)

	err := &errorNoSpanBuf{SpanName: "do"}
	assert.Equal("no span buffer (span name: 'do')", err.Error())
}

func TestErrorFlushLostTraces(t *testing.T) {
	assert := assert.New(t)

	err := &errorFlushLostTraces{Nb: 100}
	assert.Equal("unable to flush traces, lost 100 traces", err.Error())
}

func TestErrorFlushLostServices(t *testing.T) {
	assert := assert.New(t)

	err := &errorFlushLostServices{Nb: 100}
	assert.Equal("unable to flush services, lost 100 services", err.Error())
}

func TestErrorKey(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("this is something unexpected", errorKey(fmt.Errorf("this is something unexpected")))
	assert.Equal("", errorKey(nil))
}

func TestAggregateErrors(t *testing.T) {
	assert := assert.New(t)

	errChan := make(chan error, 100)
	errChan <- &errorSpanBufFull{Len: 1000}
	errChan <- &errorSpanBufFull{Len: 1000}
	errChan <- &errorSpanBufFull{Len: 1000}
	errChan <- &errorSpanBufFull{Len: 1000}
	errChan <- &errorFlushLostTraces{Nb: 42}
	errChan <- &errorTraceIDMismatch{Expected: 42, Actual: 1}
	errChan <- &errorTraceIDMismatch{Expected: 42, Actual: 4095}

	errs := aggregateErrors(errChan)

	assert.Equal(map[string]errorSummary{
		"ErrorSpanBufFull": errorSummary{
			Count:   4,
			Example: "span buffer is full (length: 1000)",
		},
		"ErrorTraceIDMismatch": errorSummary{
			Count:   2,
			Example: "trace ID mismatch (expected: 2a actual: fff)",
		},
		"ErrorFlushLostTraces": errorSummary{
			Count:   1,
			Example: "unable to flush traces, lost 42 traces",
		},
	}, errs)
}
