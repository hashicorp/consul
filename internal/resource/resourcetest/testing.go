// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import "github.com/hashicorp/consul/sdk/testutil"

// T represents the subset of testing.T methods that will be used
// by the various functionality in this package
type T interface {
	testutil.TestingTB
}
