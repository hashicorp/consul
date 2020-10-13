package logging

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"github.com/fatih/color"
)

type ColorOption uint8

const (
	// ColorNever is the default coloration, and does not inject color codes into the io.Writer.
	ColorNever ColorOption = iota
	// ColorAuto checks if os.Stdout is a tty, and if so enables coloring.
	ColorAuto
	// ColorAlways will enable coloring, regardless of whether os.Stdout is a tty or not.
	ColorAlways
)

func NewColorOption(v string) (ColorOption, error) {
	switch v {
	case "auto":
		return ColorAuto, nil
	case "always", "on", "enabled":
		return ColorAlways, nil
	case "", "never", "off", "disabled":
		return ColorNever, nil
	default:
		return ColorNever, fmt.Errorf("invalid color value %v, must be one of: auto,on,off", v)
	}
}

func newColorWriter(out io.Writer, option ColorOption) io.Writer {
	switch {
	case option == ColorNever:
		return out
	case option == ColorAlways:
		color.NoColor = false
		return &colorWriter{out: out}
	case color.NoColor:
		return out
	default:
		return &colorWriter{out: out}
	}
}

type colorWriter struct {
	out io.Writer
}

func (c *colorWriter) Write(b []byte) (int, error) {
	for label, color := range levelLabels {
		index := bytes.Index(b, []byte(label))
		if index < 0 {
			continue
		}

		buf := bufio.NewWriter(c.out)
		buf.Write(b[:index])
		color.Fprint(buf, label)
		buf.Write(b[index+len(label):])
		return len(b), buf.Flush()

	}
	return c.out.Write(b)
}

var levelLabels = map[string]*color.Color{
	"[WARN]":  color.New(color.FgYellow),
	"[ERROR]": color.New(color.FgRed),
}
