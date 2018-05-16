package spi

import "github.com/v2pro/plz/msgfmt"

var coloredNames = map[int]string{}

func init() {
	levels := []int{
		LevelTraceCall,
		LevelTrace,
		LevelDebugCall,
		LevelDebug,
		LevelInfoCall,
		LevelInfo,
		LevelWarn,
		LevelError,
		LevelFatal,
	}
	for _, level := range levels {
		coloredNames[level] = msgfmt.Sprintf("\x1b[{color};1m[{level}]\x1b[0m ",
			"color", getColor(level), "level", LevelName(level))
	}
}

func ColoredLevelName(level int) string {
	return coloredNames[level]
}

const (
	nocolor = 0
	black   = 30
	red     = 31
	green   = 32
	yellow  = 33
	blue    = 34
	purple  = 35
	cyan    = 36
	gray    = 37
)

func getColor(level int) int {
	switch level {
	case LevelTrace, LevelTraceCall:
		return cyan
	case LevelDebug, LevelDebugCall:
		return blue
	case LevelInfo, LevelInfoCall:
		return green
	case LevelWarn:
		return yellow
	case LevelError:
		return red
	case LevelFatal:
		return purple
	default:
		return nocolor
	}
}
