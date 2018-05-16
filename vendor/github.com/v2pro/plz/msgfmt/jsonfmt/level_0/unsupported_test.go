package test

import (
	"testing"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"github.com/stretchr/testify/require"
	"github.com/v2pro/plz/reflect2"
)

func Test_unsupported(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(make(chan int, 10)))
	output := encoder.Encode(nil,nil, reflect2.PtrOf(make(chan int, 10)))
	should.Equal(`"can not encode chan int  to json"`, string(output))
}