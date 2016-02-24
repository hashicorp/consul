package frame

import (
	"fmt"
	"io"
)

const (
	maxSynBodySize  = 8
	maxSynFrameSize = headerSize + maxSynBodySize
)

type RStreamSyn struct {
	Header
	body           [maxSynBodySize]byte
	streamPriority StreamPriority
	streamType     StreamType
}

// StreamType returns the stream's defined type as specified by
// the remote endpoint
func (f *RStreamSyn) StreamType() StreamType {
	return f.streamType
}

// StreamPriority returns the stream priority set on this frame
func (f *RStreamSyn) StreamPriority() StreamPriority {
	return f.streamPriority
}

func (f *RStreamSyn) parseFields() error {
	var length uint16 = 0
	flags := f.Flags()

	if flags.IsSet(flagStreamPriority) {
		f.streamPriority = StreamPriority(order.Uint32(f.body[length : length+4]))
		length += 4
	} else {
		f.streamPriority = 0
	}

	if flags.IsSet(flagStreamType) {
		f.streamType = StreamType(order.Uint32(f.body[length : length+4]))
		length += 4
	} else {
		f.streamType = 0
	}

	if length != f.Length() {
		return fmt.Errorf("Expected length %d for flags %v, but got %v", length, flags, f.Length())
	}

	return nil
}

func (f *RStreamSyn) readFrom(d deserializer) (err error) {
	if _, err = io.ReadFull(d, f.body[:f.Length()]); err != nil {
		return
	}

	if err = f.parseFields(); err != nil {
		return
	}

	return
}

type WStreamSyn struct {
	Header
	data   [maxSynFrameSize]byte
	length int
}

func (f *WStreamSyn) writeTo(s serializer) (err error) {
	_, err = s.Write(f.data[:headerSize+f.Length()])
	return
}

func (f *WStreamSyn) Set(streamId StreamId, streamPriority StreamPriority, streamType StreamType, fin bool) (err error) {
	var (
		flags  flagsType
		length int = 0
	)

	// set fin bit
	if fin {
		flags.Set(flagFin)
	}

	if streamPriority != 0 {
		if streamPriority > priorityMask {
			err = protoError("Priority %d is out of range", streamPriority)
			return
		}

		flags.Set(flagStreamPriority)
		start := headerSize + length
		order.PutUint32(f.data[start:start+4], uint32(streamPriority))
		length += 4
	}

	if streamType != 0 {
		flags.Set(flagStreamType)
		start := headerSize + length
		order.PutUint32(f.data[start:start+4], uint32(streamType))
		length += 4
	}

	// make the frame
	if err = f.Header.SetAll(TypeStreamSyn, length, streamId, flags); err != nil {
		return
	}
	return
}

func NewWStreamSyn() (f *WStreamSyn) {
	f = new(WStreamSyn)
	f.Header = Header(f.data[:headerSize])
	return
}
