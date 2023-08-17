// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package agent

// enterpriseDelegate has no functions in OSS
type enterpriseDelegate interface{}
