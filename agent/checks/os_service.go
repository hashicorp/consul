// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
