// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
