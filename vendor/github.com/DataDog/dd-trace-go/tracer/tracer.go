package tracer

import (
	"context"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/DataDog/dd-trace-go/tracer/ext"
)

const (
	flushInterval = 2 * time.Second
)

func init() {
	randGen = rand.New(newRandSource())
}

type Service struct {
	Name    string `json:"-"`        // the internal of the service (e.g. acme_search, datadog_web)
	App     string `json:"app"`      // the name of the application (e.g. rails, postgres, custom-app)
	AppType string `json:"app_type"` // the type of the application (e.g. db, web)
}

func (s Service) Equal(s2 Service) bool {
	return s.Name == s2.Name && s.App == s2.App && s.AppType == s2.AppType
}

// Tracer creates, buffers and submits Spans which are used to time blocks of
// compuration.
//
// When a tracer is disabled, it will not submit spans for processing.
type Tracer struct {
	transport Transport // is the transport mechanism used to delivery spans to the agent
	sampler   sampler   // is the trace sampler to only keep some samples

	debugMu             sync.RWMutex // protects the debugLoggingEnabled attribute while allowing concurrent reads
	debugLoggingEnabled bool

	enableMu sync.RWMutex
	enabled  bool // defines if the Tracer is enabled or not

	meta   map[string]string
	metaMu sync.RWMutex

	channels tracerChans
	services map[string]Service // name -> service

	exit   chan struct{}
	exitWG *sync.WaitGroup

	forceFlushIn  chan struct{}
	forceFlushOut chan struct{}
}

// NewTracer creates a new Tracer. Most users should use the package's
// DefaultTracer instance.
func NewTracer() *Tracer {
	return NewTracerTransport(newDefaultTransport())
}

// NewTracerTransport create a new Tracer with the given transport.
func NewTracerTransport(transport Transport) *Tracer {
	t := &Tracer{
		enabled:             true,
		transport:           transport,
		sampler:             newAllSampler(),
		debugLoggingEnabled: false,

		channels: newTracerChans(),

		services: make(map[string]Service),

		exit:   make(chan struct{}),
		exitWG: &sync.WaitGroup{},

		forceFlushIn:  make(chan struct{}, 0), // must be size 0 (blocking)
		forceFlushOut: make(chan struct{}, 0), // must be size 0 (blocking)
	}

	// start a background worker
	t.exitWG.Add(1)
	go t.worker()

	return t
}

// Stop stops the tracer.
func (t *Tracer) Stop() {
	close(t.exit)
	t.exitWG.Wait()
}

// SetEnabled will enable or disable the tracer.
func (t *Tracer) SetEnabled(enabled bool) {
	t.enableMu.Lock()
	defer t.enableMu.Unlock()
	t.enabled = enabled
}

// Enabled returns whether or not a tracer is enabled.
func (t *Tracer) Enabled() bool {
	t.enableMu.RLock()
	defer t.enableMu.RUnlock()
	return t.enabled
}

// SetSampleRate sets a sample rate for all the future traces.
// sampleRate has to be between 0.0 and 1.0 and represents the ratio of traces
// that will be sampled. 0.0 means that the tracer won't send any trace. 1.0
// means that the tracer will send all traces.
func (t *Tracer) SetSampleRate(sampleRate float64) {
	if sampleRate == 1 {
		t.sampler = newAllSampler()
	} else if sampleRate >= 0 && sampleRate < 1 {
		t.sampler = newRateSampler(sampleRate)
	} else {
		log.Printf("tracer.SetSampleRate rate must be between 0 and 1, now: %f", sampleRate)
	}
}

// SetServiceInfo update the application and application type for the given
// service.
func (t *Tracer) SetServiceInfo(name, app, appType string) {
	t.channels.pushService(Service{
		Name:    name,
		App:     app,
		AppType: appType,
	})
}

// SetMeta adds an arbitrary meta field at the tracer level.
// This will append those tags to each span created by the tracer.
func (t *Tracer) SetMeta(key, value string) {
	if t == nil { // Defensive, span could be initialized with nil tracer
		return
	}

	t.metaMu.Lock()
	if t.meta == nil {
		t.meta = make(map[string]string)
	}
	t.meta[key] = value
	t.metaMu.Unlock()
}

