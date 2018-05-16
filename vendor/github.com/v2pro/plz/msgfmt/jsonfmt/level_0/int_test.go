package test

import (
	"testing"
	"unsafe"
	"github.com/stretchr/testify/require"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"github.com/v2pro/plz/reflect2"
)

func Test_int8(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(int8(1)))
	should.Equal("-1", string(encoder.Encode(nil,nil, reflect2.PtrOf(int8(-1)))))
}

func Test_uint8(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(uint8(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(uint8(222)))))
}

func Test_int16(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(int16(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(int16(222)))))
}

func Test_uint16(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(uint16(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(uint16(222)))))
}

func Test_int32(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(int32(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(int32(222)))))
}

func Test_uint32(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(uint32(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(uint32(222)))))
}

func Test_int64(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(int64(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(int64(222)))))
}

func Test_uint64(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(uint64(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(uint64(222)))))
}

func Test_int(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(1))
	should.Equal("1", string(encoder.Encode(nil,nil, reflect2.PtrOf(1))))
	should.Equal("998123123", string(encoder.Encode(nil,nil, reflect2.PtrOf(998123123))))
	should.Equal("-998123123", string(encoder.Encode(nil,nil, reflect2.PtrOf(-998123123))))
}

func Test_uint(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(uint(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(uint(222)))))
}

func Benchmark_int(b *testing.B) {
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(1))
	values := []int{998123123, 123123435, 1230}
	ptrs := make([]unsafe.Pointer, len(values))
	for i := 0; i < len(values); i++ {
		ptrs[i] = unsafe.Pointer(&values[i])
	}
	var buf []byte
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		buf = encoder.Encode(nil,buf, ptrs[i%3])
	}
}
