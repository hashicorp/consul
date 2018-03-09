package opentracing

import (
	"os"
	"path/filepath"
)

// Configuration struct configures the Datadog tracer. Please use the NewConfiguration
// constructor to begin.
type Configuration struct {
	// Enabled, when false, returns a no-op implementation of the Tracer.
	Enabled bool

	// Debug, when true, writes details to logs.
	Debug bool

	// ServiceName specifies the name of this application.
	ServiceName string

	// SampleRate sets the Tracer sample rate (ext/priority.go).
	SampleRate float64

	// AgentHostname specifies the hostname of the agent where the traces
	// are sent to.
	AgentHostname string

	// AgentPort specifies the port that the agent is listening on.
	AgentPort string

	// GlobalTags holds a set of tags that will be automatically applied to
	// all spans.
	GlobalTags map[string]interface{}

	// TextMapPropagator is an injector used for Context propagation.
	TextMapPropagator Propagator
}

// NewConfiguration creates a `Configuration` object with default values.
func NewConfiguration() *Configuration {
	// default service name is the Go binary name
	binaryName := filepath.Base(os.Args[0])

	// Configuration struct with default values
	return &Configuration{
		Enabled:           true,
		Debug:             false,
		ServiceName:       binaryName,
		SampleRate:        1,
		AgentHostname:     "localhost",
		AgentPort:         "8126",
		GlobalTags:        make(map[string]interface{}),
		TextMapPropagator: NewTextMapPropagator("", "", ""),
	}
}

type noopCloser struct{}

func (c *noopCloser) Close() error { return nil }
