package test

import (
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"github.com/v2pro/plz/reflect2"
)

func Test_array(t *testing.T) {
	should := require.New(t)
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf([3]int{}))
	should.Equal("[1,2,3]", string(encoder.Encode(nil, nil, reflect2.PtrOf([3]int{
		1, 2, 3,
	}))))
}