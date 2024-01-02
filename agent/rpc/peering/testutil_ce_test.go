// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package peering_test

import (
	"testing"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/go-hclog"
)

func newDefaultDepsEnterprise(t *testing.T, logger hclog.Logger, c *consul.Config) consul.EnterpriseDeps {
	t.Helper()
	return consul.EnterpriseDeps{}
}
