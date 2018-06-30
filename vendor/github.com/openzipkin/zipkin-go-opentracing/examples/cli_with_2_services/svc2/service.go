// +build go1.7

package svc2

import (
	"context"
	"errors"
)

// Service constants
const (
	Int64Max = 1<<63 - 1
	Int64Min = -(Int64Max + 1)
)

// Service errors
var (
	ErrIntOverflow = errors.New("integer overflow occurred")
)

// Service interface to our svc2 service.
type Service interface {
	Sum(ctx context.Context, a int64, b int64) (int64, error)
}
