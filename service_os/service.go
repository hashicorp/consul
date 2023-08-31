// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package service_os

var chanGraceExit = make(chan int)

func Shutdown_Channel() <-chan int {
	return chanGraceExit
}
