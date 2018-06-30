/*
 *
 * Copyright 2018 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package conn

import (
	"errors"

	"google.golang.org/grpc/credentials/alts/core"
)

const counterLen = 12

var (
	errInvalidCounter = errors.New("invalid counter")
)

// counter is a 96-bit, little-endian counter.
type counter struct {
	value       [counterLen]byte
	invalid     bool
	overflowLen int
}

// newOutCounter returns an outgoing counter initialized to the starting sequence
// number for the client/server side of a connection.
func newOutCounter(s core.Side, overflowLen int) (c counter) {
	c.overflowLen = overflowLen
	if s == core.ServerSide {
		// Server counters in ALTS record have the little-endian high bit
		// set.
		c.value[counterLen-1] = 0x80
	}
	return
}

// newInCounter returns an incoming counter initialized to the starting sequence
// number for the client/server side of a connection. This is used in ALTS record
// to check that incoming counters are as expected, since ALTS record guarantees
// that messages are unwrapped in the same order that the peer wrapped them.
func newInCounter(s core.Side, overflowLen int) (c counter) {
	c.overflowLen = overflowLen
	if s == core.ClientSide {
		// Server counters in ALTS record have the little-endian high bit
		// set.
		c.value[counterLen-1] = 0x80
	}
	return
}

// counterFromValue creates a new counter given an initial value.
func counterFromValue(value []byte, overflowLen int) (c counter) {
	c.overflowLen = overflowLen
	copy(c.value[:], value)
	return
}

// Value returns the current value of the counter as a byte slice.
func (c *counter) Value() ([]byte, error) {
	if c.invalid {
		return nil, errInvalidCounter
	}
	return c.value[:], nil
}

// Inc increments the counter and checks for overflow.
func (c *counter) Inc() {
	// If the counter is already invalid, there is not need to increase it.
	if c.invalid {
		return
	}
	i := 0
	for ; i < c.overflowLen; i++ {
		c.value[i]++
		if c.value[i] != 0 {
			break
		}
	}
	if i == c.overflowLen {
		c.invalid = true
	}
}

// counterSide returns the connection side (client/server) a sequence counter is
// associated with.
func counterSide(c []byte) core.Side {
	if c[counterLen-1]&0x80 == 0x80 {
		return core.ServerSide
	}
	return core.ClientSide
}
