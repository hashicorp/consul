package checks

import (
	"errors"
)

const (
	errOSServiceStatusCritical = "OS Service unhealthy"
)

var (
	ErrOSServiceStatusCritical = errors.New(errOSServiceStatusCritical)
)
