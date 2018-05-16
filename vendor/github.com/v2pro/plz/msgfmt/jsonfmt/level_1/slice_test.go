package test

import (
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"github.com/v2pro/plz/reflect2"
)

func Test_slice(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf([]int(nil)))
	should.Equal("[1,2,3]", string(encoder.Encode(nil,nil, reflect2.PtrOf([]int{
		1, 2, 3,
	}))))
}