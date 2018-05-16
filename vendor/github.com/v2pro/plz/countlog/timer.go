package countlog

import (
	"github.com/v2pro/plz/countlog/spi"
	"time"
)

func TraceTimer() int64 {
	if LevelTrace < spi.MinLevel {
		return 0
	}
	return time.Now().UnixNano()
}

func DebugTimer() int64 {
	if LevelDebug < spi.MinLevel {
		return 0
	}
	return time.Now().UnixNano()
}

func InfoTimer() int64 {
	if LevelInfo < spi.MinLevel {
		return 0
	}
	return time.Now().UnixNano()
}
