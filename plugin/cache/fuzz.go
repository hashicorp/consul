// +build fuzz

package cache

import (
	"time"

	"github.com/coredns/coredns/plugin/pkg/fuzz"
)

// Fuzz fuzzes cache.
func Fuzz(data []byte) int {
	c := &Cache{pcap: defaultCap, ncap: defaultCap, pttl: maxTTL, nttl: maxNTTL, prefetch: 0, duration: 1 * time.Minute}

	return fuzz.Do(c, data)
}
