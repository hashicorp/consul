package hrf

import (
	"github.com/v2pro/plz/countlog/spi"
	"time"
)

type timerFormatter struct {
	idx int
}

func (formatter *timerFormatter) Format(space []byte, event *spi.Event) []byte {
	latency := event.Timestamp.UnixNano() - event.Properties[formatter.idx].(int64)
	space = append(space, "\ntimer: "...)
	space = append(space, time.Duration(latency).String()...)
	return space
}

