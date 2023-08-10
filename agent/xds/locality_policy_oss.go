// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
