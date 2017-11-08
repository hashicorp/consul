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
		{123, 456, 1.0}:    true,
		{123, 456, 999}:    true,
		{123, 456, 0.0}:    false,
		{123, 456, -42}:    false,
		{1229998, 0, 0.01}: false,
		{1229999, 0, 0.01}: false,
		{1230000, 0, 0.01}: true,
		{1230001, 0, 0.01}: true,
		{1230098, 0, 0.01}: true,
		{1230099, 0, 0.01}: true,
		{1230100, 0, 0.01}: false,
		{1230101, 0, 0.01}: false,
		{1, 9999999, 0.01}: false,
		{999, 0, 0.99}:     true,
		{9999, 0, 0.99}:    false,
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
