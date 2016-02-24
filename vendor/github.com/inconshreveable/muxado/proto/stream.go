package proto

import (
	"fmt"
	"github.com/inconshreveable/muxado/proto/buffer"
	"github.com/inconshreveable/muxado/proto/frame"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	zeroTime         time.Time
	resetRemoveDelay = 10 * time.Second
	closeError       = fmt.Errorf("Stream closed")
)

type Stream struct {
	id            frame.StreamId       // stream id (const)
	streamType    frame.StreamType     // related stream id (const)
	session       session              // the parent session (const)
	inBuffer      *buffer.Inbound      // buffer for data coming in from the remote side
	outBuffer     *buffer.Outbound     // manages size of the outbound window
	sentRst       uint32               // == 1 only if we sent a reset to close this connection
	writer        sync.Mutex           // only one writer at a time
	wdata         *frame.WStreamData   // the frame this stream is currently writing
	winc          *frame.WStreamWndInc // window increment currently being written
	readDeadline  time.Time            // deadline for reads (protected by buffer mutex)
	writeDeadline time.Time            // deadline for writes (protected by writer mutex)
}

// private interface for Streams to call Sessions
type session interface {
	ISession
	writeFrame(frame.WFrame, time.Time) error
	die(frame.ErrorCode, error) error
	removeStream(frame.StreamId)
}

////////////////////////////////
// public interface
////////////////////////////////
func NewStream(id frame.StreamId, priority frame.StreamPriority, streamType frame.StreamType, finLocal bool, finRemote bool, windowSize uint32, sess session) stream {
	str := &Stream{
		id:         id,
		inBuffer:   buffer.NewInbound(int(windowSize)),
		outBuffer:  buffer.NewOutbound(int(windowSize)),
		streamType: streamType,
		session:    sess,
		wdata:      frame.NewWStreamData(),
		winc:       frame.NewWStreamWndInc(),
	}

	if finLocal {
		str.inBuffer.SetError(io.EOF)
	}

	if finRemote {
		str.outBuffer.SetError(fmt.Errorf("Stream closed"))
	}

	return str
}

func (s *Stream) Write(buf []byte) (n int, err error) {
	return s.write(buf, false)
}

func (s *Stream) Read(buf []byte) (n int, err error) {
	// read from the buffer
	n, err = s.inBuffer.Read(buf)

	// if we read more than zero, we send a window update
	if n > 0 {
		errWnd := s.sendWindowUpdate(uint32(n))
		if errWnd != nil {
			err = errWnd
			s.die(frame.InternalError, err)
		}
	}

	return
}

// Close closes the stream in a manner that attempts to emulate a net.Conn's Close():
// - It calls HalfClose() with an empty buffer to half-close the stream on the remote side
// - It calls closeWith() so that all future Read/Write operations will fail
// - If the stream receives another STREAM_DATA frame from the remote side, it will send a STREAM_RST with a CANCELED error code
func (s *Stream) Close() error {
	s.HalfClose([]byte{})
	s.closeWith(closeError)
	return nil
}

func (s *Stream) SetDeadline(deadline time.Time) (err error) {
	if err = s.SetReadDeadline(deadline); err != nil {
		return
	}
	if err = s.SetWriteDeadline(deadline); err != nil {
		return
	}
	return
}

func (s *Stream) SetReadDeadline(dl time.Time) error {
	s.inBuffer.SetDeadline(dl)
	return nil
}

func (s *Stream) SetWriteDeadline(dl time.Time) error {
	s.writer.Lock()
	s.writeDeadline = dl
	s.writer.Unlock()
	return nil
}

func (s *Stream) HalfClose(buf []byte) (n int, err error) {
	return s.write(buf, true)
}

func (s *Stream) Id() frame.StreamId {
	return s.id
}

func (s *Stream) StreamType() frame.StreamType {
	return s.streamType
}

func (s *Stream) Session() ISession {
	return s.session
}

func (s *Stream) LocalAddr() net.Addr {
	return s.session.LocalAddr()
}

func (s *Stream) RemoteAddr() net.Addr {
	return s.session.RemoteAddr()
}

