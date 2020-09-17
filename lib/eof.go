package lib

import (
	"errors"
	"io"
	"strings"

	"github.com/hashicorp/yamux"
)

var yamuxStreamClosed = yamux.ErrStreamClosed.Error()
var yamuxSessionShutdown = yamux.ErrSessionShutdown.Error()

// IsErrEOF returns true if we get an EOF error from the socket itself, or
// an EOF equivalent error from yamux.
func IsErrEOF(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}

	errStr := err.Error()
	if strings.Contains(errStr, yamuxStreamClosed) ||
		strings.Contains(errStr, yamuxSessionShutdown) ||
		strings.HasSuffix(errStr, io.EOF.Error()) {
		return true
	}

	return false
}
