// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package xds

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
)

func prioritizeByLocalityFailover(_ hclog.Logger, _ *structs.Locality, _ structs.CheckServiceNodes) []structs.CheckServiceNodes {
	return nil
}
