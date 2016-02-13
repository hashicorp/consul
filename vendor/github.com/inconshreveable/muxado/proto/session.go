package proto

import (
	"fmt"
	"github.com/inconshreveable/muxado/proto/frame"
	"io"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultWindowSize       = 0x10000 // 64KB
	defaultAcceptQueueDepth = 100
	MinExtensionType        = 0xFFFFFFFF - 0x100 // 512 extensions
)

// private interface for Sessions to call Streams
type stream interface {
	IStream
	handleStreamData(*frame.RStreamData)
	handleStreamWndInc(*frame.RStreamWndInc)
	handleStreamRst(*frame.RStreamRst)
	closeWith(error)
}

// for extensions
type ExtAccept func() (IStream, error)
type Extension interface {
	Start(ISession, ExtAccept) frame.StreamType
}

type deadReason struct {
	errorCode   frame.ErrorCode
	err         error
	remoteDebug []byte
}

// factory function that creates new streams
type streamFactory func(id frame.StreamId, priority frame.StreamPriority, streamType frame.StreamType, finLocal bool, finRemote bool, windowSize uint32, sess session) stream

// checks the parity of a stream id (local vs remote, client vs server)
type parityFn func(frame.StreamId) bool

// state for each half of the session (remote and local)
type halfState struct {
	goneAway int32  // true if that half of the stream has gone away
	lastId   uint32 // last id used/seen from one half of the session
}

// Session implements a simple streaming session manager. It has the following characteristics:
//
// - When closing the Session, it does not linger, all pending write operations will fail immediately.
// - It completely ignores stream priority when processing and writing frames
// - It offers no customization of settings like window size/ping time
type Session struct {
	conn              net.Conn                         // connection the transport is running over
	transport         frame.Transport                  // transport
	streams           StreamMap                        // all active streams
	local             halfState                        // client state
	remote            halfState                        // server state
	syn               *frame.WStreamSyn                // STREAM_SYN frame for opens
	wr                sync.Mutex                       // synchronization when writing frames
	accept            chan stream                      // new streams opened by the remote
	diebit            int32                            // true if we're dying
	remoteDebug       []byte                           // debugging data sent in the remote's GoAway frame
	defaultWindowSize uint32                           // window size when creating new streams
	newStream         streamFactory                    // factory function to make new streams
	dead              chan deadReason                  // dead
	isLocal           parityFn                         // determines if a stream id is local or remote
	exts              map[frame.StreamType]chan stream // map of extension stream type -> accept channel for the extension
}

func NewSession(conn net.Conn, newStream streamFactory, isClient bool, exts []Extension) ISession {
	sess := &Session{
		conn:              conn,
		transport:         frame.NewBasicTransport(conn),
		streams:           NewConcurrentStreamMap(),
		local:             halfState{lastId: 0},
		remote:            halfState{lastId: 0},
		syn:               frame.NewWStreamSyn(),
		diebit:            0,
		defaultWindowSize: defaultWindowSize,
		accept:            make(chan stream, defaultAcceptQueueDepth),
		newStream:         newStream,
		dead:              make(chan deadReason, 1), // don't block die() if there is no Wait call
		exts:              make(map[frame.StreamType]chan stream),
	}

	if isClient {
		sess.isLocal = sess.isClient
		sess.local.lastId += 1
	} else {
		sess.isLocal = sess.isServer
		sess.remote.lastId += 1
	}

	for _, ext := range exts {
		sess.startExtension(ext)
	}

	go sess.reader()

	return sess
}

////////////////////////////////
// public interface
////////////////////////////////

func (s *Session) Open() (IStream, error) {
	return s.OpenStream(0, 0, false)
}

func (s *Session) OpenStream(priority frame.StreamPriority, streamType frame.StreamType, fin bool) (ret IStream, err error) {
	// check if the remote has gone away
	if atomic.LoadInt32(&s.remote.goneAway) == 1 {
		return nil, fmt.Errorf("Failed to create stream, remote has gone away.")
	}

	// this lock prevents the following race:
	// goroutine1       goroutine2
	// - inc stream id
	//                  - inc stream id
	//                  - send streamsyn
	// - send streamsyn
	s.wr.Lock()

	// get the next id we can use
	nextId := frame.StreamId(atomic.AddUint32(&s.local.lastId, 2))

	// make the stream
	str := s.newStream(nextId, priority, streamType, fin, false, s.defaultWindowSize, s)

	// add to to the stream map
	s.streams.Set(nextId, str)

	// write the frame
	if err = s.syn.Set(nextId, priority, streamType, fin); err != nil {
		s.wr.Unlock()
		s.die(frame.InternalError, err)
		return
	}

	if err = s.transport.WriteFrame(s.syn); err != nil {
		s.wr.Unlock()
		s.die(frame.InternalError, err)
		return
	}

	s.wr.Unlock()
	return str, nil
}

