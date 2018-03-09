package tracer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPushTrace(t *testing.T) {
	assert := assert.New(t)

	channels := newTracerChans()

	trace := []*Span{
		&Span{
			Name:     "pylons.request",
			Service:  "pylons",
			Resource: "/",
		},
		&Span{
			Name:     "pylons.request",
			Service:  "pylons",
			Resource: "/foo",
		},
	}
	channels.pushTrace(trace)

	assert.Len(channels.trace, 1, "there should be data in channel")
	assert.Len(channels.traceFlush, 0, "no flush requested yet")

	pushed := <-channels.trace
	assert.Equal(trace, pushed)

	many := traceChanLen/2 + 1
	for i := 0; i < many; i++ {
		channels.pushTrace(make([]*Span, i))
	}
	assert.Len(channels.trace, many, "all traces should be in the channel, not yet blocking")
	assert.Len(channels.traceFlush, 1, "a trace flush should have been requested")

	for i := 0; i < cap(channels.trace); i++ {
		channels.pushTrace(make([]*Span, i))
	}
	assert.Len(channels.trace, traceChanLen, "buffer should be full")
	assert.NotEqual(0, len(channels.err), "there should be an error logged")
	err := <-channels.err
	assert.Equal(&errorTraceChanFull{Len: traceChanLen}, err)
}

func TestPushService(t *testing.T) {
	assert := assert.New(t)

	channels := newTracerChans()

	service := Service{
		Name:    "redis-master",
		App:     "redis",
		AppType: "db",
	}
	channels.pushService(service)

	assert.Len(channels.service, 1, "there should be data in channel")
	assert.Len(channels.serviceFlush, 0, "no flush requested yet")

	pushed := <-channels.service
	assert.Equal(service, pushed)

	many := serviceChanLen/2 + 1
	for i := 0; i < many; i++ {
		channels.pushService(Service{
			Name:    fmt.Sprintf("service%d", i),
			App:     "custom",
			AppType: "web",
		})
	}
	assert.Len(channels.service, many, "all services should be in the channel, not yet blocking")
	assert.Len(channels.serviceFlush, 1, "a service flush should have been requested")

	for i := 0; i < cap(channels.service); i++ {
		channels.pushService(Service{
			Name:    fmt.Sprintf("service%d", i),
			App:     "custom",
			AppType: "web",
		})
	}
	assert.Len(channels.service, serviceChanLen, "buffer should be full")
	assert.NotEqual(0, len(channels.err), "there should be an error logged")
	err := <-channels.err
	assert.Equal(&errorServiceChanFull{Len: serviceChanLen}, err)
}

func TestPushErr(t *testing.T) {
	assert := assert.New(t)

	channels := newTracerChans()

	err := fmt.Errorf("ooops")
	channels.pushErr(err)

	assert.Len(channels.err, 1, "there should be data in channel")
	assert.Len(channels.errFlush, 0, "no flush requested yet")

	pushed := <-channels.err
	assert.Equal(err, pushed)

	many := errChanLen/2 + 1
	for i := 0; i < many; i++ {
		channels.pushErr(fmt.Errorf("err %d", i))
	}
	assert.Len(channels.err, many, "all errs should be in the channel, not yet blocking")
	assert.Len(channels.errFlush, 1, "a err flush should have been requested")
	for i := 0; i < cap(channels.err); i++ {
		channels.pushErr(fmt.Errorf("err %d", i))
	}
	// if we reach this, means pushErr is not blocking, which is what we want to double-check
}
