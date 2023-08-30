// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows
// +build !windows

package agent

import (
	"os"
	"syscall"
)

var forwardSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
