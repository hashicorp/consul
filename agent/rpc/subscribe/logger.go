package subscribe

import (
	"sync"
	"time"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// streamID is used in logs as a unique identifier for a subscription. The value
// is created lazily on the first call to String() so that we do not call it
// if trace logging is disabled.
// If a random UUID can not be created, defaults to the current time formatted
// as RFC3339Nano.
//
// TODO(banks) it might be nice one day to replace this with OpenTracing ID
// if one is set etc. but probably pointless until we support that properly
// in other places so it's actually propagated properly. For now this just
// makes lifetime of a stream more traceable in our regular server logs for
// debugging/dev.
type streamID struct {
	once sync.Once
	id   string
}

func (s *streamID) String() string {
	s.once.Do(func() {
		var err error
		s.id, err = uuid.GenerateUUID()
		if err != nil {
			s.id = time.Now().Format(time.RFC3339Nano)
		}
	})
	return s.id
}

func (h *Server) newLoggerForRequest(req *pbsubscribe.SubscribeRequest) Logger {
	return h.Logger.With(
		"topic", req.Topic.String(),
		"dc", req.Datacenter,
		"key", req.Key,
		"index", req.Index,
		"stream_id", &streamID{})
}

type eventLogger struct {
	logger       Logger
	snapshotDone bool
	count        uint64
}

func (l *eventLogger) Trace(e stream.Event) {
	switch {
	case e.IsEndOfSnapshot():
		l.snapshotDone = true
		l.logger.Trace("snapshot complete", "index", e.Index, "sent", l.count)
	case e.IsNewSnapshotToFollow():
		l.logger.Trace("starting new snapshot", "sent", l.count)
		return
	case l.snapshotDone:
		l.logger.Trace("sending events", "index", e.Index, "sent", l.count, "batch_size", e.Len())
	}

	l.count += uint64(e.Len())
}
