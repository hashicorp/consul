package tracer

const (
	// traceChanLen is the capacity of the trace channel. This channels is emptied
	// on a regular basis (worker thread) or when it reaches 50% of its capacity.
	// If it's full, then data is simply dropped and ignored, with a log message.
	// This only happens under heavy load,
	traceChanLen = 1000
	// serviceChanLen is the length of the service channel. As for the trace channel,
	// it's emptied by worker thread or when it reaches 50%. Note that there should
	// be much less data here, as service data does not be to be updated that often.
	serviceChanLen = 50
	// errChanLen is the number of errors we keep in the error channel. When this
	// one is full, errors are just ignored, dropped, nothing left. At some point,
	// there's already a whole lot of errors in the backlog, there's no real point
	// in keeping millions of errors, a representative sample is enough. And we
	// don't want to block user code and/or bloat memory or log files with redundant data.
	errChanLen = 200
)

// traceChans holds most tracer channels together, it's mostly used to
// pass them together to the span buffer/context. It's obviously safe
// to access it concurrently as it contains channels only. And it's convenient
// to have it isolated from tracer, for the sake of unit testing.
type tracerChans struct {
	trace        chan []*Span
	service      chan Service
	err          chan error
	traceFlush   chan struct{}
	serviceFlush chan struct{}
	errFlush     chan struct{}
}

func newTracerChans() tracerChans {
	return tracerChans{
		trace:        make(chan []*Span, traceChanLen),
		service:      make(chan Service, serviceChanLen),
		err:          make(chan error, errChanLen),
		traceFlush:   make(chan struct{}, 1),
		serviceFlush: make(chan struct{}, 1),
		errFlush:     make(chan struct{}, 1),
	}
}

func (tc *tracerChans) pushTrace(trace []*Span) {
	if len(tc.trace) >= cap(tc.trace)/2 { // starts being full, anticipate, try and flush soon
		select {
		case tc.traceFlush <- struct{}{}:
		default: // a flush was already requested, skip
		}
	}
	select {
	case tc.trace <- trace:
	default: // never block user code
		tc.pushErr(&errorTraceChanFull{Len: len(tc.trace)})
	}
}

func (tc *tracerChans) pushService(service Service) {
	if len(tc.service) >= cap(tc.service)/2 { // starts being full, anticipate, try and flush soon
		select {
		case tc.serviceFlush <- struct{}{}:
		default: // a flush was already requested, skip
		}
	}
	select {
	case tc.service <- service:
	default: // never block user code
		tc.pushErr(&errorServiceChanFull{Len: len(tc.service)})
	}
}

func (tc *tracerChans) pushErr(err error) {
	if len(tc.err) >= cap(tc.err)/2 { // starts being full, anticipate, try and flush soon
		select {
		case tc.errFlush <- struct{}{}:
		default: // a flush was already requested, skip
		}
	}
	select {
	case tc.err <- err:
	default:
		// OK, if we get this, our error error buffer is full,
		// we can assume it is filled with meaningful messages which
		// are going to be logged and hopefully read, nothing better
		// we can do, blocking would make things worse.
	}
}
