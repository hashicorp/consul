// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package versiontest

import "github.com/hashicorp/consul/version"

// IsEnterprise returns true if the current build is a Consul Enterprise build.
//
// This should only be called from test code.
func IsEnterprise() bool {
	return version.VersionMetadata == "ent"
}