func (s *Session) Accept() (str IStream, err error) {
	var ok bool
	if str, ok = <-s.accept; !ok {
		return nil, fmt.Errorf("Session closed")
	}

	return
}

func (s *Session) Kill() error {
	return s.transport.Close()
}

func (s *Session) Close() error {
	return s.die(frame.NoError, fmt.Errorf("Session Close()"))
}

func (s *Session) GoAway(errorCode frame.ErrorCode, debug []byte) (err error) {
	if !atomic.CompareAndSwapInt32(&s.local.goneAway, 0, 1) {
		return fmt.Errorf("Already sent GoAway!")
	}

	s.wr.Lock()
	f := frame.NewWGoAway()
	remoteId := frame.StreamId(atomic.LoadUint32(&s.remote.lastId))
	if err = f.Set(remoteId, errorCode, debug); err != nil {
		s.wr.Unlock()
		s.die(frame.InternalError, err)
		return
	}

	if err = s.transport.WriteFrame(f); err != nil {
		s.wr.Unlock()
		s.die(frame.InternalError, err)
		return
	}

	s.wr.Unlock()
	return
}

func (s *Session) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Session) Wait() (frame.ErrorCode, error, []byte) {
	reason := <-s.dead
	return reason.errorCode, reason.err, reason.remoteDebug
}

////////////////////////////////
// private interface for streams
////////////////////////////////

// removeStream removes a stream from this session's stream registry
//
// It does not error if the stream is not present
func (s *Session) removeStream(id frame.StreamId) {
	s.streams.Delete(id)
	return
}

// writeFrame writes the given frame to the transport and returns the error from the write operation
func (s *Session) writeFrame(f frame.WFrame, dl time.Time) (err error) {
	s.wr.Lock()
	s.conn.SetWriteDeadline(dl)
	err = s.transport.WriteFrame(f)
	s.wr.Unlock()
	return
}

// die closes the session cleanly with the given error and protocol error code
func (s *Session) die(errorCode frame.ErrorCode, err error) error {
	// only one shutdown ever happens
	if !atomic.CompareAndSwapInt32(&s.diebit, 0, 1) {
		return fmt.Errorf("Shutdown already in progress")
	}

	// send a go away frame
	s.GoAway(errorCode, []byte(err.Error()))

	// now we're safe to stop accepting incoming connections
	close(s.accept)

	// we cleaned up as best as possible, close the transport
	s.transport.Close()

	// notify all of the streams that we're closing
	s.streams.Each(func(id frame.StreamId, str stream) {
		str.closeWith(fmt.Errorf("Session closed"))
	})

	s.dead <- deadReason{errorCode, err, s.remoteDebug}

	return nil
}

////////////////////////////////
// internal methods
////////////////////////////////

// reader() reads frames from the underlying transport and handles passes them to handleFrame
func (s *Session) reader() {
	defer s.recoverPanic("reader()")

	// close all of the extension accept channels when we're done
	// we do this here instead of in die() since otherwise it wouldn't
	// be safe to access s.exts
	defer func() {
		for _, extAccept := range s.exts {
			close(extAccept)
		}
	}()

	for {
		f, err := s.transport.ReadFrame()
		if err != nil {
			// if we fail to read a frame, terminate the session
			_, ok := err.(*frame.FramingError)
			if ok {
				s.die(frame.ProtocolError, err)
			} else {
				s.die(frame.InternalError, err)
			}
			return
		}

		s.handleFrame(f)
	}
}

