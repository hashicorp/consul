package frame

import "io"

const (
	headerSize = 8
)

type Header []byte

func newHeader() Header {
	return Header(make([]byte, headerSize))
}

func (b Header) readFrom(d deserializer) (err error) {
	// read the header
	if _, err = io.ReadFull(d, []byte(b)); err != nil {
		return err
	}
	return
}

func (b Header) Length() uint16 {
	return order.Uint16(b[:2]) & lengthMask
}

func (b Header) SetLength(length int) (err error) {
	if length > lengthMask || length < 0 {
		return protoError("Frame length %d out of range", length)
	}

	order.PutUint16(b[:2], uint16(length))
	return
}

func (b Header) Type() FrameType {
	return FrameType((b[3]) & typeMask)
}

func (b Header) SetType(t FrameType) (err error) {
	b[3] = byte(t & typeMask)
	return
}

func (b Header) StreamId() StreamId {
	return StreamId(order.Uint32(b[4:]) & streamMask)
}

func (b Header) SetStreamId(streamId StreamId) (err error) {
	if streamId > streamMask {
		return protoError("Stream id %d out of range", streamId)
	}

	order.PutUint32(b[4:], uint32(streamId))
	return
}

func (b Header) Flags() flagsType {
	return flagsType(b[2])
}

func (b Header) SetFlags(fl flagsType) (err error) {
	b[2] = byte(fl)
	return
}

func (b Header) Fin() bool {
	return b.Flags().IsSet(flagFin)
}

func (b Header) SetAll(ftype FrameType, length int, streamId StreamId, flags flagsType) (err error) {
	if err = b.SetType(ftype); err != nil {
		return
	}
	if err = b.SetLength(length); err != nil {
		return
	}
	if err = b.SetStreamId(streamId); err != nil {
		return
	}
	if err = b.SetFlags(flags); err != nil {
		return
	}
	return
}
