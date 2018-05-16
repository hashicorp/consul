package jsonfmt

import (
	"unsafe"
	"math"
	"strconv"
	"context"
)

var pow10 []uint64

func init() {
	pow10 = []uint64{1, 10, 100, 1000, 10000, 100000, 1000000}
}

type lossyFloat64Encoder struct {
}

func (encoder *lossyFloat64Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteFloat64Lossy(space, *(*float64)(ptr))
}

type lossyFloat32Encoder struct {
}

func (encoder *lossyFloat32Encoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteFloat32Lossy(space, *(*float32)(ptr))
}

// WriteFloat64 write float64 to stream
func WriteFloat64(space []byte, val float64) []byte {
	abs := math.Abs(val)
	fmt := byte('f')
	// Note: Must use float32 comparisons for underlying float32 value to get precise cutoffs right.
	if abs != 0 {
		if abs < 1e-6 || abs >= 1e21 {
			fmt = 'e'
		}
	}
	return append(space, strconv.FormatFloat(float64(val), fmt, -1, 64)...)
}
// WriteFloat64Lossy write float64 to stream with ONLY 6 digits precision although much much faster
func WriteFloat64Lossy(space []byte, val float64) []byte {
	if val < 0 {
		space = append(space, '-')
		val = -val
	}
	if val > 0x4ffffff {
		return WriteFloat64(space, val)
	}
	precision := 6
	exp := uint64(1000000) // 6
	lval := uint64(val*float64(exp) + 0.5)
	space = WriteUint64(space, lval / exp)
	fval := lval % exp
	if fval == 0 {
		return space
	}
	space = append(space, '.')
	for p := precision - 1; p > 0 && fval < pow10[p]; p-- {
		space = append(space, '0')
	}
	space = WriteUint64(space, fval)
	for space[len(space)-1] == '0' {
		space = space[:len(space) - 1]
	}
	return space
}

// WriteFloat32 write float32 to stream
func WriteFloat32(space []byte, val float32) []byte {
	abs := math.Abs(float64(val))
	fmt := byte('f')
	// Note: Must use float32 comparisons for underlying float32 value to get precise cutoffs right.
	if abs != 0 {
		if float32(abs) < 1e-6 || float32(abs) >= 1e21 {
			fmt = 'e'
		}
	}
	return append(space, strconv.FormatFloat(float64(val), fmt, -1, 32)...)
}

// WriteFloat32Lossy write float32 to stream with ONLY 6 digits precision although much much faster
func WriteFloat32Lossy(space []byte, val float32) []byte {
	if val < 0 {
		space = append(space, '-')
		val = -val
	}
	if val > 0x4ffffff {
		return WriteFloat32(space, val)
	}
	precision := 6
	exp := uint64(1000000) // 6
	lval := uint64(float64(val)*float64(exp) + 0.5)
	space = WriteUint64(space, lval / exp)
	fval := lval % exp
	if fval == 0 {
		return space
	}
	space = append(space, '.')
	for p := precision - 1; p > 0 && fval < pow10[p]; p-- {
		space = append(space, '0')
	}
	space = WriteUint64(space, fval)
	for space[len(space)-1] == '0' {
		space = space[:len(space) - 1]
	}
	return space
}