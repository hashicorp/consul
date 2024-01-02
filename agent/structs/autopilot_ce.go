// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package structs

func (c *AutopilotConfig) autopilotConfigExt() interface{} {
	return nil
}
