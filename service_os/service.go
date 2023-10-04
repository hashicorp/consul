// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package service_os

// test that frontend tack is not running when changes are outside of ui folder
var chanGraceExit = make(chan int)

func Shutdown_Channel() <-chan int {
	return chanGraceExit
}
