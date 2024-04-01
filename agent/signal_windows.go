// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package agent

import (
	"os"
)

var forwardSignals = []os.Signal{os.Interrupt}
