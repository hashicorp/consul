// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import "errors"

var (
	errSourcesTenancy       = errors.New("permissions sources may not specify partitions, peers, and sameness_groups together")
	errSourceWildcards      = errors.New("permission sources may not have wildcard namespaces and explicit names.")
	errSourceExcludes       = errors.New("must be defined on wildcard sources")
	errInvalidPrefixValues  = errors.New("prefix values, regex values, and explicit names must not combined")
	errHeaderRulesInvalid   = errors.New("header rule must contain header name")
	ErrWildcardNotSupported = errors.New("traffic permissions without explicit destinations are not yet supported")
)
