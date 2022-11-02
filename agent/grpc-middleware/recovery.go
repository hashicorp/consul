package middleware

import (
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PanicHandlerMiddlewareOpts returns the []recovery.Option containing
// recovery handler function.
func PanicHandlerMiddlewareOpts(logger Logger) []recovery.Option {
	return []recovery.Option{
		recovery.WithRecoveryHandler(NewPanicHandler(logger)),
	}
}

// NewPanicHandler returns a recovery.RecoveryHandlerFunc closure function
// to handle panic in GRPC server's handlers.
func NewPanicHandler(logger Logger) recovery.RecoveryHandlerFunc {
	return func(p interface{}) (err error) {
		// Log the panic and the stack trace of the Goroutine that caused the panic.
		stacktrace := hclog.Stacktrace()
		logger.Error("panic serving grpc request",
			"panic", p,
			"stack", stacktrace,
		)

		return status.Errorf(codes.Internal, "grpc: panic serving request")
	}
}

type Logger interface {
	Error(string, ...interface{})
}
