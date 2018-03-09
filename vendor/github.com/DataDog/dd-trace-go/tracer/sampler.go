package tracer

const (
	// sampleRateMetricKey is the metric key holding the applied sample rate. Has to be the same as the Agent.
	sampleRateMetricKey = "_sample_rate"

	// constants used for the Knuth hashing, same constants as the Agent.
	maxTraceID      = ^uint64(0)
	maxTraceIDFloat = float64(maxTraceID)
	samplerHasher   = uint64(1111111111111111111)
)

// sampler is the generic interface of any sampler
type sampler interface {
	Sample(span *Span) // Tells if a trace is sampled and sets `span.Sampled`
}

// allSampler samples all the traces
type allSampler struct{}

func newAllSampler() *allSampler {
	return &allSampler{}
}

// Sample samples a span
func (s *allSampler) Sample(span *Span) {
	// Nothing to do here, since by default a trace is sampled
}

// rateSampler samples from a sample rate
type rateSampler struct {
	SampleRate float64
}

// newRateSampler returns an initialized rateSampler with its sample rate
func newRateSampler(sampleRate float64) *rateSampler {
	return &rateSampler{
		SampleRate: sampleRate,
	}
}

// Sample samples a span
func (s *rateSampler) Sample(span *Span) {
	if s.SampleRate < 1 {
		span.Sampled = sampleByRate(span.TraceID, s.SampleRate)
		span.SetMetric(sampleRateMetricKey, s.SampleRate)
	}
}

// sampleByRate tells if a trace (from its ID) with a given rate should be sampled.
// Its implementation has to be the same as the Trace Agent.
func sampleByRate(traceID uint64, sampleRate float64) bool {
	if sampleRate < 1 {
		return traceID*samplerHasher < uint64(sampleRate*maxTraceIDFloat)
	}
	return true
}
