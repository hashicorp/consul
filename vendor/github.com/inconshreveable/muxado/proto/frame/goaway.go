package frame

import "io"

const (
	goAwayBodySize  = 8
	goAwayFrameSize = headerSize + goAwayBodySize
)

// Instruct the remote side not to initiate new streams
type RGoAway struct {
	Header
	body  [goAwayBodySize]byte
	debug []byte
}

func (f *RGoAway) LastStreamId() StreamId {
	return StreamId(order.Uint32(f.body[0:]) & streamMask)
}

func (f *RGoAway) ErrorCode() ErrorCode {
	return ErrorCode(order.Uint32(f.body[4:]))
}

func (f *RGoAway) Debug() []byte {
	return f.debug
}

func (f *RGoAway) readFrom(d deserializer) (err error) {
	if _, err = io.ReadFull(d, f.body[:]); err != nil {
		return
	}

	f.debug = make([]byte, f.Length()-goAwayBodySize)
	if _, err = io.ReadFull(d, f.debug); err != nil {
		return
	}

	return
}

type WGoAway struct {
	Header
	data  [goAwayFrameSize]byte
	debug []byte
}

func (f *WGoAway) writeTo(s serializer) (err error) {
	if _, err = s.Write(f.data[:]); err != nil {
		return
	}

	if _, err = s.Write(f.debug); err != nil {
		return
	}

	return
}

func (f *WGoAway) Set(lastStreamId StreamId, errorCode ErrorCode, debug []byte) (err error) {
	if f.Header.SetAll(TypeGoAway, len(debug)+goAwayFrameSize, 0, 0); err != nil {
		return
	}

	if lastStreamId > streamMask {
		err = protoError("Related stream id %d is out of range", lastStreamId)
		return
	}

	order.PutUint32(f.data[headerSize:], uint32(lastStreamId))
	order.PutUint32(f.data[headerSize+4:], uint32(errorCode))
	return
}

func NewWGoAway() (f *WGoAway) {
	f = new(WGoAway)
	f.Header = Header(f.data[:headerSize])
	return
}
