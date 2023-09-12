package types

import "errors"

var (
	errInvalidAction         = errors.New("action must be either allow or deny")
	errEmptyDestinationRules = errors.New("permissions must contain at least one destination rule")
	errSourcesTenancy        = errors.New("permissions sources may not specify partitions, peers, and sameness_groups together")
	errInvalidPrefixValues   = errors.New("prefix values must be valid and not combined with explicit names")
	errEmptySources          = errors.New("permissions must contain at least one source")
)
