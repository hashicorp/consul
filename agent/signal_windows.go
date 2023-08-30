// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows
// +build windows

package agent

import (
	"os"
)

var forwardSignals = []os.Signal{os.Interrupt}
