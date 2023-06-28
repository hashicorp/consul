// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package xds

import (
	"github.com/hashicorp/consul/agent/structs"
)

func prioritizeByLocalityFailover(locality *structs.Locality, csns structs.CheckServiceNodes) []structs.CheckServiceNodes {
	return nil
}