func (s *Session) handleFrame(rf frame.RFrame) {
	switch f := rf.(type) {
	case *frame.RStreamSyn:
		// if we're going away, refuse new streams
		if atomic.LoadInt32(&s.local.goneAway) == 1 {
			rstF := frame.NewWStreamRst()
			rstF.Set(f.StreamId(), frame.RefusedStream)
			go s.writeFrame(rstF, time.Time{})
			return
		}

		if f.StreamId() <= frame.StreamId(atomic.LoadUint32(&s.remote.lastId)) {
			s.die(frame.ProtocolError, fmt.Errorf("Stream id %d is less than last remote id.", f.StreamId()))
			return
		}

		if s.isLocal(f.StreamId()) {
			s.die(frame.ProtocolError, fmt.Errorf("Stream id has wrong parity for remote endpoint: %d", f.StreamId()))
			return
		}

		// update last remote id
		atomic.StoreUint32(&s.remote.lastId, uint32(f.StreamId()))

		// make the new stream
		str := s.newStream(f.StreamId(), f.StreamPriority(), f.StreamType(), false, f.Fin(), s.defaultWindowSize, s)

		// add it to the stream map
		s.streams.Set(f.StreamId(), str)

		// check if this is an extension stream
		if f.StreamType() >= MinExtensionType {
			extAccept, ok := s.exts[f.StreamType()]
			if !ok {
				// Extension type of stream not registered
				fRst := frame.NewWStreamRst()
				if err := fRst.Set(f.StreamId(), frame.StreamClosed); err != nil {
					s.die(frame.InternalError, err)
				}

				s.wr.Lock()
				defer s.wr.Unlock()
				s.transport.WriteFrame(fRst)
			} else {
				extAccept <- str
			}

			return
		}

		// put the new stream on the accept channel
		s.accept <- str

	case *frame.RStreamData:
		if str := s.getStream(f.StreamId()); str != nil {
			str.handleStreamData(f)
		} else {
			// if we get a data frame on a non-existent connection, we still
			// need to read out the frame body so that the stream stays in a
			// good state. read the payload into a throwaway buffer
			discard := make([]byte, f.Length())
			io.ReadFull(f.Reader(), discard)

			// DATA frames on closed connections are just stream-level errors
			fRst := frame.NewWStreamRst()
			if err := fRst.Set(f.StreamId(), frame.StreamClosed); err != nil {
				s.die(frame.InternalError, err)
			}

			s.wr.Lock()
			defer s.wr.Unlock()
			s.transport.WriteFrame(fRst)
			return
		}

	case *frame.RStreamRst:
		// delegate to the stream to handle these frames
		if str := s.getStream(f.StreamId()); str != nil {
			str.handleStreamRst(f)
		}
	case *frame.RStreamWndInc:
		// delegate to the stream to handle these frames
		if str := s.getStream(f.StreamId()); str != nil {
			str.handleStreamWndInc(f)
		}

	case *frame.RGoAway:
		atomic.StoreInt32(&s.remote.goneAway, 1)
		s.remoteDebug = f.Debug()

		lastId := f.LastStreamId()
		s.streams.Each(func(id frame.StreamId, str stream) {
			// close all streams that we opened above the last handled id
			if s.isLocal(str.Id()) && str.Id() > lastId {
				str.closeWith(fmt.Errorf("Remote is going away"))
			}
		})

	default:
		s.die(frame.ProtocolError, fmt.Errorf("Unrecognized frame type: %v", reflect.TypeOf(f)))
		return
	}
}

func (s *Session) recoverPanic(prefix string) {
	if r := recover(); r != nil {
		s.die(frame.InternalError, fmt.Errorf("%s panic: %v", prefix, r))
	}
}

func (s *Session) getStream(id frame.StreamId) (str stream) {
	// decide if this id is in the "idle" state (i.e. greater than any we've seen for that parity)
	var lastId *uint32
	if s.isLocal(id) {
		lastId = &s.local.lastId
	} else {
		lastId = &s.remote.lastId
	}

	if uint32(id) > atomic.LoadUint32(lastId) {
		s.die(frame.ProtocolError, fmt.Errorf("%d is an invalid, unassigned stream id", id))
	}

	// find the stream in the stream map
	var ok bool
	if str, ok = s.streams.Get(id); !ok {
		return nil
	}

	return
}

// check if a stream id is for a client stream. client streams are odd
func (s *Session) isClient(id frame.StreamId) bool {
	return uint32(id)&1 == 1
}

func (s *Session) isServer(id frame.StreamId) bool {
	return !s.isClient(id)
}

//////////////////////////////////////////////
// session extensions
//////////////////////////////////////////////
func (s *Session) startExtension(ext Extension) {
	accept := make(chan stream)
	extAccept := func() (IStream, error) {
		s, ok := <-accept
		if !ok {
			return nil, fmt.Errorf("Failed to accept connection, shutting down")
		}

		return s, nil
	}

	extType := ext.Start(s, extAccept)
	s.exts[extType] = accept
}

//////////////////////////////////////////////
// net adaptors
//////////////////////////////////////////////
func (s *Session) NetDial(_, _ string) (net.Conn, error) {
	str, err := s.Open()
	return net.Conn(str), err
}

func (s *Session) NetListener() net.Listener {
	return &netListenerAdaptor{s}
}

type netListenerAdaptor struct {
	*Session
}

func (a *netListenerAdaptor) Addr() net.Addr {
	return a.LocalAddr()
}

func (a *netListenerAdaptor) Accept() (net.Conn, error) {
	str, err := a.Session.Accept()
	return net.Conn(str), err
}
