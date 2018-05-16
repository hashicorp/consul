package hrf

import (
	"testing"
	"github.com/v2pro/plz/countlog/spi"
	"github.com/stretchr/testify/require"
)

func Test_message(t *testing.T) {
	should := require.New(t)
	format := &Format{}
	formatter := format.FormatterOf(&spi.LogSite{
		Event: "hello {key}",
		Sample: []interface{}{"key", "world"},
	})
	output := formatter.Format(nil, &spi.Event{
		Properties: []interface{}{"key", "world"},
	})
	should.Equal("hello world\x1b[90;1m\nkey: world\x1b[0m\x1b[90;1m\x1b[0m\n", string(output))
}