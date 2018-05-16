package test

import (
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"github.com/v2pro/plz/reflect2"
)

func Test_float64(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(float64(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(float64(222)))))
	should.Equal("1.2345", string(encoder.Encode(nil,nil, reflect2.PtrOf(float64(1.2345)))))
	should.Equal("1.23456", string(encoder.Encode(nil,nil, reflect2.PtrOf(float64(1.23456)))))
	should.Equal("1.234567", string(encoder.Encode(nil,nil, reflect2.PtrOf(float64(1.234567)))))
	should.Equal("1.001", string(encoder.Encode(nil,nil, reflect2.PtrOf(float64(1.001)))))
}

func Test_float32(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(float32(1)))
	should.Equal("222", string(encoder.Encode(nil,nil, reflect2.PtrOf(float32(222)))))
	should.Equal("1.2345", string(encoder.Encode(nil,nil, reflect2.PtrOf(float32(1.2345)))))
	should.Equal("1.23456", string(encoder.Encode(nil,nil, reflect2.PtrOf(float32(1.23456)))))
	should.Equal("1.234567", string(encoder.Encode(nil,nil, reflect2.PtrOf(float32(1.234567)))))
	should.Equal("1.001", string(encoder.Encode(nil,nil, reflect2.PtrOf(float32(1.001)))))
}