/////////////////////////////////////
// session's stream interface
/////////////////////////////////////
func (s *Stream) handleStreamData(f *frame.RStreamData) {
	// skip writing for zero-length frames (typically for sending FIN)
	if f.Length() > 0 {
		// write the data into the buffer
		if _, err := s.inBuffer.ReadFrom(f.Reader()); err != nil {
			if err == buffer.FullError {
				s.resetWith(frame.FlowControlError, fmt.Errorf("Flow control buffer overflowed"))
			} else if err == closeError {
				// We're trying to emulate net.Conn's Close() behavior where we close our side of the connection,
				// and if we get any more frames from the other side, we RST it.
				s.resetWith(frame.Cancel, fmt.Errorf("Stream closed"))
			} else if err == buffer.AlreadyClosed {
				// there was already an error set
				s.resetWith(frame.StreamClosed, err)
			} else {
				// the transport returned some sort of IO error
				s.die(frame.ProtocolError, err)
			}
			return
		}
	}

	if f.Fin() {
		s.inBuffer.SetError(io.EOF)
		s.maybeRemove()
	}

}

func (s *Stream) handleStreamRst(f *frame.RStreamRst) {
	s.closeWith(fmt.Errorf("Stream reset by peer with error %d", f.ErrorCode()))
}

func (s *Stream) handleStreamWndInc(f *frame.RStreamWndInc) {
	s.outBuffer.Increment(int(f.WindowIncrement()))
}

func (s *Stream) closeWith(err error) {
	s.outBuffer.SetError(err)
	s.inBuffer.SetError(err)
	s.session.removeStream(s.id)
}

////////////////////////////////
// internal methods
////////////////////////////////

func (s *Stream) closeWithAndRemoveLater(err error) {
	s.outBuffer.SetError(err)
	s.inBuffer.SetError(err)
	time.AfterFunc(resetRemoveDelay, func() {
		s.session.removeStream(s.id)
	})
}

func (s *Stream) maybeRemove() {
	if buffer.BothClosed(s.inBuffer, s.outBuffer) {
		s.session.removeStream(s.id)
	}
}

func (s *Stream) resetWith(errorCode frame.ErrorCode, resetErr error) {
	// only ever send one reset
	if !atomic.CompareAndSwapUint32(&s.sentRst, 0, 1) {
		return
	}

	// close the stream
	s.closeWithAndRemoveLater(resetErr)

	// make the reset frame
	rst := frame.NewWStreamRst()
	if err := rst.Set(s.id, errorCode); err != nil {
		s.die(frame.InternalError, err)
	}

	// need write lock to make sure no data frames get sent after we send the reset
	s.writer.Lock()

	// send it
	if err := s.session.writeFrame(rst, zeroTime); err != nil {
		s.writer.Unlock()
		s.die(frame.InternalError, err)
	}

	s.writer.Unlock()
}

func (s *Stream) write(buf []byte, fin bool) (n int, err error) {
	// a write call can pass a buffer larger that we can send in a single frame
	// only allow one writer at a time to prevent interleaving frames from concurrent writes
	s.writer.Lock()

	bufSize := len(buf)
	bytesRemaining := bufSize
	for bytesRemaining > 0 || fin {
		// figure out the most we can write in a single frame
		writeReqSize := min(0x3FFF, bytesRemaining)

		// and then reduce that to however much is available in the window
		// this blocks until window is available and may not return all that we asked for
		var writeSize int
		if writeSize, err = s.outBuffer.Decrement(writeReqSize); err != nil {
			s.writer.Unlock()
			return
		}

		// calculate the slice of the buffer we'll write
		start, end := n, n+writeSize

		// only send fin for the last frame
		finBit := fin && end == bufSize

		// make the frame
		if err = s.wdata.Set(s.id, buf[start:end], finBit); err != nil {
			s.writer.Unlock()
			s.die(frame.InternalError, err)
			return
		}

		// write the frame
		if err = s.session.writeFrame(s.wdata, s.writeDeadline); err != nil {
			s.writer.Unlock()
			return
		}

		// update our counts
		n += writeSize
		bytesRemaining -= writeSize

		if finBit {
			s.outBuffer.SetError(fmt.Errorf("Stream closed"))
			s.maybeRemove()

			// handles the empty buffer case with fin case
			fin = false
		}
	}

	s.writer.Unlock()
	return
}

// sendWindowUpdate sends a window increment frame
// with the given increment
func (s *Stream) sendWindowUpdate(inc uint32) (err error) {
	// send a window update
	if err = s.winc.Set(s.id, inc); err != nil {
		return
	}

	// XXX: write this async? We can only write one at
	// a time if we're not allocating new ones from the heap
	if err = s.session.writeFrame(s.winc, zeroTime); err != nil {
		return
	}

	return
}

// die is called when a protocol error occurs and the entire
// session must be destroyed.
func (s *Stream) die(errorCode frame.ErrorCode, err error) {
	s.closeWith(fmt.Errorf("Stream closed on error: %v", err))
	s.session.die(errorCode, err)
}

func min(n1, n2 int) int {
	if n1 > n2 {
		return n2
	} else {
		return n1
	}
}
