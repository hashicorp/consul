// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"github.com/hashicorp/consul/sdk/testutil"
	hclog "github.com/hashicorp/go-hclog"
)

func newDefaultDepsEnterprise(t testutil.TestingTB, _ hclog.Logger, _ *Config) EnterpriseDeps {
	t.Helper()
	return EnterpriseDeps{}
}
