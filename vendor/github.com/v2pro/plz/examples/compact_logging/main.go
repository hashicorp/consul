package main

import (
	. "github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/countlog/output"
	"github.com/v2pro/plz/countlog/output/json"
	"github.com/v2pro/plz/countlog/output/lumberjack"
	"github.com/v2pro/plz/concurrent"
)

func main() {
	writer := concurrent.NewAsyncWriter(output.AsyncWriterConfig{
		Writer: &lumberjack.Logger{
			Filename: "/tmp/test.log.json",
		},
	})
	defer writer.Close()
	EventWriter = output.NewEventWriter(output.EventWriterConfig{
		Format: &json.Format{},
		Writer: writer,
	})
	for i := 0; i < 10; i++ {
		Info("game score calculated",
			"playerId", 1328 + i,
			"scores", []int{1, 2, 7 + i})
	}
}
