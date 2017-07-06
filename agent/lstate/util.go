package lstate

import (
	"math"
	"time"
)

const (
	// This scale factor means we will add a minute after we cross 128 nodes,
	// another at 256, another at 512, etc. By 8192 nodes, we will scale up
	// by a factor of 8.
	//
	// If you update this, you may need to adjust the tuning of
	// CoordinateUpdatePeriod and CoordinateUpdateMaxBatchSize.
	aeScaleThreshold = 128
)

// aeScale is used to scale the time interval at which anti-entropy updates take
// place. It is used to prevent saturation as the cluster size grows.
func aeScale(interval time.Duration, n int) time.Duration {
	// Don't scale until we cross the threshold
	if n <= aeScaleThreshold {
		return interval
	}

	multiplier := math.Ceil(math.Log2(float64(n))-math.Log2(aeScaleThreshold)) + 1.0
	return time.Duration(multiplier) * interval
}
