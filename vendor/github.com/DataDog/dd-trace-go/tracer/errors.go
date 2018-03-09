package tracer

import (
	"log"
	"strconv"
)

const (
	errorPrefix = "Datadog Tracer Error: "
)

// errorSpanBufFull is raised when there's no more room in the buffer
type errorSpanBufFull struct {
	// Len is the length of the buffer (which is full)
	Len int
}

// Error provides a readable error message.
func (e *errorSpanBufFull) Error() string {
	return "span buffer is full (length: " + strconv.Itoa(e.Len) + ")"
}

// errorTraceChanFull is raised when there's no more room in the channel
type errorTraceChanFull struct {
	// Len is the length of the channel (which is full)
	Len int
}

// Error provides a readable error message.
func (e *errorTraceChanFull) Error() string {
	return "trace channel is full (length: " + strconv.Itoa(e.Len) + ")"
}

// errorServiceChanFull is raised when there's no more room in the channel
type errorServiceChanFull struct {
	// Len is the length of the channel (which is full)
	Len int
}

// Error provides a readable error message.
func (e *errorServiceChanFull) Error() string {
	return "service channel is full (length: " + strconv.Itoa(e.Len) + ")"
}

// errorTraceIDMismatch is raised when a trying to put a span in the wrong place.
type errorTraceIDMismatch struct {
	// Expected is the trace ID we should have.
	Expected uint64
	// Actual is the trace ID we have and is wrong.
	Actual uint64
}

// Error provides a readable error message.
func (e *errorTraceIDMismatch) Error() string {
	return "trace ID mismatch (expected: " +
		strconv.FormatUint(e.Expected, 16) +
		" actual: " +
		strconv.FormatUint(e.Actual, 16) +
		")"
}

// errorNoSpanBuf is raised when trying to finish/push a span that has no buffer associated to it.
type errorNoSpanBuf struct {
	// SpanName is the name of the span which could not be pushed (hint for the log reader).
	SpanName string
}

// Error provides a readable error message.
func (e *errorNoSpanBuf) Error() string {
	return "no span buffer (span name: '" + e.SpanName + "')"
}

// errorFlushLostTraces is raised when trying to finish/push a span that has no buffer associated to it.
type errorFlushLostTraces struct {
	// Nb is the number of traces lost in that flush
	Nb int
}

// Error provides a readable error message.
func (e *errorFlushLostTraces) Error() string {
	return "unable to flush traces, lost " + strconv.Itoa(e.Nb) + " traces"
}

// errorFlushLostServices is raised when trying to finish/push a span that has no buffer associated to it.
type errorFlushLostServices struct {
	// Nb is the number of services lost in that flush
	Nb int
}

// Error provides a readable error message.
func (e *errorFlushLostServices) Error() string {
	return "unable to flush services, lost " + strconv.Itoa(e.Nb) + " services"
}

type errorSummary struct {
	Count   int
	Example string
}

// errorKey returns a unique key for each error type
func errorKey(err error) string {
	if err == nil {
		return ""
	}
	switch err.(type) {
	case *errorSpanBufFull:
		return "ErrorSpanBufFull"
	case *errorTraceChanFull:
		return "ErrorTraceChanFull"
	case *errorServiceChanFull:
		return "ErrorServiceChanFull"
	case *errorTraceIDMismatch:
		return "ErrorTraceIDMismatch"
	case *errorNoSpanBuf:
		return "ErrorNoSpanBuf"
	case *errorFlushLostTraces:
		return "ErrorFlushLostTraces"
	case *errorFlushLostServices:
		return "ErrorFlushLostServices"
	}
	return err.Error() // possibly high cardinality, but this is unexpected
}

func aggregateErrors(errChan <-chan error) map[string]errorSummary {
	errs := make(map[string]errorSummary, len(errChan))

	for {
		select {
		case err := <-errChan:
			if err != nil { // double-checking, we don't want to panic here...
				key := errorKey(err)
				summary := errs[key]
				summary.Count++
				summary.Example = err.Error()
				errs[key] = summary
			}
		default: // stop when there's no more data
			return errs
		}
	}
}

// logErrors logs the errors, preventing log file flooding, when there
// are many messages, it caps them and shows a quick summary.
// As of today it only logs using standard golang log package, but
// later we could send those stats to agent [TODO:christian].
func logErrors(errChan <-chan error) {
	errs := aggregateErrors(errChan)

	for _, v := range errs {
		var repeat string
		if v.Count > 1 {
			repeat = " (repeated " + strconv.Itoa(v.Count) + " times)"
		}
		log.Println(errorPrefix + v.Example + repeat)
	}
}
