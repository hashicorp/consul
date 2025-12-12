// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package service_os

var chanGraceExit = make(chan int)

func Shutdown_Channel() <-chan int {
	return chanGraceExit
}
