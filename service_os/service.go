// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package service_os

var chanGraceExit = make(chan int)

func Shutdown_Channel() <-chan int {
	return chanGraceExit
}
