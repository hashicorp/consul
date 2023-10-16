package serf

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
)

const (
	ReconnectTimeoutTag = "rc_tm"
)

// DefaultConfig returns a Consul-flavored Serf default configuration,
// suitable as a basis for a LAN, WAN, segment, or area.
func DefaultConfig() *serf.Config {
	base := serf.DefaultConfig()

	// This effectively disables the annoying queue depth warnings.
	base.QueueDepthWarning = 1000000

	// This enables dynamic sizing of the message queue depth based on the
	// cluster size.
	base.MinQueueDepth = 4096

	// This gives leaves some time to propagate through the cluster before
	// we shut down. The value was chosen to be reasonably short, but to
	// allow a leave to get to over 99.99% of the cluster with 100k nodes
	// (using https://www.serf.io/docs/internals/simulator.html).
	base.LeavePropagateDelay = 3 * time.Second

	return base
}

func GetTags(serf *serf.Serf) map[string]string {
	tags := make(map[string]string)
	for tag, value := range serf.LocalMember().Tags {
		tags[tag] = value
	}

	return tags
}

func UpdateTag(serf *serf.Serf, tag, value string) {
	tags := GetTags(serf)
	tags[tag] = value

	serf.SetTags(tags)
}

type ReconnectOverride struct {
	logger hclog.Logger
}

func NewReconnectOverride(logger hclog.Logger) *ReconnectOverride {
	if logger == nil {
		logger = hclog.Default()
	}

	return &ReconnectOverride{
		logger: logger,
	}
}

func (r *ReconnectOverride) ReconnectTimeout(m *serf.Member, timeout time.Duration) time.Duration {
	val, ok := m.Tags[ReconnectTimeoutTag]
	if !ok {
		return timeout
	}
	newTimeout, err := time.ParseDuration(val)
	if err != nil {
		r.logger.Warn("Member is advertising a malformed reconnect timeout", "member", m.Name, "rc_tm", val)
		return timeout
	}

	// ignore a timeout of 0 as that indicates the default should be used
	if newTimeout == 0 {
		return timeout
	}

	return newTimeout
}
