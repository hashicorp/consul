package seed

import (
	crand "crypto/rand"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"sync"
	"time"
)

var (
	once   sync.Once
	secure bool
	seeded bool
)

// Init provides best-effort seeding (which is better than running with Go's
// default seed of 1).  If `/dev/urandom` is available, Init() will seed Go's
// runtime with entropy from `/dev/urandom` and return true because the runtime
// was securely seeded.  If Init() has already initialized the random number or
// it had failed to securely initialize the random number generation, Init()
// will return false.  See MustInit().
func Init() (bool, error) {
	var err error
	once.Do(func() {
		var n *big.Int
		n, err = crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			rand.Seed(time.Now().UTC().UnixNano())
			return
		}
		rand.Seed(n.Int64())
		secure = true
		seeded = true
	})
	return seeded && secure, err
}

// MustInit provides guaranteed seeding.  If `/dev/urandom` is not available,
// MustInit will panic() with an error indicating why reading from
// `/dev/urandom` failed.  See Init()
func MustInit() {
	once.Do(func() {
		n, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			panic(fmt.Sprintf("Unable to seed the random number generator: %v", err))
		}
		rand.Seed(n.Int64())
		secure = true
		seeded = true
	})
}

// Secure returns true if a cryptographically secure seed was used to
// initialize rand.
func Secure() bool {
	return secure
}

// Seeded returns true if Init has seeded the random number generator.
func Seeded() bool {
	return seeded
}
