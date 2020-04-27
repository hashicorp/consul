package router

import (
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
)

// routerFn selects one of the router operations to map to incoming Serf events.
type routerFn func(types.AreaID, *metadata.Server) error

// handleMemberEvents attempts to apply the given Serf member event to the given
// router function.
func handleMemberEvent(logger hclog.Logger, fn routerFn, areaID types.AreaID, e serf.Event) {
	me, ok := e.(serf.MemberEvent)
	if !ok {
		logger.Error("Bad event type", "event", e)
		return
	}

	for _, m := range me.Members {
		ok, parts := metadata.IsConsulServer(m)
		if !ok {
			logger.Warn("Non-server in server-only area",
				"non_server", m.Name,
				"area", areaID,
			)
			continue
		}

		if err := fn(areaID, parts); err != nil {
			logger.Error("Failed to process event for server in area",
				"event", me.Type.String(),
				"server", m.Name,
				"area", areaID,
				"error", err,
			)
			continue
		}

		logger.Info("Handled event for server in area",
			"event", me.Type.String(),
			"server", m.Name,
			"area", areaID,
		)
	}
}

// HandleSerfEvents is a long-running goroutine that pushes incoming events from
// a Serf manager's channel into the given router. This will return when the
// shutdown channel is closed.
func HandleSerfEvents(logger hclog.Logger, router *Router, areaID types.AreaID, shutdownCh <-chan struct{}, eventCh <-chan serf.Event) {
	for {
		select {
		case <-shutdownCh:
			return

		case e := <-eventCh:
			switch e.EventType() {
			case serf.EventMemberJoin:
				handleMemberEvent(logger, router.AddServer, areaID, e)

			case serf.EventMemberLeave, serf.EventMemberReap:
				handleMemberEvent(logger, router.RemoveServer, areaID, e)

			case serf.EventMemberFailed:
				handleMemberEvent(logger, router.FailServer, areaID, e)

			case serf.EventMemberUpdate:
				handleMemberEvent(logger, router.AddServer, areaID, e)

			// All of these event types are ignored.
			case serf.EventUser:
			case serf.EventQuery:

			default:
				logger.Warn("Unhandled Serf Event", "event", e)
			}
		}
	}
}
