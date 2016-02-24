package ext

// XXX: There's no logging around heartbeats - how can we do this in a way that is useful
// as a library?
//
// XXX: When we close the session because of a lost heartbeat or because of an error in the
// heartbeating, there is no way to tell that; a Session will just appear to stop working

import (
	"encoding/binary"
	proto "github.com/inconshreveable/muxado/proto"
	"github.com/inconshreveable/muxado/proto/frame"
	"io"
	"time"
)

const (
	defaultHeartbeatInterval  = 3 * time.Second
	defaultHeartbeatTolerance = 10 * time.Second
)

type Heartbeat struct {
	sess   proto.ISession
	accept proto.ExtAccept

	mark      chan int
	interval  time.Duration
	tolerance time.Duration
}

func NewDefaultHeartbeat() *Heartbeat {
	return NewHeartbeat(defaultHeartbeatInterval, defaultHeartbeatTolerance)
}

func NewHeartbeat(interval, tolerance time.Duration) *Heartbeat {
	return &Heartbeat{
		mark:      make(chan int),
		interval:  interval,
		tolerance: tolerance,
	}
}

func (h *Heartbeat) Start(sess proto.ISession, accept proto.ExtAccept) frame.StreamType {
	h.sess = sess
	h.accept = accept
	go h.respond()
	go h.request()
	go h.check()

	return heartbeatExtensionType
}

func (h *Heartbeat) check() {
	t := time.NewTimer(h.interval + h.tolerance)

	for {
		select {
		case <-t.C:
			// time out waiting for a response!
			h.sess.Close()
			return

		case <-h.mark:
			t.Reset(h.interval + h.tolerance)
		}
	}
}

func (h *Heartbeat) respond() {
	// close the session on any errors
	defer h.sess.Close()

	stream, err := h.accept()
	if err != nil {
		return
	}

	// read the next heartbeat id and respond
	buf := make([]byte, 4)
	for {
		_, err := io.ReadFull(stream, buf)
		if err != nil {
			return
		}

		_, err = stream.Write(buf)
		if err != nil {
			return
		}
	}
}

func (h *Heartbeat) request() {
	// close the session on any errors
	defer h.sess.Close()

	// request highest possible priority for heartbeats
	priority := frame.StreamPriority(0x7FFFFFFF)
	stream, err := h.sess.OpenStream(priority, heartbeatExtensionType, false)
	if err != nil {
		return
	}

	// send heartbeats and then check that we got them back
	var id uint32
	for {
		time.Sleep(h.interval)

		if err := binary.Write(stream, binary.BigEndian, id); err != nil {
			return
		}

		var respId uint32
		if err := binary.Read(stream, binary.BigEndian, &respId); err != nil {
			return
		}

		if id != respId {
			return
		}

		// record the time
		h.mark <- 1
	}
}
