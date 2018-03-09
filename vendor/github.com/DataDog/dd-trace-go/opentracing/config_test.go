package opentracing

import (
	"testing"

	ot "github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
)

func TestConfigurationDefaults(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	assert.Equal(true, config.Enabled)
	assert.Equal(false, config.Debug)
	assert.Equal(float64(1), config.SampleRate)
	assert.Equal("opentracing.test", config.ServiceName)
	assert.Equal("localhost", config.AgentHostname)
	assert.Equal("8126", config.AgentPort)
}

func TestConfiguration(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	config.SampleRate = 0
	config.ServiceName = "api-intake"
	config.AgentHostname = "ddagent.consul.local"
	config.AgentPort = "58126"
	tracer, closer, err := NewTracer(config)
	assert.NotNil(tracer)
	assert.NotNil(closer)
	assert.Nil(err)
	assert.Equal("api-intake", tracer.(*Tracer).config.ServiceName)
}

func TestTracerServiceName(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	config.ServiceName = ""
	tracer, closer, err := NewTracer(config)
	assert.Nil(tracer)
	assert.Nil(closer)
	assert.NotNil(err)
	assert.Equal("A Datadog Tracer requires a valid `ServiceName` set", err.Error())
}

func TestDisabledTracer(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	config.Enabled = false
	tracer, closer, err := NewTracer(config)
	assert.IsType(&ot.NoopTracer{}, tracer)
	assert.IsType(&noopCloser{}, closer)
	assert.Nil(err)
}
