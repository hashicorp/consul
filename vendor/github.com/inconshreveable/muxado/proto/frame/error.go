package frame

import (
	"fmt"
)

const (
	NoError = iota
	ProtocolError
	InternalError
	FlowControlError
	StreamClosed
	FrameSizeError
	RefusedStream
	Cancel
	NoSuchError
)

type FramingError struct {
	error
}

func protoError(fmtstr string, args ...interface{}) FramingError {
	return FramingError{fmt.Errorf(fmtstr, args...)}
}