// getAllMeta returns all the meta set by this tracer.
// In most cases, it is nil.
func (t *Tracer) getAllMeta() map[string]string {
	if t == nil { // Defensive, span could be initialized with nil tracer
		return nil
	}

	var meta map[string]string

	t.metaMu.RLock()
	if t.meta != nil {
		meta = make(map[string]string, len(t.meta))
		for key, value := range t.meta {
			meta[key] = value
		}
	}
	t.metaMu.RUnlock()

	return meta
}

// NewRootSpan creates a span with no parent. Its ids will be randomly
// assigned.
func (t *Tracer) NewRootSpan(name, service, resource string) *Span {
	spanID := NextSpanID()
	span := NewSpan(name, service, resource, spanID, spanID, 0, t)

	span.buffer = newSpanBuffer(t.channels, 0, 0)
	t.Sample(span)
	// [TODO:christian] introduce distributed sampling here
	span.buffer.Push(span)

	// Add the process id to all root spans
	span.SetMeta(ext.Pid, strconv.Itoa(os.Getpid()))

	return span
}

// NewChildSpan returns a new span that is child of the Span passed as
// argument.
func (t *Tracer) NewChildSpan(name string, parent *Span) *Span {
	spanID := NextSpanID()

	// when we're using parenting in inner functions, it's possible that
	// a nil pointer is sent to this function as argument. To prevent a crash,
	// it's better to be defensive and to produce a wrongly configured span
	// that is not sent to the trace agent.
	if parent == nil {
		span := NewSpan(name, "", name, spanID, spanID, spanID, t)

		span.buffer = newSpanBuffer(t.channels, 0, 0)
		t.Sample(span)
		// [TODO:christian] introduce distributed sampling here
		span.buffer.Push(span)

		return span
	}

	parent.RLock()
	// child that is correctly configured
	span := NewSpan(name, parent.Service, name, spanID, parent.TraceID, parent.SpanID, parent.tracer)

	// child sampling same as the parent
	span.Sampled = parent.Sampled
	if parent.HasSamplingPriority() {
		span.SetSamplingPriority(parent.GetSamplingPriority())
	}

	span.parent = parent
	span.buffer = parent.buffer
	parent.RUnlock()

	span.buffer.Push(span)

	return span
}

// NewChildSpanFromContext will create a child span of the span contained in
// the given context. If the context contains no span, an empty span will be
// returned.
func (t *Tracer) NewChildSpanFromContext(name string, ctx context.Context) *Span {
	span, _ := SpanFromContext(ctx) // tolerate nil spans
	return t.NewChildSpan(name, span)
}

// NewChildSpanWithContext will create and return a child span of the span contained in the given
// context, as well as a copy of the parent context containing the created
// child span. If the context contains no span, an empty root span will be returned.
// If nil is passed in for the context, a context will be created.
func (t *Tracer) NewChildSpanWithContext(name string, ctx context.Context) (*Span, context.Context) {
	span := t.NewChildSpanFromContext(name, ctx)
	return span, span.Context(ctx)
}

// SetDebugLogging will set the debug level
func (t *Tracer) SetDebugLogging(debug bool) {
	t.debugMu.Lock()
	defer t.debugMu.Unlock()
	t.debugLoggingEnabled = debug
}

// DebugLoggingEnabled returns true if the debug level is enabled and false otherwise.
func (t *Tracer) DebugLoggingEnabled() bool {
	t.debugMu.RLock()
	defer t.debugMu.RUnlock()
	return t.debugLoggingEnabled
}

func (t *Tracer) getTraces() [][]*Span {
	traces := make([][]*Span, 0, len(t.channels.trace))

	for {
		select {
		case trace := <-t.channels.trace:
			traces = append(traces, trace)
		default: // return when there's no more data
			return traces
		}
	}
}

