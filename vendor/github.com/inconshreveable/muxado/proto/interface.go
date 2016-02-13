package proto

import (
	"github.com/inconshreveable/muxado/proto/frame"
	"net"
	"time"
)

type IStream interface {
	Write([]byte) (int, error)
	Read([]byte) (int, error)
	Close() error
	SetDeadline(time.Time) error
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	HalfClose([]byte) (int, error)
	Id() frame.StreamId
	StreamType() frame.StreamType
	Session() ISession
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
}

type ISession interface {
	Open() (IStream, error)
	OpenStream(frame.StreamPriority, frame.StreamType, bool) (IStream, error)
	Accept() (IStream, error)
	Kill() error
	GoAway(frame.ErrorCode, []byte) error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close() error
	Wait() (frame.ErrorCode, error, []byte)
	NetListener() net.Listener
	NetDial(_, _ string) (net.Conn, error)
}
