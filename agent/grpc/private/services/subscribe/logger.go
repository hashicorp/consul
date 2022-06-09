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

func newLoggerForRequest(l Logger, req *pbsubscribe.SubscribeRequest) Logger {
	return l.With(
		"topic", req.Topic.String(),
		"dc", req.Datacenter,
		"peer", req.PeerName,
		"key", req.Key,
		"namespace", req.Namespace,
		"partition", req.Partition,
		"request_index", req.Index,
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
		return
	case e.IsNewSnapshotToFollow():
		l.logger.Trace("starting new snapshot", "sent", l.count)
		return
	}

	size := 1
	if l, ok := e.Payload.(length); ok {
		size = l.Len()
	}
	l.logger.Trace("sending events", "index", e.Index, "sent", l.count, "batch_size", size)
	l.count += uint64(size)
}

type length interface {
	Len() int
}
