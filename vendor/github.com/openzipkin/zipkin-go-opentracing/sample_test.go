package zipkintracer_test

import (
	"testing"

	zipkin "github.com/openzipkin/zipkin-go-opentracing"
)

func TestBoundarySampler(t *testing.T) {
	type triple struct {
		id   uint64
		salt int64
		rate float64
	}
	for input, want := range map[triple]bool{
		triple{123, 456, 1.0}:    true,
		triple{123, 456, 999}:    true,
		triple{123, 456, 0.0}:    false,
		triple{123, 456, -42}:    false,
		triple{1229998, 0, 0.01}: false,
		triple{1229999, 0, 0.01}: false,
		triple{1230000, 0, 0.01}: true,
		triple{1230001, 0, 0.01}: true,
		triple{1230098, 0, 0.01}: true,
		triple{1230099, 0, 0.01}: true,
		triple{1230100, 0, 0.01}: false,
		triple{1230101, 0, 0.01}: false,
		triple{1, 9999999, 0.01}: false,
		triple{999, 0, 0.99}:     true,
		triple{9999, 0, 0.99}:    false,
	} {
		sampler := zipkin.NewBoundarySampler(input.rate, input.salt)
		if have := sampler(input.id); want != have {
			t.Errorf("%#+v: want %v, have %v", input, want, have)
		}
	}
}

func TestCountingSampler(t *testing.T) {
	for n := 1; n < 100; n++ {
		sampler := zipkin.NewCountingSampler(float64(n) / 100)
		found := 0
		for i := 0; i < 100; i++ {
			if sampler(1) {
				found++
			}
		}
		if found != n {
			t.Errorf("want %d, have %d\n", n, found)
		}
	}
}
