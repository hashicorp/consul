package muxado

import (
	"github.com/inconshreveable/muxado/proto/frame"
	"net"
	"time"
)

type StreamId frame.StreamId
type StreamPriority frame.StreamPriority
type StreamType frame.StreamType
type ErrorCode frame.ErrorCode

// Stream is a full duplex stream-oriented connection that is multiplexed over a Session.
// Stream implement the net.Conn inteface.
type Stream interface {
	// Write writes the bytes in the given buffer to the stream
	Write([]byte) (int, error)

	// Read reads the next bytes on the stream into the given buffer
	Read([]byte) (int, error)

	// Close closes the stream. It attempts to behave as Close does for a TCP conn in that it
	// half-closes the stream for sending, and it will send an RST if any more data is received
	// from the remote side.
	Close() error

	// SetDeadline sets a time after which future Read and Write operations will fail.
	SetDeadline(time.Time) error

	// SetReadDeadline sets a time after which future Read operations will fail.
	SetReadDeadline(time.Time) error

	// SetWriteDeadline sets a time after which future Write operations will fail.
	SetWriteDeadline(time.Time) error

	// HalfClose sends a data frame with a fin flag set to half-close the stream from the local side.
	HalfClose([]byte) (int, error)

	// Id returns the stream's id.
	Id() StreamId

	// StreamType returns the stream's type
	StreamType() StreamType

	// Session returns the session object this stream is running on.
	Session() Session

	// RemoteAddr returns the session transport's remote address.
	RemoteAddr() net.Addr

	// LocalAddr returns the session transport's local address.
	LocalAddr() net.Addr
}

// Session multiplexes many Streams over a single underlying stream transport.
// Both sides of a muxado session can open new Streams. Sessions can also accept
// new streams from the remote side.
//
// A muxado Session implements the net.Listener interface, returning new Streams from the remote side.
type Session interface {

	// Open initiates a new stream on the session. It is equivalent to OpenStream(0, 0, false)
	Open() (Stream, error)

	// OpenStream initiates a new stream on the session. A caller can specify a stream's priority and an opaque stream type.
	// Setting fin to true will cause the stream to be half-closed from the local side immediately upon creation.
	OpenStream(priority StreamPriority, streamType StreamType, fin bool) (Stream, error)

	// Accept returns the next stream initiated by the remote side
	Accept() (Stream, error)

	// Kill closes the underlying transport stream immediately.
	//
	// You SHOULD always perfer to call Close() instead so that the connection
	// closes cleanly by sending a GoAway frame.
	Kill() error

	// Close instructs the session to close cleanly, sending a GoAway frame if one hasn't already been sent.
	//
	// This implementation does not "linger". Pending writes on streams may fail.
	//
	// You MAY call Close() more than once. Each time after
	// the first, Close() will return an error.
	Close() error

	// GoAway instructs the other side of the connection to stop
	// initiating new streams by sending a GoAway frame. Most clients
	// will just call Close(), but you may want explicit control of this
	// in order to facilitate clean shutdowns.
	//
	// You MAY call GoAway() more than once. Each time after the first,
	// GoAway() will return an error.
	GoAway(ErrorCode, []byte) error

	// LocalAddr returns the local address of the transport stream over which the session is running.
	LocalAddr() net.Addr

	// RemoteAddr returns the address of the remote side of the transport stream over which the session is running.
	RemoteAddr() net.Addr

	// Wait blocks until the session has shutdown and returns the error code for session termination. It also
	// returns the error that caused the session to terminate as well as any debug information sent in the GoAway
	// frame by the remote side.
	Wait() (code ErrorCode, err error, debug []byte)

	// NetListener returns an adaptor object which allows this Session to be used as a net.Listener. The returned
	// net.Listener returns new streams initiated by the remote side as net.Conn's when calling Accept().
	NetListener() net.Listener

	// NetDial is a function that implements the same API as net.Dial and can be used in place of it. Users should keep
	// in mind that it is the same as a call to Open(). It ignores both arguments passed to it, always initiate a new stream
	// to the remote side.
	NetDial(_, _ string) (net.Conn, error)
}
