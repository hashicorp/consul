package frame

import (
	"encoding/binary"
	"io"
)

var (
	order = binary.BigEndian
)

// BasicTransport can serialize/deserialize frames on an underlying
// net.Conn to implement the muxado protocol.
type BasicTransport struct {
	io.ReadWriteCloser
	Header
	RStreamSyn
	RStreamRst
	RStreamData
	RStreamWndInc
	RGoAway
}

// WriteFrame writes the given frame to the underlying transport
func (t *BasicTransport) WriteFrame(frame WFrame) (err error) {
	// each frame knows how to write iteself to the framer
	err = frame.writeTo(t)
	return
}

// ReadFrame reads the next frame from the underlying transport
func (t *BasicTransport) ReadFrame() (f RFrame, err error) {
	// read the header
	if _, err = io.ReadFull(t, []byte(t.Header)); err != nil {
		return nil, err
	}

	switch t.Header.Type() {
	case TypeStreamSyn:
		frame := &t.RStreamSyn
		frame.Header = t.Header
		err = frame.readFrom(t)
		return frame, err

	case TypeStreamRst:
		frame := &t.RStreamRst
		frame.Header = t.Header
		err = frame.readFrom(t)
		return frame, err

	case TypeStreamData:
		frame := &t.RStreamData
		frame.Header = t.Header
		err = frame.readFrom(t)
		return frame, err

	case TypeStreamWndInc:
		frame := &t.RStreamWndInc
		frame.Header = t.Header
		err = frame.readFrom(t)
		return frame, err

	case TypeGoAway:
		frame := &t.RGoAway
		frame.Header = t.Header
		err = frame.readFrom(t)
		return frame, err

	default:
		return nil, protoError("Illegal frame type: %d", t.Header.Type())
	}

	return
}

func NewBasicTransport(rwc io.ReadWriteCloser) *BasicTransport {
	trans := &BasicTransport{ReadWriteCloser: rwc, Header: make([]byte, headerSize)}
	return trans
}
