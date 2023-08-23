// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package proxystateconverter

import (
	"github.com/hashicorp/consul/agent/structs"
)

func prioritizeByLocalityFailover(locality *structs.Locality, csns structs.CheckServiceNodes) []structs.CheckServiceNodes {
	return nil
}
