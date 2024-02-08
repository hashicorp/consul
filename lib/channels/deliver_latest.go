// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package channels

import "fmt"

// DeliverLatest will drain the channel discarding any messages if there are any and sends the current message.
func DeliverLatest[T any](val T, ch chan T) error {
	// Send if chan is empty
	select {
	case ch <- val:
		return nil
	default:
	}

	// If it falls through to here, the channel is not empty.
	// Drain the channel.
	done := false
	for !done {
		select {
		case <-ch:
			continue
		default:
			done = true
		}
	}

	// Attempt to send again.  If it is not empty, throw an error
	select {
	case ch <- val:
		return nil
	default:
		return fmt.Errorf("failed to deliver latest event: chan full again after draining")
	}
}
