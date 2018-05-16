package jsonfmt

import (
	"unsafe"
	"context"
)

var digits []uint32

func init() {
	digits = make([]uint32, 1000)
	for i := uint32(0); i < 1000; i++ {
		digits[i] = (((i / 100) + '0') << 16) + ((((i / 10) % 10) + '0') << 8) + i%10 + '0'
		if i < 10 {
			digits[i] += 2 << 24
		} else if i < 100 {
			digits[i] += 1 << 24
		}
	}
}

type int8Encoder struct {
}

func (encoder *int8Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteInt8(space, *(*int8)(ptr))
}

type uint8Encoder struct {
}

func (encoder *uint8Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteUint8(space, *(*uint8)(ptr))
}

type int16Encoder struct {
}

func (encoder *int16Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteInt16(space, *(*int16)(ptr))
}

type uint16Encoder struct {
}

func (encoder *uint16Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteUint16(space, *(*uint16)(ptr))
}

type int32Encoder struct {
}

func (encoder *int32Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteInt32(space, *(*int32)(ptr))
}

type uint32Encoder struct {
}

func (encoder *uint32Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteUint32(space, *(*uint32)(ptr))
}

type int64Encoder struct {
}

func (encoder *int64Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteInt64(space, *(*int64)(ptr))
}

type uint64Encoder struct {
}

func (encoder *uint64Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteUint64(space, *(*uint64)(ptr))
}

func WriteUint8(space []byte, val uint8) []byte {
	return writeFirstBuf(space, digits[val])
}

func WriteInt8(space []byte, nval int8) []byte {
	var val uint8
	if nval < 0 {
		val = uint8(-nval)
		space = append(space, '-')
	} else {
		val = uint8(nval)
	}
	return writeFirstBuf(space, digits[val])
}

func WriteInt16(space []byte, nval int16) []byte {
	var val uint16
	if nval < 0 {
		val = uint16(-nval)
		space = append(space, '-')
	} else {
		val = uint16(nval)
	}
	return WriteUint16(space, val)
}

func WriteUint16(space []byte, val uint16) []byte {
	q1 := val / 1000
	if q1 == 0 {
		return writeFirstBuf(space, digits[val])
	}
	r1 := val - q1*1000
	space = writeFirstBuf(space, digits[q1])
	return writeBuf(space, digits[r1])
}

func WriteInt32(space []byte, nval int32) []byte {
	var val uint32
	if nval < 0 {
		val = uint32(-nval)
		space = append(space, '-')
	} else {
		val = uint32(nval)
	}
	return WriteUint32(space, val)
}

func WriteUint32(space []byte, val uint32) []byte {
	q1 := val / 1000
	if q1 == 0 {
		return writeFirstBuf(space, digits[val])
	}
	r1 := val - q1*1000
	q2 := q1 / 1000
	if q2 == 0 {
		space = writeFirstBuf(space, digits[q1])
		return writeBuf(space, digits[r1])
	}
	r2 := q1 - q2*1000
	q3 := q2 / 1000
	if q3 == 0 {
		space = writeFirstBuf(space, digits[q2])
	} else {
		r3 := q2 - q3*1000
		space = append(space, byte(q3 + '0'))
		return writeBuf(space, digits[r3])
	}
	space = writeBuf(space, digits[r2])
	return writeBuf(space, digits[r1])
}

func WriteInt64(space []byte, nval int64) []byte {
	var val uint64
	if nval < 0 {
		val = uint64(-nval)
		space = append(space, '-')
	} else {
		val = uint64(nval)
	}
	return WriteUint64(space, val)
}

func WriteUint64(space []byte, val uint64) []byte {
	q1 := val / 1000
	if q1 == 0 {
		return writeFirstBuf(space, digits[val])
	}
	r1 := val - q1*1000
	q2 := q1 / 1000
	if q2 == 0 {
		space = writeFirstBuf(space, digits[q1])
		space = writeBuf(space, digits[r1])
		return space
	}
	r2 := q1 - q2*1000
	q3 := q2 / 1000
	if q3 == 0 {
		space = writeFirstBuf(space, digits[q2])
		space = writeBuf(space, digits[r2])
		space = writeBuf(space, digits[r1])
		return space
	}
	r3 := q2 - q3*1000
	q4 := q3 / 1000
	if q4 == 0 {
		space = writeFirstBuf(space, digits[q3])
		space = writeBuf(space, digits[r3])
		space = writeBuf(space, digits[r2])
		space = writeBuf(space, digits[r1])
		return space
	}
	r4 := q3 - q4*1000
	q5 := q4 / 1000
	if q5 == 0 {
		space = writeFirstBuf(space, digits[q4])
		space = writeBuf(space, digits[r4])
		space = writeBuf(space, digits[r3])
		space = writeBuf(space, digits[r2])
		space = writeBuf(space, digits[r1])
		return space
	}
	r5 := q4 - q5*1000
	q6 := q5 / 1000
	if q6 == 0 {
		space = writeFirstBuf(space, digits[q5])
	} else {
		space = append(space, byte(q6 + '0'))
		r6 := q5 - q6*1000
		space = writeBuf(space, digits[r6])
	}
	space = writeBuf(space, digits[r5])
	space = writeBuf(space, digits[r4])
	space = writeBuf(space, digits[r3])
	space = writeBuf(space, digits[r2])
	space = writeBuf(space, digits[r1])
	return space
}


func writeFirstBuf(space []byte, v uint32) []byte {
	start := v >> 24
	if start == 0 {
		space = append(space, byte(v >> 16), byte(v >> 8))
	} else if start == 1 {
		space = append(space, byte(v >> 8))
	}
	space = append(space, byte(v))
	return space
}

func writeBuf(space []byte, v uint32) []byte {
	return append(space, byte(v >> 16), byte(v >> 8), byte(v))
}