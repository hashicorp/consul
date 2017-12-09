package lib

import (
	"time"

	"github.com/hashicorp/serf/serf"
)

// SerfDefaultConfig returns a Consul-flavored Serf default configuration,
// suitable as a basis for a LAN, WAN, segment, or area.
func SerfDefaultConfig() *serf.Config {
	base := serf.DefaultConfig()

	// This gives leaves some time to propagate through the cluster before
	// we shut down. The value was chosen to be reasonably short, but to
	// allow a leave to get to over 99.99% of the cluster with 100k nodes
	// (using https://www.serf.io/docs/internals/simulator.html).
	base.LeavePropagateDelay = 3 * time.Second

	// This effectively disables the annoying queue depth warnings.
	base.QueueDepthWarning = 1000000

	// This enables dynamic sizing of the message queue depth based on the
	// cluster size.
	base.MinQueueDepth = 4096

	return base
}
