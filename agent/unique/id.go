package unique

import (
	"math/rand"
	"strconv"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano()) // seed random number generator
}

func ID() string {
	id := strconv.FormatUint(rand.Uint64(), 36)
	for len(id) < 16 {
		id += " "
	}
	return id
}
