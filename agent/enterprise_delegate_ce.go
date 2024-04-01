// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package agent

// enterpriseDelegate has no functions in CE
type enterpriseDelegate interface{}
