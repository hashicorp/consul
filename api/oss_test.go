// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package api

// The following defaults return "default" in enterprise and "" in OSS.
// This constant is useful when a default value is needed for an
// operation that will reject non-empty values in OSS.
const defaultNamespace = ""
const defaultPartition = ""
