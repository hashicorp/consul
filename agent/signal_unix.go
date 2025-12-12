// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package agent

import (
	"os"
	"syscall"
)

var forwardSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
