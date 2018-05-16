package spi

import (
	"os"
	"github.com/v2pro/plz/msgfmt"
)

var OnError = func(err error) {
	msgfmt.Fprintf(os.Stderr, "countlog encountered error: {err}\n", "err", err.Error())
	os.Stderr.Sync()
}