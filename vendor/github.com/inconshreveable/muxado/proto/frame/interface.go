package frame

import (
	"io"
)

type Transport interface {
	WriteFrame(WFrame) error
	ReadFrame() (RFrame, error)
	Close() error
}

// A frame can read and write itself to a serializer/deserializer
type RFrame interface {
	StreamId() StreamId
	Type() FrameType
	readFrom(deserializer) error
}

type WFrame interface {
	writeTo(serializer) error
}

type deserializer io.Reader
type serializer io.Writer
