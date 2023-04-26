// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/lib"
	libserf "github.com/hashicorp/consul/lib/serf"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/types"
)

// setupSerf is used to setup and initialize a Serf
func (c *Client) setupSerf(conf *serf.Config, ch chan serf.Event, path string) (*serf.Serf, error) {
	conf.Init()

	conf.NodeName = c.config.NodeName
	conf.Tags["role"] = "node"
	conf.Tags["dc"] = c.config.Datacenter
	conf.Tags["segment"] = c.config.Segment
	conf.Tags["id"] = string(c.config.NodeID)
	conf.Tags["vsn"] = fmt.Sprintf("%d", c.config.ProtocolVersion)
	conf.Tags["vsn_min"] = fmt.Sprintf("%d", ProtocolVersionMin)
	conf.Tags["vsn_max"] = fmt.Sprintf("%d", ProtocolVersionMax)
	conf.Tags["build"] = c.config.Build
	if c.config.AdvertiseReconnectTimeout != 0 {
		conf.Tags[libserf.ReconnectTimeoutTag] = c.config.AdvertiseReconnectTimeout.String()
	}

	// We use the Intercept variant here to ensure that serf and memberlist logs
	// can be streamed via the monitor endpoint
	serfLogger := c.logger.
		NamedIntercept(logging.Serf).
		NamedIntercept(logging.LAN).
		StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true})
	memberlistLogger := c.logger.
		NamedIntercept(logging.Memberlist).
		NamedIntercept(logging.LAN).
		StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true})

	conf.MemberlistConfig.Logger = memberlistLogger
	conf.Logger = serfLogger
	conf.EventCh = ch
	conf.ProtocolVersion = protocolVersionMap[c.config.ProtocolVersion]
	conf.RejoinAfterLeave = c.config.RejoinAfterLeave
	conf.Merge = &lanMergeDelegate{
		dc:        c.config.Datacenter,
		nodeID:    c.config.NodeID,
		nodeName:  c.config.NodeName,
		segment:   c.config.Segment,
		server:    false,
		partition: c.config.AgentEnterpriseMeta().PartitionOrDefault(),
	}

	conf.SnapshotPath = filepath.Join(c.config.DataDir, path)
	if err := lib.EnsurePath(conf.SnapshotPath, false); err != nil {
		return nil, err
	}

	addSerfMetricsLabels(conf, false, c.config.Segment, c.config.AgentEnterpriseMeta().PartitionOrDefault(), "")

	addEnterpriseSerfTags(conf.Tags, c.config.AgentEnterpriseMeta())

	conf.ReconnectTimeoutOverride = libserf.NewReconnectOverride(c.logger)

	enterpriseModifyClientSerfConfigLAN(c.config, conf)

	return serf.Create(conf)
}

// lanEventHandler is used to handle events from the lan Serf cluster
func (c *Client) lanEventHandler() {
	var numQueuedEvents int
	for {
		numQueuedEvents = len(c.eventCh)
		if numQueuedEvents > serfEventBacklogWarning {
			c.logger.Warn("number of queued serf events above warning threshold",
				"queued_events", numQueuedEvents,
				"warning_threshold", serfEventBacklogWarning,
			)
		}

		select {
		case e := <-c.eventCh:
			switch e.EventType() {
			case serf.EventMemberJoin:
				c.nodeJoin(e.(serf.MemberEvent))
			case serf.EventMemberLeave, serf.EventMemberFailed, serf.EventMemberReap:
				c.nodeFail(e.(serf.MemberEvent))
			case serf.EventUser:
				c.localEvent(e.(serf.UserEvent))
			case serf.EventMemberUpdate: // Ignore
				c.nodeUpdate(e.(serf.MemberEvent))
			case serf.EventQuery: // Ignore
			default:
				c.logger.Warn("unhandled LAN Serf Event", "event", e)
			}
		case <-c.shutdownCh:
			return
		}
	}
}

// nodeJoin is used to handle join events on the serf cluster
func (c *Client) nodeJoin(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := metadata.IsConsulServer(m)
		if !ok {
			continue
		}
		if parts.Datacenter != c.config.Datacenter {
			c.logger.Warn("server has joined the wrong cluster: wrong datacenter",
				"server", m.Name,
				"datacenter", parts.Datacenter,
			)
			continue
		}
		c.logger.Info("adding server", "server", parts)
		c.router.AddServer(types.AreaLAN, parts)

		// Trigger the callback
		if c.config.ServerUp != nil {
			c.config.ServerUp()
		}
	}
}

// nodeUpdate is used to handle update events on the serf cluster
func (c *Client) nodeUpdate(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := metadata.IsConsulServer(m)
		if !ok {
			continue
		}
		if parts.Datacenter != c.config.Datacenter {
			c.logger.Warn("server has joined the wrong cluster: wrong datacenter",
				"server", m.Name,
				"datacenter", parts.Datacenter,
			)
			continue
		}
		c.logger.Info("updating server", "server", parts.String())
		c.router.AddServer(types.AreaLAN, parts)
	}
}

// nodeFail is used to handle fail events on the serf cluster
func (c *Client) nodeFail(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := metadata.IsConsulServer(m)
		if !ok {
			continue
		}
		c.logger.Info("removing server", "server", parts.String())
		c.router.RemoveServer(types.AreaLAN, parts)
	}
}

// localEvent is called when we receive an event on the local Serf
func (c *Client) localEvent(event serf.UserEvent) {
	// Handle only consul events
	if !strings.HasPrefix(event.Name, "consul:") {
		return
	}

	switch name := event.Name; {
	case name == newLeaderEvent:
		c.logger.Info("New leader elected", "payload", string(event.Payload))

		// Trigger the callback
		if c.config.ServerUp != nil {
			c.config.ServerUp()
		}
	case isUserEvent(name):
		event.Name = rawUserEventName(name)
		c.logger.Debug("user event", "name", event.Name)

		// Trigger the callback
		if c.config.UserEventHandler != nil {
			c.config.UserEventHandler(event)
		}
	default:
		if !c.handleEnterpriseUserEvents(event) {
			c.logger.Warn("Unhandled local event", "event", event)
		}
	}
}
