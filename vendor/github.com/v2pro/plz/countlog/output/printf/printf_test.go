package printf

import (
	"testing"
	"github.com/v2pro/plz/countlog/spi"
	"github.com/stretchr/testify/require"
	"os"
	"io/ioutil"
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
	should.Equal("hello world", string(output))
}