// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

func (c *AutopilotConfig) autopilotConfigExt() interface{} {
	return nil
}