// flushTraces will push any currently buffered traces to the server.
func (t *Tracer) flushTraces() {
	traces := t.getTraces()

	if t.DebugLoggingEnabled() {
		log.Printf("Sending %d traces", len(traces))
		for _, trace := range traces {
			if len(trace) > 0 {
				log.Printf("TRACE: %d\n", trace[0].TraceID)
				for _, span := range trace {
					log.Printf("SPAN:\n%s", span.String())
				}
			}
		}
	}

	// bal if there's nothing to do
	if !t.Enabled() || t.transport == nil || len(traces) == 0 {
		return
	}

	_, err := t.transport.SendTraces(traces)
	if err != nil {
		t.channels.pushErr(err)
		t.channels.pushErr(&errorFlushLostTraces{Nb: len(traces)}) // explicit log messages with nb of lost traces
	}
}

func (t *Tracer) updateServices() bool {
	servicesModified := false
	for {
		select {
		case service := <-t.channels.service:
			if s, found := t.services[service.Name]; !found || !s.Equal(service) {
				t.services[service.Name] = service
				servicesModified = true
			}
		default: // return when there's no more data
			return servicesModified
		}
	}
}

// flushTraces will push any currently buffered services to the server.
func (t *Tracer) flushServices() {
	servicesModified := t.updateServices()

	if !t.Enabled() || !servicesModified {
		return
	}

	_, err := t.transport.SendServices(t.services)
	if err != nil {
		t.channels.pushErr(err)
		t.channels.pushErr(&errorFlushLostServices{Nb: len(t.services)}) // explicit log messages with nb of lost services
	}
}

// flushErrs will process log messages that were queued
func (t *Tracer) flushErrs() {
	logErrors(t.channels.err)
}

func (t *Tracer) flush() {
	t.flushTraces()
	t.flushServices()
	t.flushErrs()
}

// ForceFlush forces a flush of data (traces and services) to the agent.
// Flushes are done by a background task on a regular basis, so you never
// need to call this manually, mostly useful for testing and debugging.
func (t *Tracer) ForceFlush() {
	t.forceFlushIn <- struct{}{}
	<-t.forceFlushOut
}

// Sample samples a span with the internal sampler.
func (t *Tracer) Sample(span *Span) {
	t.sampler.Sample(span)
}

// worker periodically flushes traces and services to the transport.
func (t *Tracer) worker() {
	defer t.exitWG.Done()

	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-flushTicker.C:
			t.flush()

		case <-t.forceFlushIn:
			t.flush()
			t.forceFlushOut <- struct{}{} // caller blocked until this is done

		case <-t.channels.traceFlush:
			t.flushTraces()

		case <-t.channels.serviceFlush:
			t.flushServices()

		case <-t.channels.errFlush:
			t.flushErrs()

		case <-t.exit:
			t.flush()
			return
		}
	}
}

// DefaultTracer is a global tracer that is enabled by default. All of the
// packages top level NewSpan functions use the default tracer.
//
//	span := tracer.NewRootSpan("sql.query", "user-db", "select * from foo where id = ?")
//	defer span.Finish()
//
var DefaultTracer = NewTracer()

// NewRootSpan creates a span with no parent. Its ids will be randomly
// assigned.
func NewRootSpan(name, service, resource string) *Span {
	return DefaultTracer.NewRootSpan(name, service, resource)
}

// NewChildSpan creates a span that is a child of parent. It will inherit the
// parent's service and resource.
func NewChildSpan(name string, parent *Span) *Span {
	return DefaultTracer.NewChildSpan(name, parent)
}

// NewChildSpanFromContext will create a child span of the span contained in
// the given context. If the context contains no span, a span with
// no service or resource will be returned.
func NewChildSpanFromContext(name string, ctx context.Context) *Span {
	return DefaultTracer.NewChildSpanFromContext(name, ctx)
}

// NewChildSpanWithContext will create and return a child span of the span contained in the given
// context, as well as a copy of the parent context containing the created
// child span. If the context contains no span, an empty root span will be returned.
// If nil is passed in for the context, a context will be created.
func NewChildSpanWithContext(name string, ctx context.Context) (*Span, context.Context) {
	return DefaultTracer.NewChildSpanWithContext(name, ctx)
}

// Enable will enable the default tracer.
func Enable() {
	DefaultTracer.SetEnabled(true)
}

// Disable will disable the default tracer.
func Disable() {
	DefaultTracer.SetEnabled(false)
}
