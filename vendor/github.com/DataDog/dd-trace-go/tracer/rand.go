package tracer

import (
	cryptorand "crypto/rand"
	"log"
	"math"
	"math/big"
	"math/rand"
	"sync"
	"time"
)

// randGen is the global thread safe random number generator
var randGen *rand.Rand

type randSource struct {
	source rand.Source
	sync.Mutex
}

func newRandSource() *randSource {
	var seed int64

	max := big.NewInt(math.MaxInt64)
	n, err := cryptorand.Int(cryptorand.Reader, max)
	if err == nil {
		seed = n.Int64()
	} else {
		log.Printf("%scannot generate random seed: %v; using current time\n", errorPrefix, err)
		seed = time.Now().UnixNano()
	}

	source := rand.NewSource(seed)

	return &randSource{source: source}
}

func (rs *randSource) Int63() int64 {
	rs.Lock()
	n := rs.source.Int63()
	rs.Unlock()

	return n
}

func (rs *randSource) Seed(seed int64) {
	rs.Lock()
	rs.Seed(seed)
	rs.Unlock()
}
