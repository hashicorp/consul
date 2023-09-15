// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import "errors"

var (
	errInvalidAction       = errors.New("action must be either allow or deny")
	errSourcesTenancy      = errors.New("permissions sources may not specify partitions, peers, and sameness_groups together")
	errInvalidPrefixValues = errors.New("prefix values, regex values, and explicit names must not combined")
)
