package main

import (
	"github.com/v2pro/plz/countlog/output"
	. "github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/countlog/output/printf"
)

func main() {
	EventWriter = output.NewEventWriter(output.EventWriterConfig{
		Format: &printf.Format{
			`[{level}] ` +
				`{timestamp, goTime, 15:04:05} ` +
				`{message} @ {file}:{line}`},
	})
	Info("{userA} called {userB} at {sometime}",
		"userA", "lily",
		"userB", "tom",
		"sometime", "yesterday")
	Info("{userA} called {userB} at {sometime}",
		"userA", "lily",
		"userB", "tom",
		"sometime", "yesterday")
}
