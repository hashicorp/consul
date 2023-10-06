// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import "errors"

var (
	errInvalidName   = errors.New("invalid namespace name provided")
	errOwnerNonEmpty = errors.New("namespace should not have an owner")
)
