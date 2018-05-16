package json

import (
	"testing"
	"github.com/v2pro/plz/countlog/spi"
	"github.com/stretchr/testify/require"
)

func Test_event(t *testing.T) {
	should := require.New(t)
	format := &Format{}
	formatter := format.FormatterOf(&spi.LogSite{
		Sample: []interface{}{
			"k1", "v",
			"k2", 100,
		},
	})
	output := formatter.Format(nil, &spi.Event{
		Properties: []interface{}{
			"k1", "v",
			"k2", 100,
		},
	})
	should.Equal(`{"k1":"v","k2":100}`, string(output))
}