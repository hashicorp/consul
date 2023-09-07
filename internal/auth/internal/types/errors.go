package types

import "errors"

var (
	errInvalidAction         = errors.New("action must be either allow or deny")
	errEmptyDestinationRules = errors.New("permissions must contain at least one destination rule")
	errEmptySources          = errors.New("permissions must contain at least one source")
	errWildcardNotSupported  = errors.New("partition and wildcard traffic permissions are not yet supported")
)
