package frame

import (
	"io"
)

const (
	// data frames are actually longer, but they are variable length
	dataFrameSize = headerSize
)

type RStreamData struct {
	Header
	fixed [dataFrameSize]byte

	toRead io.LimitedReader // when reading, the underlying connection's io.Reader is handed up
}

func (f *RStreamData) Reader() io.Reader {
	return &f.toRead
}

func (f *RStreamData) readFrom(d deserializer) (err error) {
	// not using io.LimitReader to avoid a heap memory allocation in the hot path
	f.toRead.R = d
	f.toRead.N = int64(f.Length())
	return
}

// WStreamData is a StreamData frame that you can write
// It delivers opaque data on a stream to the application layer
type WStreamData struct {
	Header
	fixed   [dataFrameSize]byte
	toWrite []byte // when writing, you just pass a byte slice to write
}

func (f *WStreamData) writeTo(s serializer) (err error) {
	if _, err = s.Write(f.fixed[:]); err != nil {
		return err
	}

	if _, err = s.Write(f.toWrite); err != nil {
		return err
	}

	return
}

func (f *WStreamData) Set(streamId StreamId, data []byte, fin bool) (err error) {
	var flags flagsType
	if fin {
		flags.Set(flagFin)
	}

	if err = f.Header.SetAll(TypeStreamData, len(data), streamId, flags); err != nil {
		return
	}

	f.toWrite = data
	return
}

func NewWStreamData() (f *WStreamData) {
	f = new(WStreamData)
	f.Header = f.fixed[:headerSize]
	return
}
