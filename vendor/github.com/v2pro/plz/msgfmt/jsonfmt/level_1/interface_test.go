package test

import (
	"testing"
	"github.com/stretchr/testify/require"
	"io"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"github.com/v2pro/plz/reflect2"
)

func Test_slice_of_eface(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(([]interface{})(nil)))
	should.Equal("[1,null,3]", string(encoder.Encode(nil,nil, reflect2.PtrOf([]interface{}{
		1, nil, 3,
	}))))
}

type TestCloser int

func (closer TestCloser) Close() error {
	return nil
}

func Test_slice_of_iface(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(([]io.Closer)(nil)))
	should.Equal("[1,null,3]", string(encoder.Encode(nil,nil, reflect2.PtrOf([]io.Closer{
		TestCloser(1), nil, TestCloser(3),
	}))))
}
