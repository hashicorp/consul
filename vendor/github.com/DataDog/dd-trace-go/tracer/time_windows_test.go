package tracer

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func BenchmarkNormalTimeNow(b *testing.B) {
	for n := 0; n < b.N; n++ {
		lowPrecisionNow()
	}
}

func BenchmarkHighPrecisionTime(b *testing.B) {
	for n := 0; n < b.N; n++ {
		highPrecisionNow()
	}
}

func TestHighPrecisionTimerIsMoreAccurate(t *testing.T) {
	startLow := lowPrecisionNow()
	startHigh := highPrecisionNow()
	stopHigh := highPrecisionNow()
	for stopHigh == startHigh {
		stopHigh = highPrecisionNow()
	}
	stopLow := lowPrecisionNow()
	assert.Equal(t, int64(0), stopLow-startLow)
}
