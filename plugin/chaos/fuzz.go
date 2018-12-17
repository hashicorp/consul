// +build fuzz

package chaos

import (
	"github.com/coredns/coredns/plugin/pkg/fuzz"
)

// Fuzz fuzzes cache.
func Fuzz(data []byte) int {
	c := Chaos{}
	return fuzz.Do(c, data)
}
