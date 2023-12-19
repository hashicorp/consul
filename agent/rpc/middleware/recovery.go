package middleware

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
)

// NewPanicHandler returns a RecoveryHandlerFunc type function
// to handle panic in RPC server's handlers.
func NewPanicHandler(logger hclog.Logger) RecoveryHandlerFunc {
	return func(p interface{}) (err error) {
		// Log the panic and the stack trace of the Goroutine that caused the panic.
		stacktrace := hclog.Stacktrace()
		logger.Error("panic serving rpc request",
			"panic", p,
			"stack", stacktrace,
		)

		return fmt.Errorf("rpc: panic serving request")
	}
}

type RecoveryHandlerFunc func(p interface{}) (err error)
