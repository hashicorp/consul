package frame

import "io"

const (
	rstBodySize  = 4
	rstFrameSize = headerSize + rstBodySize
)

// RsStreamRst is a STREAM_RST frame that is read from a transport
type RStreamRst struct {
	Header
	body [rstBodySize]byte
}

func (f *RStreamRst) readFrom(d deserializer) (err error) {
	if f.Length() != rstBodySize {
		return protoError("STREAM_RST length must be %d, got %d", rstBodySize, f.Length())
	}

	if _, err = io.ReadFull(d, f.body[:]); err != nil {
		return
	}

	return
}

func (f *RStreamRst) ErrorCode() ErrorCode {
	return ErrorCode(order.Uint32(f.body[0:]))
}

// WStreamRst is a STREAM_RST frame that can be written, it terminate a stream ungracefully
type WStreamRst struct {
	Header
	all [rstFrameSize]byte
}

func NewWStreamRst() (f *WStreamRst) {
	f = new(WStreamRst)
	f.Header = Header(f.all[:headerSize])
	return
}

func (f *WStreamRst) writeTo(s serializer) (err error) {
	_, err = s.Write(f.all[:])
	return
}

func (f *WStreamRst) Set(streamId StreamId, errorCode ErrorCode) (err error) {
	if err = f.Header.SetAll(TypeStreamRst, rstBodySize, streamId, 0); err != nil {
		return
	}

	if err = validRstErrorCode(errorCode); err != nil {
		return
	}

	order.PutUint32(f.all[headerSize:], uint32(errorCode))
	return
}

func validRstErrorCode(errorCode ErrorCode) error {
	if errorCode >= NoSuchError {
		return protoError("Invalid error code %d for STREAM_RST", errorCode)
	}
	return nil
}
