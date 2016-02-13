package frame

import "io"

const (
	wndIncBodySize  = 4
	wndIncFrameSize = headerSize + wndIncBodySize
)

// Increase a stream's flow control window size
type RStreamWndInc struct {
	Header
	body [wndIncBodySize]byte
}

func (f *RStreamWndInc) WindowIncrement() (inc uint32) {
	return order.Uint32(f.body[:]) & wndIncMask
}

func (f *RStreamWndInc) readFrom(d deserializer) (err error) {
	if f.Length() != wndIncBodySize {
		return protoError("WND_INC length must be %d, got %d", wndIncBodySize, f.Length())
	}

	_, err = io.ReadFull(d, f.body[:])
	return
}

type WStreamWndInc struct {
	Header
	data [wndIncFrameSize]byte
}

func (f *WStreamWndInc) writeTo(s serializer) (err error) {
	_, err = s.Write(f.data[:])
	return
}

func (f *WStreamWndInc) Set(streamId StreamId, inc uint32) (err error) {
	if inc > wndIncMask {
		return protoError("Window increment %d out of range", inc)
	}

	order.PutUint32(f.data[headerSize:], inc)

	if err = f.Header.SetAll(TypeStreamWndInc, wndIncBodySize, streamId, 0); err != nil {
		return
	}

	return
}

func NewWStreamWndInc() (f *WStreamWndInc) {
	f = new(WStreamWndInc)
	f.Header = Header(f.data[:headerSize])
	return
}
