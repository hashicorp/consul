package types

import "errors"

var (
	errInvalidAction       = errors.New("action must be either allow or deny")
	errSourcesTenancy      = errors.New("permissions sources may not specify partitions, peers, and sameness_groups together")
	errInvalidPrefixValues = errors.New("prefix values must be valid and not combined with explicit names")
)
