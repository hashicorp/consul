package cli

import (
	"io"

	mcli "github.com/mitchellh/cli"
)

// Ui implements the mitchellh/cli.Ui interface, while exposing the underlying
// io.Writer used for stdout and stderr.
// TODO: rename to UI to match golang style guide
type Ui interface {
	mcli.Ui
	Stdout() io.Writer
	Stderr() io.Writer
}

// BasicUI augments mitchellh/cli.BasicUi by exposing the underlying io.Writer.
type BasicUI struct {
	mcli.BasicUi
}

func (b *BasicUI) Stdout() io.Writer {
	return b.BasicUi.Writer
}

func (b *BasicUI) Stderr() io.Writer {
	return b.BasicUi.ErrorWriter
}

// Command is an alias to reduce the diff. It can be removed at any time.
type Command mcli.Command
